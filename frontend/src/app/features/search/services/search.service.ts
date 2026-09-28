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
  private readonly _lastSearchQuery = signal<string>("");
  private readonly _pagination = signal<PaginationInfo | null>(null);
  private readonly _currentPage = signal(1);

  constructor() {
    this.restoreState();
  }

  readonly songs = this._songs.asReadonly();
  readonly status = this._searchStatus.asReadonly();
  readonly suggestions = this._suggestions.asReadonly();
  readonly suggestionsLoading = this._suggestionsLoading.asReadonly();
  readonly lastQuery = this._lastSearchQuery.asReadonly();
  readonly pagination = this._pagination.asReadonly();
  readonly currentPage = this._currentPage.asReadonly();

  /**
   * Search songs via Fiber: Last.fm catalog → provider stream map.
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
      return of({ songs: this._songs(), pagination: this._pagination() });
    }
    this._searchStatus.set(SearchStatus.Loading);
    this._songs.set([]);
    const q = encodeURIComponent(searchTerm);
    const url =
      page > 1
        ? `${BASE_API_URL}/search?q=${q}&page=${page}`
        : `${BASE_API_URL}/search?q=${q}`;
    return this.httpClient.get<SearchResponse>(url).pipe(
      tap((response) => {
        this._songs.set(response.songs);
        this._pagination.set(response.pagination ?? null);
        this._currentPage.set(page);
        this._searchStatus.set(SearchStatus.Finished);
        this._lastSearchQuery.set(searchTerm);
        this.saveState();
      }),
      catchError(() => {
        this._lastSearchQuery.set(searchTerm);
        this._currentPage.set(page);
        this._pagination.set(null);
        this._searchStatus.set(SearchStatus.Error);
        return of({ songs: [], pagination: null });
      }),
    );
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
    this._songs.set([]);
    this._searchStatus.set(SearchStatus.None);
    this.resetSuggestions();
    this._lastSearchQuery.set("");
    this._pagination.set(null);
    this._currentPage.set(1);
    this.storageService.removeItem(SEARCH_STORAGE_KEY);
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
    if (!state) return;
    this._songs.set(state.songs);
    this._searchStatus.set(state.status);
    this._lastSearchQuery.set(state.query);
    this._pagination.set(state.pagination ?? null);
    this._currentPage.set(state.currentPage || 1);
  }
}
