import {
  Component,
  computed,
  effect,
  input,
  inject,
  ChangeDetectionStrategy,
  untracked,
  DestroyRef,
} from "@angular/core";
import { takeUntilDestroyed } from "@angular/core/rxjs-interop";
import { SearchBarComponent } from "./search-bar/search-bar.component";
import { PageContentComponent } from "../../shared/components/page-content/page-content.component";
import { SongListComponent } from "../../shared/components/song-list/song-list.component";
import { PaginationComponent } from "../../shared/components/pagination/pagination.component";
import { SearchStatus, sourceHost } from "../../core/models/song.model";
import { Router } from "@angular/router";
import { SearchService } from "./services/search.service";
import { SettingsService } from "../../core/services/settings.service";
import { PlayerService } from "../../core/services/player.service";
import { PlaylistService } from "../../core/services/playlist.service";
import { EmptyStateComponent } from "../../shared/empty-state/empty-state.component";
import { FontAwesomeModule } from "@fortawesome/angular-fontawesome";
import { PlayerButtonComponent } from "../../shared/player-button/player-button.component";
import { FavoriteButtonComponent } from "../../shared/favorite-button/favorite-button.component";
import {
  faMagnifyingGlass,
  faTriangleExclamation,
} from "../../shared/icons";
import { OfflineStorageService } from "../library/services/offline-storage.service";
import { PlayerStatus } from "../player/models/player.model";
@Component({
  selector: "app-search-page",
  standalone: true,
  imports: [
    SearchBarComponent,
    FontAwesomeModule,
    EmptyStateComponent,
    PageContentComponent,
    SongListComponent,
    PaginationComponent,
    PlayerButtonComponent,
    FavoriteButtonComponent,
  ],
  templateUrl: "./search-page.component.html",
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class SearchPageComponent {
  private router = inject(Router);
  private songsService = inject(SearchService);
  private settingsService = inject(SettingsService);
  private playlistService = inject(PlaylistService);
  public playerService = inject(PlayerService);
  private offlineStorageService = inject(OfflineStorageService);
  private destroyRef = inject(DestroyRef);
  query = input<string>("");
  songs = computed(() => this.songsService.songs());
  status = computed(() => this.songsService.status());
  pagination = computed(() => this.songsService.pagination());
  searchStatus = SearchStatus;
  faMagnifyingGlass = faMagnifyingGlass;
  faTriangleExclamation = faTriangleExclamation;
  sourceHost = sourceHost;

  /** Idle only when the route has no query and we are not mid-search. */
  hasActiveSearch = computed(
    () =>
      !!this.query() ||
      this.status() === SearchStatus.Loading ||
      this.status() === SearchStatus.Finished ||
      this.status() === SearchStatus.Error,
  );

  topSong = computed(() => {
    const list = this.songs() ?? [];
    return list[0] ?? null;
  });
  remainingSongs = computed(() => {
    const list = this.songs() ?? [];
    return list.slice(1);
  });

  isTopSongUnavailable = computed(() => {
    const song = this.topSong();
    if (!song) return false;
    return (
      this.settingsService.isNavigatorOffline() &&
      !this.offlineStorageService.availableOfflineSongIds().includes(song.id)
    );
  });

  isTopSongActive = computed(() => {
    const top = this.topSong();
    const current = this.playlistService.currentSong();
    if (!top || !current) return false;
    return top.link === current.link;
  });

  isTopSongError = computed(
    () =>
      this.isTopSongActive() &&
      this.playerService.status() === PlayerStatus.Error,
  );

  topPlayerStatus = computed(() =>
    this.isTopSongActive()
      ? this.playerService.status()
      : PlayerStatus.Paused,
  );

  songListEmptyTitle = computed(() =>
    this.remainingSongs().length === 0
      ? "Top match shown above"
      : "Empty vault",
  );

  songListEmptyDescription = computed(() =>
    this.remainingSongs().length === 0
      ? "That's the best result for this search."
      : "Try a different search!",
  );
  constructor() {
    effect(() => {
      if (
        this.settingsService.isServerDown() &&
        this.router.url !== "/library"
      ) {
        this.router.navigate(["/library"]);
      }
    });
    effect(() => {
      const currentQuery = this.query();
      if (!currentQuery) {
        const lastQuery = untracked(() => this.songsService.lastQuery());
        if (lastQuery) {
          this.router.navigate(["/songs", lastQuery], { replaceUrl: true });
        }
      }
    });
  }
  onChangePlaylist(): void {
    this.playlistService.setCurrentPlaylist(this.songs());
  }

  onPageChange(page: number): void {
    const query = this.query();
    if (!query) return;
    this.songsService
      .searchSongs(query, page)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe();
    window.scrollTo({ top: 0, behavior: "smooth" });
  }

  async toggleTopSong(): Promise<void> {
    const song = this.topSong();
    if (!song || this.isTopSongUnavailable()) return;

    const current = this.playlistService.currentSong();
    const isActive = !!current && current.link === song.link;

    if (isActive) {
      if (this.playerService.status() === PlayerStatus.Playing) {
        this.playerService.pause();
      } else {
        // Paused / Error / Ended → play (Error hard-retries the stream).
        await this.playerService.play();
      }
      return;
    }

    // Ensure the queue contains the featured song before starting playback.
    this.playlistService.setCurrentPlaylist(this.songs());
    await this.playerService.setSong(song);
  }
}
