import {
  Component,
  OnInit,
  inject,
  computed,
  DestroyRef,
  ChangeDetectionStrategy,
} from "@angular/core";
import { RouterOutlet } from "@angular/router";
import { takeUntilDestroyed } from "@angular/core/rxjs-interop";
import { MiniPlayerComponent } from "./features/player/mini-player/mini-player.component";
import { FullPlayerComponent } from "./features/player/full-player/full-player.component";
import { NavigationComponent } from "./features/navigation/navigation.component";
import { ConnectionStatusComponent } from "./shared/connection-status/connection-status.component";
import { UpdateOverlayComponent } from "./shared/update-overlay/update-overlay.component";
import { GlobalModalComponent } from "./shared/components/global-modal/global-modal.component";
import { SettingsService } from "./core/services/settings.service";
import { UserService } from "./features/library/services/user.service";
import { AudioService } from "./core/services/audio.service";
import { MediaSessionService } from "./core/services/media-session.service";
import { OfflineStorageService } from "./features/library/services/offline-storage.service";
import { ModalService } from "./core/services/modal.service";
import { AppUpdateService } from "./core/services/app-update.service";
import { PlayerService } from "./core/services/player.service";
import { PlaylistService } from "./core/services/playlist.service";
import { ToastService } from "./core/services/toast.service";
import { KeyboardDismissComponent } from "./shared/keyboard-dismiss/keyboard-dismiss.component";

/** Enables :active styles on iOS — no-op handler, passive. */
function noopTouch(): void {}

@Component({
  selector: "app-root",
  standalone: true,
  imports: [
    RouterOutlet,
    MiniPlayerComponent,
    FullPlayerComponent,
    NavigationComponent,
    UpdateOverlayComponent,
    ConnectionStatusComponent,
    GlobalModalComponent,
    KeyboardDismissComponent,
  ],
  templateUrl: "./app.component.html",
  styleUrl: "./app.component.scss",
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class AppComponent implements OnInit {
  private updateService = inject(AppUpdateService);
  private mediaSessionService = inject(MediaSessionService);
  private audioService = inject(AudioService);
  private offlineStorageService = inject(OfflineStorageService);
  private destroyRef = inject(DestroyRef);
  newUpdateAvailable = this.updateService.newUpdateAvailable;
  secondsToUpdate = this.updateService.secondsToUpdate;
  readonly playlistService = inject(PlaylistService);
  readonly playerService = inject(PlayerService);
  readonly settingsService = inject(SettingsService);
  readonly modalService = inject(ModalService);
  readonly toast = inject(ToastService);
  ngOnInit(): void {
    this.initializeServices();
    // iOS Safari: :active (press scale) only fires if a touch listener exists.
    document.addEventListener("touchstart", noopTouch, { passive: true });
    this.destroyRef.onDestroy(() =>
      document.removeEventListener("touchstart", noopTouch),
    );
    this.settingsService
      .wakeUntilUp()
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe();
  }
  onToggleSize() {
    this.settingsService.toggleMiniPlayer();
  }
  private initializeServices(): void {
    this.audioService.initialize();
    this.settingsService.initialize();
    this.mediaSessionService.initialize();
    this.offlineStorageService.initialize();
    // Continue listening — restore queue + position (paused; autoplay blocked).
    void this.playerService.restoreSession();
  }
}
