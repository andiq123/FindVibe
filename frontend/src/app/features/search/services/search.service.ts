import { Injectable, signal, inject } from "@angular/core";
import { HttpClient } from "@angular/common/http";
import { catchError, Observable, tap, of } from "rxjs";
import {
  SearchStatus,
  Song,
  SearchResponse,
  PaginationInfo,
} from "../../../core/models/song.model";
import { environment } from "../../../../environments/environment";
import { StorageService } from "../../../core/services/storage.service";
import { directAudioEnabled } from "../../../core/utils/song-audio";
import { artworkKey, fillSearchArtwork, missingArtwork } from "./search-artwork";
import { readSearchStream } from "./search-stream";
import { SettingsService } from "../../../core/services/settings.service";

const BASE_API_URL = environment.API_URL;
const SEARCH_STORAGE_KEY = "search_state";

interface SearchState {
  songs: Song[];
  status: SearchStatus;
  query: string;
  pagination: PaginationInfo | null;
  currentPage: number;
}

@Injectable({
  providedIn: "root",
})
export class SearchService {
  private readonly httpClient = inject(HttpClient);
  private readonly storageService = inject(StorageService);
  private readonly settingsService = inject(SettingsService);
  private readonly _songs = signal<Song[]>([]);
  private readonly _searchStatus = signal<SearchStatus>(SearchStatus.None);
  private readonly _suggestions = signal<string[]>([]);
  private readonly _suggestionsLoading = signal<boolean>(false);
  private suggestQuery = "";
  private activeSearch?: AbortController;
  private artworkRequest?: AbortController;
  readonly searchWarning = signal("");
  private readonly _lastSearchQuery = signal<string>("");
  private readonly _pagination = signal<PaginationInfo | null>(null);
  private readonly _currentPage = signal(1);

  constructor() {
    this.restoreState();
    this.loadMissingArtwork();
  }

  readonly songs = this._songs.asReadonly();
  readonly status = this._searchStatus.asReadonly();
  readonly suggestions = this._suggestions.asReadonly();
  readonly suggestionsLoading = this._suggestionsLoading.asReadonly();
  readonly lastQuery = this._lastSearchQuery.asReadonly();
  readonly pagination = this._pagination.asReadonly();
  readonly currentPage = this._currentPage.asReadonly();

  /**
   * Search music providers directly via Fiber, rendering each batch as it arrives.
   * Only successfully mapped playable tracks are returned.
   */
  searchSongs(
    searchTerm: string,
    page = 1,
    force = false,
  ): Observable<SearchResponse> {
    if (
      !force &&
      searchTerm === this._lastSearchQuery() &&
      page === this._currentPage() &&
      this._searchStatus() === SearchStatus.Finished &&
      this._songs().length > 0
    ) {
      this.loadMissingArtwork();
      return of({ songs: this._songs(), pagination: this._pagination() });
    }
    return new Observable<SearchResponse>((subscriber) => {
      this.artworkRequest?.abort();
      this.activeSearch?.abort();
      const controller = new AbortController();
      this.activeSearch = controller;
      const timeout = setTimeout(() => controller.abort(new Error("Search timed out")), 30_000);
      const isCurrent = () => this.activeSearch === controller && !subscriber.closed;
      this._searchStatus.set(SearchStatus.Loading);
      this._songs.set([]);
      this._pagination.set(null);
      this._lastSearchQuery.set(searchTerm);
      this._currentPage.set(page);
      this.searchWarning.set("");

      const run = async () => {
        let complete = false;
        const links = new Set<string>();
        try {
          const params = new URLSearchParams({ q: searchTerm, page: String(page), stream: "1" });
          const response = await fetch(`${BASE_API_URL}/search?${params}`, { signal: controller.signal });
          for await (const event of readSearchStream(response)) {
            if (!isCurrent()) return;
            if (event.type === "meta" && event.pagination) this._pagination.set(event.pagination);
            if (event.type === "song" && event.song && !links.has(event.song.link)) {
              links.add(event.song.link);
              this._songs.update((songs) => [...songs, event.song!]);
            }
            if (event.type === "error") {
              if (event.error === "no songs found") { complete = true; break; }
              throw new Error(event.error || "Search failed");
            }
            if (event.type === "done") { complete = true; break; }
          }
          if (!complete) throw new Error("Search interrupted");
          if (!isCurrent()) return;
          this._searchStatus.set(SearchStatus.Finished);
          this.settingsService.setServerUp();
          this.saveState();
          this.loadMissingArtwork();
        } catch {
          if (!isCurrent()) return;
          if (this._songs().length) {
            this.searchWarning.set("Some sources didn’t respond. You can play these results or retry.");
            this._searchStatus.set(SearchStatus.Finished);
          } else {
            this._searchStatus.set(SearchStatus.Error);
          }
        } finally {
          clearTimeout(timeout);
        }
        if (isCurrent()) {
          subscriber.next({ songs: this._songs(), pagination: this._pagination() });
          subscriber.complete();
        }
      };
      void run().finally(() => subscriber.complete());
      return () => {
        clearTimeout(timeout);
        controller.abort();
        if (this.activeSearch === controller) {
          this.activeSearch = undefined;
          if (this._searchStatus() === SearchStatus.Loading) this._searchStatus.set(SearchStatus.None);
        }
      };
    });
  }

  getSuggestions(term: string): Observable<string[]> {
    const q = term.trim();
    this.suggestQuery = q;
    if (!q) {
      this.resetSuggestions();
      return of([]);
    }
    this._suggestionsLoading.set(true);
    const { hl, gl } = this.settingsService.suggestLocale();
    return this.httpClient
      .get<string[]>(
        `${BASE_API_URL}/suggest?q=${encodeURIComponent(q)}&hl=${hl}&gl=${gl}`,
      )
      .pipe(
        tap({
          next: (result) => {
            if (this.suggestQuery !== q) return;
            this._suggestions.set(Array.isArray(result) ? result : []);
            this._suggestionsLoading.set(false);
          },
          error: () => {
            if (this.suggestQuery !== q) return;
            this._suggestionsLoading.set(false);
            this._suggestions.set([]);
          },
        }),
        catchError(() => of([])),
      );
  }

  resetSuggestions(): void {
    this.suggestQuery = "";
    this._suggestions.set([]);
    this._suggestionsLoading.set(false);
  }

  resetSearch(): void {
    this.artworkRequest?.abort();
    this.activeSearch?.abort();
    this.activeSearch = undefined;
    this.searchWarning.set("");
    this._songs.set([]);
    this._searchStatus.set(SearchStatus.None);
    this.resetSuggestions();
    this._lastSearchQuery.set("");
    this._pagination.set(null);
    this._currentPage.set(1);
    this.storageService.removeItem(SEARCH_STORAGE_KEY);
  }

  private loadMissingArtwork(): void {
    this.artworkRequest?.abort();
    const request = new AbortController();
    this.artworkRequest = request;
    void fillSearchArtwork(this._songs(), BASE_API_URL, request.signal, (key, image) => {
      this._songs.update((songs) => songs.map((song) =>
        missingArtwork(song) && artworkKey(song) === key ? { ...song, image } : song,
      ));
    }).then(() => {
      if (!request.signal.aborted) this.saveState();
    });
  }

  private saveState(): void {
    this.storageService.setItem(SEARCH_STORAGE_KEY, {
      songs: this._songs(),
      status: this._searchStatus(),
      query: this._lastSearchQuery(),
      pagination: this._pagination(),
      currentPage: this._currentPage(),
    });
  }

  private restoreState(): void {
    const state = this.storageService.getItem<SearchState>(SEARCH_STORAGE_KEY);
    if (!state || !state.songs?.every(directAudioEnabled)) return;
    this._songs.set(state.songs);
    this._searchStatus.set(state.status);
    this._lastSearchQuery.set(state.query);
    this._pagination.set(state.pagination ?? null);
    this._currentPage.set(state.currentPage || 1);
  }
}
