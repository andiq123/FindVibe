import { Injectable, signal, inject, computed, OnDestroy } from "@angular/core";
import { StorageService } from "./storage.service";
import { RepeatMode } from "../../features/player/models/player.model";
import { HttpClient } from "@angular/common/http";
import {
  Observable,
  catchError,
  defer,
  EMPTY,
  retry,
  switchMap,
  tap,
  timer,
  timeout,
} from "rxjs";
import { environment } from "../../../environments/environment";

enum ServerStatus {
  Unchecked = "unchecked",
  Up = "up",
  Down = "down",
}

/** Where Google music suggestions are biased. Default Romania. */
export type SuggestRegion = "ro" | "device";

@Injectable({
  providedIn: "root",
})
export class SettingsService implements OnDestroy {
  private readonly storageService = inject(StorageService);
  private readonly httpClient = inject(HttpClient);
  private readonly healthUrl = `${environment.API_URL}/health`;
  private readonly _repeatMode = signal<RepeatMode>(RepeatMode.OFF);
  private readonly _isShuffle = signal(false);
  private readonly _isMiniPlayer = signal(true);
  /**
   * When true, the next mini mount should skip entry slide —
   * full player already animated the close.
   */
  private quietMiniLand = false;
  private readonly _suggestRegion = signal<SuggestRegion>("ro");
  /** Full-player Save/Download MP3 button. */
  private readonly _showPlayerDownload = signal(true);
  /** Auto-cache vault audio when adding (heart / Spotify import). */
  private readonly _autoOfflineCache = signal(true);
  private readonly _serverStatus = signal(ServerStatus.Unchecked);
  private readonly _isNavigatorOffline = signal(!navigator.onLine);
  private onlineHandler = () => this._isNavigatorOffline.set(false);
  private offlineHandler = () => this._isNavigatorOffline.set(true);
  readonly repeatMode = this._repeatMode.asReadonly();
  readonly isShuffle = this._isShuffle.asReadonly();
  readonly isMiniPlayer = this._isMiniPlayer.asReadonly();
  readonly suggestRegion = this._suggestRegion.asReadonly();
  readonly showPlayerDownload = this._showPlayerDownload.asReadonly();
  readonly autoOfflineCache = this._autoOfflineCache.asReadonly();
  readonly isServerDown = computed(
    () => this._serverStatus() === ServerStatus.Down,
  );
  readonly isCheckedServer = computed(
    () => this._serverStatus() !== ServerStatus.Unchecked,
  );
  /** True when the device has no network (CDN streaming / offline vault gate). */
  readonly isNavigatorOffline = this._isNavigatorOffline.asReadonly();
  /** API unreachable or device offline — gates search / server-backed flows. */
  readonly isOffline = computed(
    () => this._isNavigatorOffline() || this.isServerDown(),
  );
  /** Google suggest hl/gl — Romania by default, or browser locale. */
  readonly suggestLocale = computed(() => {
    if (this._suggestRegion() === "device") {
      const [lang, region] = (navigator.language || "en").split("-");
      return {
        hl: (lang || "en").slice(0, 2).toLowerCase(),
        gl: (region || lang || "US").slice(0, 2).toUpperCase(),
      };
    }
    return { hl: "ro", gl: "RO" };
  });

  initialize(): void {
    window.addEventListener("online", this.onlineHandler);
    window.addEventListener("offline", this.offlineHandler);
    const repeatMode = this.storageService.getItem<RepeatMode>("repeatMode");
    const isShuffle = this.storageService.getItem<boolean>("isShuffle");
    const suggestRegion =
      this.storageService.getItem<SuggestRegion>("suggestRegion");
    const showPlayerDownload =
      this.storageService.getItem<boolean>("showPlayerDownload");
    const autoOfflineCache =
      this.storageService.getItem<boolean>("autoOfflineCache");
    if (repeatMode !== null) this._repeatMode.set(repeatMode);
    if (isShuffle !== null) this._isShuffle.set(isShuffle);
    if (suggestRegion === "ro" || suggestRegion === "device") {
      this._suggestRegion.set(suggestRegion);
    }
    if (showPlayerDownload !== null) {
      this._showPlayerDownload.set(showPlayerDownload);
    }
    if (autoOfflineCache !== null) {
      this._autoOfflineCache.set(autoOfflineCache);
    }
  }

  ngOnDestroy(): void {
    window.removeEventListener("online", this.onlineHandler);
    window.removeEventListener("offline", this.offlineHandler);
  }

  toggleMiniPlayer(): void {
    if (this._isMiniPlayer()) {
      this._isMiniPlayer.set(false);
      return;
    }
    // Closing full → mini: don't re-run entry slide (full already slid down).
    this.quietMiniLand = true;
    this._isMiniPlayer.set(true);
  }

  /** One-shot: true if mini should appear without entry animation. */
  takeQuietMiniLand(): boolean {
    if (!this.quietMiniLand) return false;
    this.quietMiniLand = false;
    return true;
  }

  toggleRepeat(): void {
    const modes = [RepeatMode.OFF, RepeatMode.ALL, RepeatMode.ONE];
    const i = modes.indexOf(this._repeatMode());
    const nextMode = modes[((i < 0 ? 0 : i) + 1) % modes.length];
    this._repeatMode.set(nextMode);
    this.storageService.setItem("repeatMode", nextMode);
  }

  toggleShuffle(): void {
    const newValue = !this._isShuffle();
    this._isShuffle.set(newValue);
    this.storageService.setItem("isShuffle", newValue);
  }

  setSuggestRegion(region: SuggestRegion): void {
    this._suggestRegion.set(region);
    this.storageService.setItem("suggestRegion", region);
  }

  setShowPlayerDownload(show: boolean): void {
    this._showPlayerDownload.set(show);
    this.storageService.setItem("showPlayerDownload", show);
  }

  toggleShowPlayerDownload(): void {
    this.setShowPlayerDownload(!this._showPlayerDownload());
  }

  setAutoOfflineCache(on: boolean): void {
    this._autoOfflineCache.set(on);
    this.storageService.setItem("autoOfflineCache", on);
  }

  toggleAutoOfflineCache(): void {
    this.setAutoOfflineCache(!this._autoOfflineCache());
  }

  setServerUp(): void {
    this._serverStatus.set(ServerStatus.Up);
  }

  setServerDown(): void {
    this._serverStatus.set(ServerStatus.Down);
  }

  wakeServer(): Observable<void> {
    return this.httpClient.get<void>(this.healthUrl);
  }

  /**
   * Single /health poke after resume — Render may have slept while the PWA
   * was backgrounded even if we previously marked Up.
   */
  nudgeWake(): void {
    this.wakeServer()
      .pipe(
        timeout({ first: 12_000 }),
        tap(() => this.setServerUp()),
        catchError(() => {
          this.setServerDown();
          return EMPTY;
        }),
      )
      .subscribe();
  }

  /**
   * Probe /health until Render answers. Starts Unchecked → banner "Waking…".
   * On failure stays Down (no flash back to Unchecked). Recurses every 20s.
   */
  wakeUntilUp(): Observable<void> {
    return defer(() =>
      this.wakeServer().pipe(
        timeout({ first: 12_000 }),
        retry({
          count: 12,
          delay: (_err, n) => timer(Math.min(2000 * n, 12_000)),
        }),
        tap(() => this.setServerUp()),
      ),
    ).pipe(
      catchError(() => {
        this.setServerDown();
        // ponytail: free Render sleeps — keep poking every 20s
        return timer(20_000).pipe(switchMap(() => this.wakeUntilUp()));
      }),
    );
  }
}
