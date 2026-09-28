import {
  Injectable,
  OnDestroy,
  inject,
  signal,
  effect,
} from "@angular/core";
import { SwUpdate, VersionReadyEvent } from "@angular/service-worker";
import {
  filter,
  interval,
  map,
  switchMap,
  takeWhile,
  tap,
  Subscription,
} from "rxjs";
import { PlayerService } from "./player.service";
import { PlayerStatus } from "../../features/player/models/player.model";

const UPDATE_COUNTDOWN_SECONDS = 3;
/** Ignore brief Paused blips during cold track changes before reloading. */
const IDLE_SETTLE_MS = 2500;

export function playbackBusy(status: PlayerStatus): boolean {
  return (
    status === PlayerStatus.Playing ||
    status === PlayerStatus.Loading ||
    status === PlayerStatus.Ended
  );
}

@Injectable({
  providedIn: "root",
})
export class AppUpdateService implements OnDestroy {
  private readonly swUpdate = inject(SwUpdate);
  private readonly player = inject(PlayerService);
  readonly newUpdateAvailable = signal(false);
  readonly secondsToUpdate = signal(UPDATE_COUNTDOWN_SECONDS);
  readonly updateLoading = signal(false);
  private checkIntervalSubscription: Subscription | null = null;
  private versionUpdatesSubscription: Subscription | null = null;
  /** Reload deferred until playback stops — don't kill background audio. */
  private pendingApply = false;
  private settleTimer: ReturnType<typeof setTimeout> | null = null;
  private visibilityHandler = () => this.onVisibility();

  constructor() {
    effect((onCleanup) => {
      if (!this.swUpdate.isEnabled) return;
      if (this.checkIntervalSubscription) {
        this.checkIntervalSubscription.unsubscribe();
        this.checkIntervalSubscription = null;
      }
      if (this.versionUpdatesSubscription) {
        this.versionUpdatesSubscription.unsubscribe();
        this.versionUpdatesSubscription = null;
      }
      this.checkForUpdate();
      this.checkIntervalSubscription = interval(6 * 60 * 60 * 1000)
        .pipe(tap(() => this.checkForUpdate()))
        .subscribe();
      this.versionUpdatesSubscription = this.swUpdate.versionUpdates
        .pipe(
          filter(
            (evt): evt is VersionReadyEvent => evt.type === "VERSION_READY",
          ),
          tap(() => this.newUpdateAvailable.set(true)),
          switchMap(() => this.countdown(UPDATE_COUNTDOWN_SECONDS)),
          tap((time) => {
            this.secondsToUpdate.set(time);
            if (time === 0) void this.applyUpdateWhenIdle();
          }),
        )
        .subscribe();
      onCleanup(() => {
        if (this.checkIntervalSubscription) {
          this.checkIntervalSubscription.unsubscribe();
          this.checkIntervalSubscription = null;
        }
        if (this.versionUpdatesSubscription) {
          this.versionUpdatesSubscription.unsubscribe();
          this.versionUpdatesSubscription = null;
        }
      });
    });

    // If we deferred for playback, apply once truly idle in the foreground.
    effect(() => {
      if (!this.pendingApply) return;
      const status = this.player.status();
      if (playbackBusy(status)) {
        this.clearSettle();
        return;
      }
      this.scheduleIdleApply();
    });

    if (typeof document !== "undefined") {
      document.addEventListener("visibilitychange", this.visibilityHandler);
    }
  }

  private checkForUpdate(): void {
    if (this.swUpdate.isEnabled) {
      this.swUpdate.checkForUpdate();
    }
  }

  private async applyUpdateWhenIdle(): Promise<void> {
    const status = this.player.status();
    if (playbackBusy(status)) {
      this.pendingApply = true;
      return;
    }
    // Visible + settled — apply now. Background pause blips stay deferred.
    if (
      typeof document !== "undefined" &&
      document.visibilityState !== "visible"
    ) {
      this.pendingApply = true;
      return;
    }
    await this.applyUpdate();
  }

  private scheduleIdleApply(): void {
    if (
      typeof document !== "undefined" &&
      document.visibilityState !== "visible"
    ) {
      return;
    }
    this.clearSettle();
    this.settleTimer = setTimeout(() => {
      this.settleTimer = null;
      if (!this.pendingApply) return;
      if (
        typeof document !== "undefined" &&
        document.visibilityState !== "visible"
      ) {
        return;
      }
      if (playbackBusy(this.player.status())) return;
      this.pendingApply = false;
      void this.applyUpdate();
    }, IDLE_SETTLE_MS);
  }

  private onVisibility(): void {
    if (!this.pendingApply) return;
    if (document.visibilityState !== "visible") {
      this.clearSettle();
      return;
    }
    if (!playbackBusy(this.player.status())) {
      this.scheduleIdleApply();
    }
  }

  private clearSettle(): void {
    if (this.settleTimer == null) return;
    clearTimeout(this.settleTimer);
    this.settleTimer = null;
  }

  async applyUpdate(): Promise<void> {
    this.updateLoading.set(true);
    try {
      // Activate waiting SW first — bare reload can keep the old worker.
      if (this.swUpdate.isEnabled) await this.swUpdate.activateUpdate();
    } finally {
      document.location.reload();
    }
  }

  private countdown(startTimer: number) {
    return interval(1000).pipe(
      map((i) => startTimer - i),
      takeWhile((val) => val >= 0),
    );
  }

  ngOnDestroy(): void {
    this.clearSettle();
    if (typeof document !== "undefined") {
      document.removeEventListener("visibilitychange", this.visibilityHandler);
    }
  }
}
