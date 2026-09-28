import {
  Injectable,
  effect,
  inject,
  OnDestroy,
  signal,
  untracked,
} from "@angular/core";
import { Song } from "../models/song.model";
import {
  PlayerStatus,
  RepeatMode,
} from "../../features/player/models/player.model";
import { SettingsService } from "./settings.service";
import { PlaylistService } from "./playlist.service";
import { OfflineStorageService } from "../../features/library/services/offline-storage.service";
import { AudioService } from "./audio.service";
import { RadioService } from "./radio.service";
import { ToastService } from "./toast.service";
import { StorageService } from "./storage.service";
import { upgradeToHttps } from "../utils/utils";

/** Cap consecutive dead tracks so a broken queue can't spin forever. */
const MAX_ERROR_SKIPS = 10;
const PERSIST_EVERY_MS = 5_000;

@Injectable({
  providedIn: "root",
})
export class PlayerService implements OnDestroy {
  private readonly settingsService = inject(SettingsService);
  private readonly playlistService = inject(PlaylistService);
  private readonly offlineStorageService = inject(OfflineStorageService);
  private readonly audioService = inject(AudioService);
  private readonly radioService = inject(RadioService);
  private readonly toast = inject(ToastService);
  private readonly storage = inject(StorageService);
  private currentObjectUrl: string | null = null;
  private handlingSongEnded = false;
  private handlingSongError = false;
  /** True only when advancing the queue (ended / next / prev) — not a user pick. */
  private allowErrorSkip = false;
  private consecutiveErrorSkips = 0;
  private wakeLock: WakeLockSentinel | null = null;
  private wakeLockGen = 0;
  private loadGen = 0;
  private persistTimerId: ReturnType<typeof setInterval> | null = null;
  private visibilityHandler = () => this.onVisibility();
  readonly status = this.audioService.status;
  readonly currentTime = this.audioService.currentTime;
  readonly duration = this.audioService.duration;
  /** Short user-facing reason while status === Error. */
  readonly playError = signal("");

  constructor() {
    effect(() => {
      if (this.status() === PlayerStatus.Ended && !this.handlingSongEnded) {
        this.handlingSongEnded = true;
        untracked(() => {
          this.handleSongEnded().finally(() => {
            this.handlingSongEnded = false;
          });
        });
      }
    });
    effect(() => {
      const status = this.status();
      if (status === PlayerStatus.Playing) {
        untracked(() => {
          this.playError.set("");
          this.allowErrorSkip = false;
          this.consecutiveErrorSkips = 0;
          void this.requestWakeLock();
          this.startPersistLoop();
          this.warmNextTrack();
        });
        return;
      }
      untracked(() => {
        this.stopPersistLoop();
        void this.releaseWakeLock();
        if (
          status === PlayerStatus.Paused ||
          status === PlayerStatus.Stopped ||
          status === PlayerStatus.Ended
        ) {
          this.persistNow();
        }
        if (status === PlayerStatus.Stopped || status === PlayerStatus.Error) {
          this.audioService.clearPreload();
        }
      });
      if (status === PlayerStatus.Error) {
        untracked(() => {
          if (!this.playError()) {
            this.playError.set("Couldn't play this track");
          }
          if (!this.handlingSongError && this.allowErrorSkip) {
            this.handlingSongError = true;
            this.handleSongError().finally(() => {
              this.handlingSongError = false;
            });
          }
        });
      }
    });
    // Queue edits (play next / radio append) — keep standby src fresh.
    effect(() => {
      const nextLink = this.playlistService.upcoming()[0]?.link ?? "";
      const status = this.status();
      if (
        status !== PlayerStatus.Playing &&
        status !== PlayerStatus.Loading &&
        status !== PlayerStatus.Paused
      ) {
        return;
      }
      untracked(() => {
        void nextLink;
        this.warmNextTrack();
      });
    });
    if (typeof document !== "undefined") {
      document.addEventListener("visibilitychange", this.visibilityHandler);
    }
  }

  /** Hydrate queue + load track paused at saved position (continue listening). */
  async restoreSession(): Promise<void> {
    if (!this.playlistService.restoreSession()) return;
    const song = this.playlistService.currentSong();
    if (!song) return;
    const seek = this.playlistService.takePendingSeek();
    const gen = await this.loadSong(song, {
      fromQueue: false,
      autoplay: false,
    });
    if (gen === this.loadGen && seek > 0) this.audioService.seek(seek);
  }

  private async handleSongEnded(): Promise<void> {
    const mode = this.settingsService.repeatMode();
    if (mode === RepeatMode.ONE) {
      await this.replayCurrentSong();
      return;
    }
    let nextSong = this.playlistService.next();
    if (!nextSong && this.playlistService.radioActive()) {
      // Queue drained while /recommend was still in flight — wait, don't hard-stop.
      const gotMore = await this.radioService.ensureMoreTracks();
      if (gotMore) nextSong = this.playlistService.next();
    }
    if (nextSong) {
      await this.setSong(nextSong, { fromQueue: true });
    } else if (mode === RepeatMode.ALL) {
      this.playlistService.jumpToIndex(0);
      const firstSong = this.playlistService.currentSong();
      if (firstSong) await this.setSong(firstSong, { fromQueue: true });
    } else {
      this.audioService.pause();
      this.audioService.seek(0);
      if (this.playlistService.radioActive()) {
        this.toast.show("Radio reached the end");
      }
    }
  }

  /** Queue advance hit a dead URL — skip ahead. User picks stay on Error. */
  private async handleSongError(): Promise<void> {
    const budget = Math.min(
      MAX_ERROR_SKIPS,
      Math.max(1, this.playlistService.queueLength()),
    );
    let skipped = 0;
    while (
      this.allowErrorSkip &&
      this.status() === PlayerStatus.Error &&
      this.consecutiveErrorSkips < budget
    ) {
      this.consecutiveErrorSkips++;
      skipped++;
      const advanced = await this.setNextSong();
      if (!advanced) break;
    }
    if (skipped > 0 && this.status() !== PlayerStatus.Error) {
      this.toast.show(
        skipped === 1
          ? "Skipped unavailable track"
          : `Skipped ${skipped} unavailable tracks`,
      );
    }
    if (this.status() === PlayerStatus.Error) {
      this.allowErrorSkip = false;
      this.consecutiveErrorSkips = 0;
      if (!this.playError()) {
        this.playError.set("Couldn't play this track");
      }
    }
  }

  async setSong(
    song: Song,
    opts?: { fromQueue?: boolean },
  ): Promise<void> {
    await this.loadSong(song, {
      fromQueue: opts?.fromQueue === true,
      autoplay: true,
    });
  }

  /** Explore / search / vault / history — same Song shape, no remap. */
  async playFromList(songs: Song[], song: Song): Promise<void> {
    this.playlistService.setCurrentPlaylist(songs);
    await this.setSong(song);
  }

  /** @returns load generation that won (for restore seek guard). */
  private async loadSong(
    song: Song,
    opts: { fromQueue: boolean; autoplay: boolean },
  ): Promise<number> {
    const gen = ++this.loadGen;
    this.cleanupObjectUrl();
    // Don't clear handlingSongEnded here — effect finally owns it; clearing
    // mid-flight re-enters Ended→next and skips tracks.
    this.playError.set("");
    this.allowErrorSkip = opts.fromQueue;
    this.playlistService.setCurrentSong(song);
    let secureLink = upgradeToHttps(song.link);

    // Online: CDN first — never await Cache API before play() (breaks iOS auto-next).
    // Offline: vault blob only.
    let src = secureLink;
    if (this.settingsService.isNavigatorOffline()) {
      const offlineResponse =
        await this.offlineStorageService.isAvailableOffline(secureLink);
      if (gen !== this.loadGen) return gen;
      if (!offlineResponse) {
        this.playError.set("Not available offline");
        this.audioService.fail();
        this.persistNow();
        return gen;
      }
      const blob = await offlineResponse.blob();
      if (gen !== this.loadGen) return gen;
      this.currentObjectUrl = URL.createObjectURL(blob);
      src = this.currentObjectUrl;
    }

    if (gen !== this.loadGen) return gen;
    if (opts.autoplay) await this.audioService.playSource(src);
    else {
      this.audioService.setSource(src);
      this.audioService.markPaused();
    }
    if (gen === this.loadGen) {
      this.persistNow();
      this.warmNextTrack();
    }
    return gen;
  }

  /** Prefetch upcoming[0] into the standby audio element (online CDN only). */
  private warmNextTrack(): void {
    const next = this.playlistService.upcoming()[0];
    if (!next?.link || this.settingsService.isNavigatorOffline()) {
      this.audioService.clearPreload();
      return;
    }
    this.audioService.preload(upgradeToHttps(next.link));
  }

  /** Retry hard-reloads the current track when we're on Error/Ended. */
  async play(): Promise<void> {
    const status = this.status();
    if (status === PlayerStatus.Error || status === PlayerStatus.Ended) {
      const song = this.playlistService.currentSong();
      if (song) {
        await this.setSong(song);
        return;
      }
    }
    await this.audioService.play();
  }

  pause(): void {
    this.audioService.pause();
    this.persistNow();
  }
  seek(time: number): void {
    this.audioService.seek(time);
    this.persistNow();
  }
  toggleShuffle(): void {
    this.settingsService.toggleShuffle();
    if (this.settingsService.isShuffle()) {
      this.playlistService.enableShuffle();
    } else {
      this.playlistService.disableShuffle();
    }
  }
  toggleRepeat(): void {
    this.settingsService.toggleRepeat();
  }
  async setPreviousSong(): Promise<Song | undefined> {
    if (this.currentTime() > 5) {
      return this.replayCurrentSong();
    }
    const previousSong = this.playlistService.previous();
    if (previousSong) {
      await this.setSong(previousSong, { fromQueue: true });
      return previousSong;
    } else if (this.settingsService.repeatMode() === RepeatMode.ALL) {
      const length = this.playlistService.queueLength();
      if (length > 0) {
        this.playlistService.jumpToIndex(length - 1);
        const lastSong = this.playlistService.currentSong();
        if (lastSong) {
          await this.setSong(lastSong, { fromQueue: true });
          return lastSong;
        }
      }
    }
    return undefined;
  }
  async setNextSong(): Promise<Song | undefined> {
    let nextSong = this.playlistService.next();
    if (!nextSong && this.playlistService.radioActive()) {
      const gotMore = await this.radioService.ensureMoreTracks();
      if (gotMore) nextSong = this.playlistService.next();
    }
    if (nextSong) {
      await this.setSong(nextSong, { fromQueue: true });
      return nextSong;
    } else if (this.settingsService.repeatMode() === RepeatMode.ALL) {
      this.playlistService.jumpToIndex(0);
      const firstSong = this.playlistService.currentSong();
      if (firstSong) {
        await this.setSong(firstSong, { fromQueue: true });
        return firstSong;
      }
    }
    return undefined;
  }

  playNext(song: Song): void {
    if (this.playlistService.playNext(song)) {
      this.toast.show("Playing next");
    }
  }

  addToQueue(song: Song): void {
    if (this.playlistService.addToQueue(song)) {
      this.toast.show("Added to queue");
    } else {
      this.toast.show("Already in queue");
    }
  }

  private async replayCurrentSong(): Promise<undefined> {
    this.audioService.seek(0);
    this.allowErrorSkip = false;
    await this.audioService.play();
    return undefined;
  }
  private cleanupObjectUrl(): void {
    if (this.currentObjectUrl) {
      URL.revokeObjectURL(this.currentObjectUrl);
      this.currentObjectUrl = null;
    }
  }
  private startPersistLoop(): void {
    if (this.persistTimerId != null) return;
    this.persistNow();
    this.persistTimerId = setInterval(() => {
      this.persistNow();
      this.recordListenTick();
    }, PERSIST_EVERY_MS);
  }
  private stopPersistLoop(): void {
    if (this.persistTimerId == null) return;
    clearInterval(this.persistTimerId);
    this.persistTimerId = null;
  }
  private persistNow(): void {
    this.playlistService.persist(this.currentTime());
  }
  private recordListenTick(): void {
    if (this.status() !== PlayerStatus.Playing) return;
    const link = this.playlistService.currentSong()?.link;
    if (link) this.storage.recordListenMs(link, PERSIST_EVERY_MS);
  }
  private onVisibility(): void {
    if (document.visibilityState === "hidden") {
      this.persistNow();
      return;
    }
    // Foreground again — poke Fiber (Render sleep) + re-arm wake lock / preload.
    this.settingsService.nudgeWake();
    // Resume auto-next that the browser blocked while backgrounded.
    if (this.audioService.hasPlayBlocked()) {
      void this.audioService.play();
    }
    if (this.status() === PlayerStatus.Playing) {
      void this.requestWakeLock();
      this.warmNextTrack();
    }
  }
  private async requestWakeLock(): Promise<void> {
    // ponytail: Screen Wake Lock — no-op where unsupported (Safari < 16.4)
    const wl = navigator.wakeLock;
    if (!wl || this.status() !== PlayerStatus.Playing) return;
    if (this.wakeLock) return;
    const gen = ++this.wakeLockGen;
    try {
      const lock = await wl.request("screen");
      if (gen !== this.wakeLockGen || this.status() !== PlayerStatus.Playing) {
        void lock.release();
        return;
      }
      this.wakeLock = lock;
      lock.addEventListener("release", () => {
        if (this.wakeLock === lock) this.wakeLock = null;
      });
    } catch {
      if (gen === this.wakeLockGen) this.wakeLock = null;
    }
  }
  private async releaseWakeLock(): Promise<void> {
    this.wakeLockGen++;
    const lock = this.wakeLock;
    this.wakeLock = null;
    try {
      await lock?.release();
    } catch {
      /* ignore */
    }
  }
  reset(): void {
    this.loadGen++;
    this.stopPersistLoop();
    this.audioService.pause();
    this.audioService.setSource("");
    this.audioService.clearPreload();
    this.playlistService.reset();
    this.cleanupObjectUrl();
    this.allowErrorSkip = false;
    this.consecutiveErrorSkips = 0;
    this.playError.set("");
    void this.releaseWakeLock();
  }
  ngOnDestroy(): void {
    this.loadGen++;
    this.cleanupObjectUrl();
    this.stopPersistLoop();
    if (typeof document !== "undefined") {
      document.removeEventListener("visibilitychange", this.visibilityHandler);
    }
    void this.releaseWakeLock();
  }
}
