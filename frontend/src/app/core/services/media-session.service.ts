import { Injectable, inject, effect, OnDestroy, untracked } from "@angular/core";
import { Title } from "@angular/platform-browser";
import { PlaylistService } from "./playlist.service";
import { PlayerService } from "./player.service";
import { SettingsService } from "./settings.service";
import {
  PlayerStatus,
  RepeatMode,
} from "../../features/player/models/player.model";
import { Song } from "../models/song.model";

const APP_TITLE = "FindVibe";

const MEDIA_ACTIONS: MediaSessionAction[] = [
  "play",
  "pause",
  "stop",
  "previoustrack",
  "nexttrack",
  "seekbackward",
  "seekforward",
  "seekto",
];

/** Absolute https/blob URL for lock-screen artwork, or "". */
export function mediaArtworkSrc(image: string): string {
  const raw = image?.trim();
  if (!raw || raw === "no_album_art.jpg") return "";
  try {
    const u = new URL(
      raw,
      typeof location !== "undefined" ? location.href : "https://localhost/",
    );
    if (u.protocol === "http:") u.protocol = "https:";
    if (u.protocol !== "https:" && u.protocol !== "blob:") return "";
    return u.href;
  } catch {
    return "";
  }
}

/**
 * Keep Now Playing alive across Ended→Loading→Playing gaps (iOS PWA).
 * Error with a current track stays "paused" so skip/retry doesn't dismiss the session.
 */
export function mediaPlaybackState(
  status: PlayerStatus,
  hasSong = false,
): MediaSessionPlaybackState {
  switch (status) {
    case PlayerStatus.Playing:
    case PlayerStatus.Loading:
    case PlayerStatus.Ended:
      return "playing";
    case PlayerStatus.Paused:
      return "paused";
    case PlayerStatus.Error:
      return hasSong ? "paused" : "none";
    default:
      return "none";
  }
}

/** Browser / PWA tab title from now-playing — pure for the self-check. */
export function tabTitle(
  song: Pick<Song, "title" | "artist"> | null | undefined,
  status: PlayerStatus,
  app = APP_TITLE,
): string {
  const title = safeLabel(song?.title ?? "");
  if (!title || status === PlayerStatus.Stopped) return app;
  const artist = safeLabel(song?.artist ?? "") || "Unknown";
  const track = `${title} · ${artist}`;
  if (status === PlayerStatus.Paused || status === PlayerStatus.Error) {
    return `❚❚ ${track} · ${app}`;
  }
  return `${track} · ${app}`;
}

function safeLabel(raw: string, max = 64): string {
  return raw.replace(/[\u0000-\u001f\u007f]+/g, "").trim().slice(0, max);
}

@Injectable({
  providedIn: "root",
})
export class MediaSessionService implements OnDestroy {
  private readonly playlist = inject(PlaylistService);
  private readonly player = inject(PlayerService);
  private readonly settings = inject(SettingsService);
  private readonly title = inject(Title);
  private lastMetaKey = "";
  private lastStatus: PlayerStatus | null = null;
  private lastTabTitle = "";
  private lastCanNext: boolean | null = null;
  private handlersBound = false;
  private positionTimer: ReturnType<typeof setInterval> | null = null;

  constructor() {
    effect(() => {
      const song = this.playlist.currentSong();
      const status = this.player.status();
      // remaining + repeat gate next-track availability on the lock screen
      const remaining = this.playlist.remaining();
      const repeat = this.settings.repeatMode();
      untracked(() => {
        this.syncTabTitle(song, status);
        if ("mediaSession" in navigator) {
          this.sync(song, status, remaining, repeat);
        }
      });
    });
  }

  initialize(): void {
    if (!("mediaSession" in navigator) || this.handlersBound) return;
    this.handlersBound = true;
    const ms = navigator.mediaSession;
    const seekBy = (offset: number) => {
      const next = this.player.currentTime() + offset;
      const duration = this.player.duration();
      this.player.seek(
        Math.max(0, duration > 0 ? Math.min(next, duration) : next),
      );
      this.pushPosition();
    };
    const handlers: [MediaSessionAction, MediaSessionActionHandler][] = [
      ["play", () => void this.player.play()],
      ["pause", () => this.player.pause()],
      ["stop", () => this.player.pause()],
      ["previoustrack", () => void this.player.setPreviousSong()],
      ["nexttrack", () => void this.player.setNextSong()],
      ["seekbackward", (d) => seekBy(-(d.seekOffset || 10))],
      ["seekforward", (d) => seekBy(d.seekOffset || 10)],
      [
        "seekto",
        (d) => {
          if (d.seekTime != null) {
            this.player.seek(d.seekTime);
            this.pushPosition();
          }
        },
      ],
    ];
    for (const [action, handler] of handlers) {
      this.setAction(action, handler);
    }
  }

  private syncTabTitle(
    song: Song | null | undefined,
    status: PlayerStatus,
  ): void {
    const next = tabTitle(song, status);
    if (next === this.lastTabTitle) return;
    this.lastTabTitle = next;
    this.title.setTitle(next);
  }

  private sync(
    song: Song | null | undefined,
    status: PlayerStatus,
    remaining: number,
    repeat: RepeatMode,
  ): void {
    const ms = navigator.mediaSession;
    const hasSong = !!song?.link;
    const key = song
      ? `${song.link}\0${song.image}\0${song.title}\0${song.artist}`
      : "";
    const metaChanged = key !== this.lastMetaKey;
    if (metaChanged) {
      this.lastMetaKey = key;
      if (song) {
        const src = mediaArtworkSrc(song.image);
        ms.metadata = new MediaMetadata({
          title: song.title || "Unknown",
          artist: song.artist || "Unknown",
          // omit type — covers are often jpeg/webp, wrong mime drops artwork
          artwork: src
            ? [
                { src, sizes: "512x512" },
                { src, sizes: "256x256" },
              ]
            : [],
        });
      } else {
        ms.metadata = null;
      }
    }

    const statusChanged = status !== this.lastStatus;
    if (statusChanged) {
      this.lastStatus = status;
      ms.playbackState = mediaPlaybackState(status, hasSong);
    }

    const playing = status === PlayerStatus.Playing;
    const keepAlive =
      playing ||
      status === PlayerStatus.Loading ||
      status === PlayerStatus.Ended ||
      (status === PlayerStatus.Error && hasSong);

    this.armPositionTimer(playing);
    if (playing || (metaChanged && keepAlive)) {
      this.pushPosition();
    } else if (statusChanged && !keepAlive) {
      try {
        ms.setPositionState({});
      } catch {
        /* ignore */
      }
    }

    this.syncNextAction(remaining, repeat);
  }

  /** Hide/disable next on lock screen when the queue can't advance. */
  private syncNextAction(remaining: number, repeat: RepeatMode): void {
    if (!this.handlersBound) return;
    const canNext =
      remaining > 0 ||
      repeat === RepeatMode.ALL ||
      this.playlist.radioActive();
    if (canNext === this.lastCanNext) return;
    this.lastCanNext = canNext;
    this.setAction(
      "nexttrack",
      canNext ? () => void this.player.setNextSong() : null,
    );
  }

  private setAction(
    action: MediaSessionAction,
    handler: MediaSessionActionHandler | null,
  ): void {
    try {
      navigator.mediaSession.setActionHandler(action, handler);
    } catch {
      // ponytail: not every action is supported on every OS
    }
  }

  private armPositionTimer(playing: boolean): void {
    if (playing) {
      if (this.positionTimer == null) {
        this.positionTimer = setInterval(() => this.pushPosition(), 1000);
      }
      return;
    }
    if (this.positionTimer != null) {
      clearInterval(this.positionTimer);
      this.positionTimer = null;
    }
  }

  private pushPosition(): void {
    const ms = navigator.mediaSession;
    if (!("setPositionState" in ms)) return;
    const duration = this.player.duration();
    const position = this.player.currentTime();
    // setPositionState throws on NaN / position > duration
    if (
      !(duration > 0) ||
      !Number.isFinite(duration) ||
      !Number.isFinite(position)
    ) {
      return;
    }
    try {
      ms.setPositionState({
        duration,
        playbackRate: 1,
        position: Math.min(Math.max(0, position), duration),
      });
    } catch {
      /* ignore */
    }
  }

  ngOnDestroy(): void {
    if (this.positionTimer != null) {
      clearInterval(this.positionTimer);
      this.positionTimer = null;
    }
    if (this.handlersBound && "mediaSession" in navigator) {
      for (const action of MEDIA_ACTIONS) {
        this.setAction(action, null);
      }
      this.handlersBound = false;
    }
    if (this.lastTabTitle !== APP_TITLE) {
      this.title.setTitle(APP_TITLE);
    }
  }
}
