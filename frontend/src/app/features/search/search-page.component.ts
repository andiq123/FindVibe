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
import { SearchStatus } from "../../core/models/song.model";
import { Router } from "@angular/router";
import { SearchService } from "./services/search.service";
import { PlaylistService } from "../../core/services/playlist.service";
import { EmptyStateComponent } from "../../shared/empty-state/empty-state.component";
import {
  faMagnifyingGlass,
  faTriangleExclamation,
} from "../../shared/icons";
@Component({
  selector: "app-search-page",
  standalone: true,
  imports: [
    SearchBarComponent,
    EmptyStateComponent,
    PageContentComponent,
    SongListComponent,
    PaginationComponent,
  ],
  templateUrl: "./search-page.component.html",
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class SearchPageComponent {
  private router = inject(Router);
  private songsService = inject(SearchService);
  private playlistService = inject(PlaylistService);
  private destroyRef = inject(DestroyRef);
  query = input<string>("");
  songs = computed(() => this.songsService.songs());
  status = computed(() => this.songsService.status());
  pagination = computed(() => this.songsService.pagination());
  searchStatus = SearchStatus;
  warning = this.songsService.searchWarning;

  retrySearch(): void {
    this.songsService.searchSongs(this.query(), 1, true)
      .pipe(takeUntilDestroyed(this.destroyRef)).subscribe();
  }
  faMagnifyingGlass = faMagnifyingGlass;
  faTriangleExclamation = faTriangleExclamation;

  /** Idle only when the route has no query and we are not mid-search. */
  hasActiveSearch = computed(
    () =>
      !!this.query() ||
      this.status() === SearchStatus.Loading ||
      this.status() === SearchStatus.Finished ||
      this.status() === SearchStatus.Error,
  );

  constructor() {
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

}
