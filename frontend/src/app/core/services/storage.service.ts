import { Injectable, signal } from "@angular/core";
import { Song } from "../models/song.model";
import { ListenStat, ListenStats } from "../utils/listen-rank";

const RECENT_SONGS_KEY = "recentLibrary";
const RECENT_SONGS_LIMIT = 50;
const LISTEN_STATS_KEY = "listenStats";
/** Cap keys so localStorage + signal don't grow without bound. */
const LISTEN_STATS_MAX = 200;
/** Batch disk writes — in-memory updates stay live for explore ranking. */
const LISTEN_PERSIST_MS = 30_000;

@Injectable({
  providedIn: "root",
})
export class StorageService {
  private readonly _recentSongs = signal<Song[]>(this.getRecentSongsFromStorage());
  readonly recentSongs = this._recentSongs.asReadonly();
  private readonly _listenStats = signal<ListenStats>(
    this.getItem<ListenStats>(LISTEN_STATS_KEY) || {},
  );
  readonly listenStats = this._listenStats.asReadonly();
  private listenPersistTimer: ReturnType<typeof setTimeout> | null = null;

  constructor() {
    if (typeof document === "undefined") return;
    document.addEventListener("visibilitychange", () => {
      if (document.visibilityState === "hidden") this.flushListenStats();
    });
  }

  setItem(key: string, value: unknown): void {
    localStorage.setItem(key, JSON.stringify(value));
  }
  getItem<T>(key: string): T | null {
    const data = localStorage.getItem(key);
    if (!data) return null;
    try {
      return JSON.parse(data) as T;
    } catch (e) {
      console.error(`Error parsing storage key "${key}":`, e);
      return null;
    }
  }
  removeItem(key: string): void {
    localStorage.removeItem(key);
  }
  addSongToRecents(song: Song): void {
    const currentSongs = this._recentSongs();
    const filtered = currentSongs.filter((s: Song) => s.link !== song.link);
    const updatedSongs = [song, ...filtered].slice(0, RECENT_SONGS_LIMIT);
    this._recentSongs.set(updatedSongs);
    this.setItem(RECENT_SONGS_KEY, updatedSongs);
    this.bumpPlay(song.link);
  }
  clearRecents(): void {
    this._recentSongs.set([]);
    this.removeItem(RECENT_SONGS_KEY);
  }
  /** Accumulate listening time while a track is playing. */
  recordListenMs(link: string, ms: number): void {
    if (!link || ms < 400) return;
    this.patchListen(link, (prev) => ({
      ms: (prev?.ms ?? 0) + ms,
      plays: prev?.plays ?? 0,
      lastAt: Date.now(),
    }));
    this.scheduleListenPersist();
  }
  private bumpPlay(link: string): void {
    if (!link) return;
    this.patchListen(link, (prev) => ({
      ms: prev?.ms ?? 0,
      plays: (prev?.plays ?? 0) + 1,
      lastAt: Date.now(),
    }));
    this.flushListenStats();
  }
  private patchListen(
    link: string,
    next: (prev: ListenStat | undefined) => ListenStat,
  ): void {
    const updated = { ...this._listenStats(), [link]: next(this._listenStats()[link]) };
    this._listenStats.set(updated);
  }
  private scheduleListenPersist(): void {
    if (this.listenPersistTimer != null) return;
    this.listenPersistTimer = setTimeout(() => {
      this.listenPersistTimer = null;
      this.flushListenStats();
    }, LISTEN_PERSIST_MS);
  }
  private flushListenStats(): void {
    if (this.listenPersistTimer != null) {
      clearTimeout(this.listenPersistTimer);
      this.listenPersistTimer = null;
    }
    const pruned = pruneListenStats(this._listenStats(), LISTEN_STATS_MAX);
    if (pruned !== this._listenStats()) this._listenStats.set(pruned);
    this.setItem(LISTEN_STATS_KEY, pruned);
  }
  private getRecentSongsFromStorage(): Song[] {
    return this.getItem<Song[]>(RECENT_SONGS_KEY) || [];
  }
}

function pruneListenStats(stats: ListenStats, max: number): ListenStats {
  const keys = Object.keys(stats);
  if (keys.length <= max) return stats;
  keys.sort((a, b) => (stats[b].lastAt ?? 0) - (stats[a].lastAt ?? 0));
  const out: ListenStats = {};
  for (const k of keys.slice(0, max)) out[k] = stats[k];
  return out;
}
