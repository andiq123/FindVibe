import {
  Component,
  computed,
  signal,
  inject,
  ChangeDetectionStrategy,
  DestroyRef,
} from "@angular/core";
import { takeUntilDestroyed } from "@angular/core/rxjs-interop";
import { LibraryService } from "./services/library.service";
import { UserService } from "./services/user.service";
import { UserFormComponent } from "./components/user-form/user-form.component";
import { TitleCasePipe } from "@angular/common";
import { finalize } from "rxjs";
import {
  faArrowDown,
  faArrowUp,
  faCheck,
  faClockRotateLeft,
  faWaveSquare,
} from "../../shared/icons";
import { FontAwesomeModule } from "@fortawesome/angular-fontawesome";
import { RouterLink } from "@angular/router";
import { PlaylistService } from "../../core/services/playlist.service";
import { SettingsService } from "../../core/services/settings.service";
import { RadioService } from "../../core/services/radio.service";
import { PlayerService } from "../../core/services/player.service";
import { StorageService } from "../../core/services/storage.service";
import { ToastService } from "../../core/services/toast.service";
import { PageContentComponent } from "../../shared/components/page-content/page-content.component";
import { SongListComponent } from "../../shared/components/song-list/song-list.component";
import { Song } from "../../core/models/song.model";

@Component({
  selector: "app-library",
  standalone: true,
  imports: [
    UserFormComponent,
    TitleCasePipe,
    FontAwesomeModule,
    PageContentComponent,
    SongListComponent,
    RouterLink,
  ],
  templateUrl: "./library.component.html",
  styleUrl: "./library.component.scss",
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class LibraryComponent {
  private libraryService = inject(LibraryService);
  private userService = inject(UserService);
  private playlistService = inject(PlaylistService);
  private settingsService = inject(SettingsService);
  private radioService = inject(RadioService);
  private playerService = inject(PlayerService);
  private storage = inject(StorageService);
  private toast = inject(ToastService);
  private destroyRef = inject(DestroyRef);

  /** Select + drag reorder — one organize mode. */
  editMode = signal(false);
  hasReordered = signal(false);
  selectedIds = signal<ReadonlySet<string>>(new Set());
  songs = this.libraryService.songs;
  songCount = computed(() => this.songs().length);
  selectedCount = computed(() => this.selectedIds().size);
  isLoggedIn = computed(() => !!this.userService.user());
  username = computed(() => this.userService.user()?.username || "");
  loadingSongs = this.libraryService.loadingSongs;
  loadingReorder = signal(false);
  aboveMini = computed(
    () =>
      !!this.playlistService.currentSong() &&
      this.settingsService.isMiniPlayer(),
  );

  faCheck = faCheck;
  faArrowUp = faArrowUp;
  faArrowDown = faArrowDown;
  faWaveSquare = faWaveSquare;
  faClockRotateLeft = faClockRotateLeft;
  /** Header Radio spinner — not per-song starts. */
  radioLoading = () => this.radioService.isLoadingStation();
  radioBusy = () => this.radioService.loading();

  /** Snapshot before first drag/bulk move — Cancel restores this. */
  private reorderSnapshot: Song[] | null = null;

  onChangePlaylist() {
    this.playlistService.setCurrentPlaylist(this.songs());
  }

  async startVaultRadio(): Promise<void> {
    if (this.radioBusy() || this.editMode()) return;
    const seed = await this.radioService.startFromVault(
      this.songs(),
      this.storage.listenStats(),
    );
    if (!seed) {
      this.toast.show(this.radioService.error() || "Couldn't start radio");
      return;
    }
    this.toast.show(`Radio · ${seed.artist}`);
    await this.playerService.setSong(seed, { fromQueue: true });
  }

  toggleEditMode() {
    if (this.loadingReorder()) return;
    if (this.editMode()) {
      this.discardEdits();
      return;
    }
    this.editMode.set(true);
  }

  toggleSongSelect(id: string) {
    const next = new Set(this.selectedIds());
    if (next.has(id)) next.delete(id);
    else next.add(id);
    this.selectedIds.set(next);
  }

  moveSelected(dest: "top" | "bottom" | "up" | "down") {
    const ids = [...this.selectedIds()];
    if (!ids.length) return;
    this.markReordered();
    if (!this.libraryService.moveSelected(ids, dest)) {
      if (this.reorderSnapshot) {
        const same = this.songs().every(
          (s, i) => s.id === this.reorderSnapshot![i]?.id,
        );
        if (same) {
          this.hasReordered.set(false);
          this.reorderSnapshot = null;
        }
      }
    }
  }

  reorderSongs(data: { from: string; to: string }) {
    this.markReordered();
    this.libraryService.changePlaces(data.from, data.to);
  }

  /** Single confirm — persist order and leave edit mode. */
  saveEdits() {
    if (this.loadingReorder() || !this.hasReordered()) return;
    this.loadingReorder.set(true);
    this.libraryService
      .saveReorders()
      .pipe(
        finalize(() => this.loadingReorder.set(false)),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe({
        next: () => {
          this.hasReordered.set(false);
          this.reorderSnapshot = null;
          this.selectedIds.set(new Set());
          this.editMode.set(false);
        },
      });
  }

  /** Cancel exits edit mode and restores order. */
  private discardEdits() {
    if (this.reorderSnapshot) {
      this.libraryService.replaceSongs(this.reorderSnapshot);
    }
    this.hasReordered.set(false);
    this.reorderSnapshot = null;
    this.selectedIds.set(new Set());
    this.editMode.set(false);
  }

  private markReordered() {
    if (!this.hasReordered()) {
      this.reorderSnapshot = this.songs().map((s) => ({ ...s }));
      this.hasReordered.set(true);
    }
  }
}
