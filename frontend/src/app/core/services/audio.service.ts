import { Injectable, signal, OnDestroy, inject } from "@angular/core";
import { PlayerStatus } from "../../features/player/models/player.model";
import { StorageService } from "./storage.service";
import { PlaylistService } from "./playlist.service";

function sameAudioSrc(el: HTMLAudioElement, src: string): boolean {
  if (!src || !el.getAttribute("src")) return false;
  try {
    const base =
      typeof location !== "undefined" ? location.href : "https://localhost/";
    return new URL(el.src).href === new URL(src, base).href;
  } catch {
    return el.src === src;
  }
}

function makeAudio(): HTMLAudioElement {
  const audio = new Audio();
  audio.volume = 1.0;
  audio.preload = "auto";
  // iOS: keep playback eligible for lock-screen / Now Playing controls
  audio.setAttribute("playsinline", "true");
  audio.setAttribute("webkit-playsinline", "true");
  return audio;
}

@Injectable({
  providedIn: "root",
})
export class AudioService implements OnDestroy {
  private readonly storageService = inject(StorageService);
  private readonly playlistService = inject(PlaylistService);
  /** Now-playing element. */
  private audio?: HTMLAudioElement;
  /** Warm next track while current plays (iOS background auto-next). */
  private preloadEl?: HTMLAudioElement;
  private abortController?: AbortController;
  private alreadyAddedToRecents = false;
  /** Background auto-next hit NotAllowedError — retry on foreground / MS play. */
  private playBlocked = false;
  readonly status = signal<PlayerStatus>(PlayerStatus.Stopped);
  readonly currentTime = signal<number>(0);
  readonly duration = signal<number>(0);

  initialize(): void {
    if (this.audio) return;
    this.audio = makeAudio();
    this.preloadEl = makeAudio();
    this.abortController = new AbortController();
    const signal = this.abortController.signal;
    this.bind(this.audio, signal);
    this.bind(this.preloadEl, signal);
  }

  /** Ignore standby element events — only the active `audio` drives status. */
  private bind(el: HTMLAudioElement, signal: AbortSignal): void {
    el.addEventListener(
      "playing",
      (e) => {
        if (e.target !== this.audio) return;
        this.playBlocked = false;
        this.status.set(PlayerStatus.Playing);
      },
      { signal },
    );
    el.addEventListener(
      "pause",
      (e) => {
        if (e.target !== this.audio) return;
        const s = this.status();
        // Src/load() emits pause before loadstart — don't latch car/MS to paused.
        if (
          s === PlayerStatus.Loading ||
          s === PlayerStatus.Error ||
          s === PlayerStatus.Ended
        ) {
          return;
        }
        this.status.set(PlayerStatus.Paused);
      },
      { signal },
    );
    el.addEventListener(
      "ended",
      (e) => {
        if (e.target !== this.audio) return;
        this.status.set(PlayerStatus.Ended);
      },
      { signal },
    );
    el.addEventListener(
      "timeupdate",
      (e) => {
        if (e.target !== this.audio || !this.audio) return;
        const time = this.audio.currentTime;
        this.currentTime.set(time);
        if (this.audio.duration && this.audio.duration !== this.duration()) {
          this.duration.set(this.audio.duration);
        }
        if (time > 7 && !this.alreadyAddedToRecents) {
          const song = this.playlistService.currentSong();
          if (song) {
            this.alreadyAddedToRecents = true;
            this.storageService.addSongToRecents(song);
          }
        }
      },
      { signal },
    );
    el.addEventListener(
      "loadstart",
      (e) => {
        if (e.target !== this.audio) return;
        this.status.set(PlayerStatus.Loading);
      },
      { signal },
    );
    el.addEventListener(
      "error",
      (e) => {
        if (e.target !== this.audio) return;
        // Src swap aborts the previous load — not a real failure.
        if (this.audio?.error?.code === MediaError.MEDIA_ERR_ABORTED) return;
        this.status.set(PlayerStatus.Error);
      },
      { signal },
    );
  }

  setSource(src: string): void {
    if (!this.audio) this.initialize();
    if (!this.audio) return;
    this.alreadyAddedToRecents = false;
    // Before src/load() so the abort-pause can't flip status to Paused.
    if (src) this.status.set(PlayerStatus.Loading);
    this.audio.src = src;
    this.audio.load();
  }

  /** Warm the standby element for the next queue item (CDN or blob URL). */
  preload(src: string): void {
    if (!this.preloadEl) this.initialize();
    if (!this.preloadEl) return;
    if (!src) {
      this.clearPreload();
      return;
    }
    if (sameAudioSrc(this.preloadEl, src)) return;
    this.preloadEl.src = src;
    this.preloadEl.load();
  }

  clearPreload(): void {
    if (!this.preloadEl?.getAttribute("src")) return;
    this.preloadEl.removeAttribute("src");
    this.preloadEl.load();
  }

  /**
   * Play `src`, swapping in the preloaded element when it's ready so iOS
   * keeps the media session across track boundaries.
   */
  async playSource(src: string): Promise<void> {
    if (!this.audio) this.initialize();
    if (!this.audio || !this.preloadEl) return;
    if (
      sameAudioSrc(this.preloadEl, src) &&
      this.preloadEl.readyState >= HTMLMediaElement.HAVE_CURRENT_DATA
    ) {
      const old = this.audio;
      this.audio = this.preloadEl;
      this.preloadEl = old;
      this.alreadyAddedToRecents = false;
      this.currentTime.set(0);
      this.duration.set(this.audio.duration || 0);
      old.pause();
      old.removeAttribute("src");
      old.load();
      await this.play();
      return;
    }
    this.setSource(src);
    await this.play();
  }

  /** Explicit failure without touching media src (offline / policy). */
  fail(): void {
    this.status.set(PlayerStatus.Error);
  }

  /** Continue-listening restore — loaded but not playing. */
  markPaused(): void {
    this.status.set(PlayerStatus.Paused);
  }

  async play(): Promise<void> {
    if (!this.audio) return;
    try {
      await this.audio.play();
      this.playBlocked = false;
    } catch (e) {
      if (e instanceof DOMException) {
        // Interrupted by a newer setSong — ignore.
        if (e.name === "AbortError") return;
        // Background auto-next often blocked — keep Loading so car/MS don't
        // latch paused; retry on foreground or Media Session play.
        if (e.name === "NotAllowedError") {
          this.playBlocked = true;
          const s = this.status();
          if (s !== PlayerStatus.Loading && s !== PlayerStatus.Ended) {
            this.status.set(PlayerStatus.Paused);
          }
          return;
        }
      }
      this.status.set(PlayerStatus.Error);
    }
  }

  hasPlayBlocked(): boolean {
    return this.playBlocked;
  }

  pause(): void {
    this.playBlocked = false;
    this.audio?.pause();
  }

  seek(time: number): void {
    if (this.audio) {
      this.audio.currentTime = time;
    }
  }

  ngOnDestroy(): void {
    for (const el of [this.audio, this.preloadEl]) {
      if (!el) continue;
      el.pause();
      el.removeAttribute("src");
      el.load();
    }
    this.audio = undefined;
    this.preloadEl = undefined;
    this.abortController?.abort();
  }
}
