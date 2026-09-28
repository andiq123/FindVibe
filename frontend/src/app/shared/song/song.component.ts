import {
  Component,
  input,
  output,
  inject,
  ChangeDetectionStrategy,
} from "@angular/core";
import {
  Song,
  sameSong,
  songKey,
  sourceHost,
} from "../../core/models/song.model";
import { PlayerStatus } from "../../features/player/models/player.model";
import { PlayerButtonComponent } from "../player-button/player-button.component";
import { MovingTitleComponent } from "../moving-title/moving-title.component";
import { NgTemplateOutlet } from "@angular/common";
import { FavoriteButtonComponent } from "../favorite-button/favorite-button.component";
import { FontAwesomeModule } from "@fortawesome/angular-fontawesome";
import { OfflineStorageService } from "../../features/library/services/offline-storage.service";
import { LibraryService } from "../../features/library/services/library.service";
import { DragAndDropDirective } from "../directives/drag-and-drop.directive";
import { PlayerService } from "../../core/services/player.service";
import { PlaylistService } from "../../core/services/playlist.service";
import { RadioService } from "../../core/services/radio.service";
import { SettingsService } from "../../core/services/settings.service";
import { ToastService } from "../../core/services/toast.service";
import {
  faGripVertical,
  faListUl,
  faTriangleExclamation,
  faWaveSquare,
} from "../icons";
@Component({
  selector: "app-song",
  standalone: true,
  imports: [
    PlayerButtonComponent,
    MovingTitleComponent,
    FavoriteButtonComponent,
    FontAwesomeModule,
    DragAndDropDirective,
    NgTemplateOutlet,
  ],
  templateUrl: "./song.component.html",
  styleUrl: "./song.component.scss",
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class SongComponent {
  private playerService = inject(PlayerService);
  private playlistService = inject(PlaylistService);
  private offlineStorageService = inject(OfflineStorageService);
  private settingsService = inject(SettingsService);
  private radioService = inject(RadioService);
  private libraryService = inject(LibraryService);
  private toast = inject(ToastService);
  song = input.required<Song>();
  /** Vault Edit — checkbox + drag handle. */
  selectMode = input(false);
  selected = input(false);
  loading = input<boolean>(false);
  /** Vault: wave button starts radio from this track instead of queue. */
  radioAction = input(false);
  reorder = output<{ from: string; to: string }>();
  playlistChange = output<void>();
  toggleSelect = output<string>();
  isActive = () => sameSong(this.playlistService.currentSong(), this.song());
  status = () =>
    this.isActive() ? this.playerService.status() : PlayerStatus.Paused;
  isAvailableOffline = () =>
    this.offlineStorageService
      .availableOfflineSongIds()
      .includes(this.song().id);
  isUnavailable = () =>
    this.settingsService.isNavigatorOffline() && !this.isAvailableOffline();
  isError = () => this.status() === PlayerStatus.Error;
  faTriangleExclamation = faTriangleExclamation;
  faListUl = faListUl;
  faWaveSquare = faWaveSquare;
  faGripVertical = faGripVertical;
  radioBusy = () => this.radioService.loading();
  radioLoading = () => this.radioService.isLoadingSong(this.song());
  sourceLabel = () => sourceHost(this.song().provider);
  async play() {
    if (this.isActive()) {
      await this.playerService.play();
      return;
    }
    this.playlistChange.emit();
    await this.playerService.setSong(this.song());
  }
  async pause() {
    this.playerService.pause();
  }
  async playOrPause() {
    if (this.selectMode()) {
      this.toggleSelect.emit(this.song().id);
      return;
    }
    if (this.isUnavailable()) return;
    if (this.status() === PlayerStatus.Playing) {
      await this.pause();
    } else {
      await this.play();
    }
  }
  onSelectToggle(event: Event) {
    event.stopPropagation();
    event.preventDefault();
    this.toggleSelect.emit(this.song().id);
  }
  /** Keep row play/pause from firing when tapping the action cluster. */
  guardActions(event: Event) {
    event.stopPropagation();
  }
  addToQueue(event: Event) {
    event.stopPropagation();
    event.preventDefault();
    this.playerService.addToQueue(this.song());
  }

  async startRadio(event: Event) {
    event.stopPropagation();
    event.preventDefault();
    if (this.radioService.loading()) return;
    const seed = this.song();
    const vault = this.libraryService.songs();
    const ok = await this.radioService.start(seed, {
      excludeLinks: vault.map((s) => s.link).filter(Boolean),
      excludeKeys: vault.map((s) => songKey(s)).filter(Boolean),
      loadingKey: songKey(seed),
    });
    if (!ok) {
      this.toast.show(this.radioService.error() || "Couldn't start radio");
      return;
    }
    this.toast.show(`Radio · ${seed.artist}`);
    await this.playerService.setSong(seed, { fromQueue: true });
  }
}
