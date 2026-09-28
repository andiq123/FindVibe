import { Injectable, signal, computed, inject } from "@angular/core";
import { Song, sameSong } from "../models/song.model";
import { shuffleArray } from "../utils/utils";
import { SettingsService } from "./settings.service";
import { StorageService } from "./storage.service";

const SESSION_KEY = "playerSession";

type PlayerSession = {
  songs: Song[];
  index: number;
  radio: boolean;
  t: number;
};

@Injectable({
  providedIn: "root",
})
export class PlaylistService {
  private readonly settingsService = inject(SettingsService);
  private readonly storage = inject(StorageService);
  private readonly originalList = signal<Song[]>([]);
  private readonly queue = signal<Song[]>([]);
  private readonly currentIndex = signal<number>(-1);
  private readonly _radioActive = signal(false);
  /** Resume offset from last session (seconds). Cleared after restore. */
  private pendingSeek = 0;
  readonly radioActive = this._radioActive.asReadonly();
  readonly queueLength = computed(() => this.queue().length);
  readonly currentSong = computed(() => {
    const q = this.queue();
    const i = this.currentIndex();
    return i >= 0 && i < q.length ? q[i] : null;
  });
  /** Tracks left after now-playing (for infinite radio prefetch). */
  readonly remaining = computed(() => {
    const i = this.currentIndex();
    if (i < 0) return 0;
    return Math.max(0, this.queue().length - i - 1);
  });
  /** Everything after now-playing (Up next editor). */
  readonly upcoming = computed(() => {
    const q = this.queue();
    const i = this.currentIndex();
    if (i < 0) return [];
    return q.slice(i + 1);
  });
  queueLinks(): string[] {
    return this.queue().map((s) => s.link);
  }
  queueSongs(): Song[] {
    return this.queue();
  }
  takePendingSeek(): number {
    const t = this.pendingSeek;
    this.pendingSeek = 0;
    return t;
  }
  setCurrentSong(song: Song | null): void {
    if (!song) {
      this.currentIndex.set(-1);
      return;
    }
    this.currentIndex.set(indexByLink(this.queue(), song.link));
    // Track change resets position — caller persists with currentTime after load.
    this.persist(0);
  }
  /** Search / library / recent — exits radio mode. */
  setCurrentPlaylist(songs: Song[]): void {
    this._radioActive.set(false);
    this.applyPlaylist(songs);
    this.persist(0);
  }
  /** Radio start — arms infinite extend. */
  setRadioPlaylist(songs: Song[]): void {
    this._radioActive.set(true);
    this.applyPlaylist(songs);
    this.persist(0);
  }
  appendSongs(songs: Song[]): number {
    if (!songs.length) return 0;
    const seen = new Set(this.queue().map((s) => s.link));
    const add = songs.filter((s) => s.link && !seen.has(s.link));
    if (!add.length) return 0;
    this.queue.update((q) => [...q, ...add]);
    this.originalList.update((q) => [...q, ...add]);
    this.persist();
    return add.length;
  }
  /** Insert right after now-playing (Play next). Dedupes by link. */
  playNext(song: Song): boolean {
    if (!song?.link) return false;
    const q = this.queue();
    const i = this.currentIndex();
    if (i < 0 || !q.length) {
      // Idle — queue for later; no now-playing index to persist.
      this._radioActive.set(false);
      this.originalList.set([song]);
      this.queue.set([song]);
      this.currentIndex.set(-1);
      return true;
    }
    const without = q.filter((s) => s.link !== song.link);
    const cur = without.findIndex((s) => s.link === q[i].link);
    const at = cur < 0 ? without.length : cur + 1;
    const next = [...without.slice(0, at), song, ...without.slice(at)];
    this.queue.set(next);
    this.originalList.update((list) => {
      const base = list.filter((s) => s.link !== song.link);
      const oi = base.findIndex((s) => s.link === q[i].link);
      const oAt = oi < 0 ? base.length : oi + 1;
      return [...base.slice(0, oAt), song, ...base.slice(oAt)];
    });
    this.currentIndex.set(next.findIndex((s) => s.link === q[i].link));
    this.persist();
    return true;
  }
  /** Append to end of queue (Add to queue). */
  addToQueue(song: Song): boolean {
    return this.appendSongs([song]) > 0;
  }
  /** Remove a future track by link (won't touch now-playing). */
  removeUpcoming(link: string): void {
    if (!link) return;
    const i = this.currentIndex();
    const cur = this.queue()[i];
    this.queue.update((q) =>
      q.filter((s, idx) => idx <= i || s.link !== link),
    );
    this.originalList.update((q) =>
      q.filter((s) => s.link !== link || s.link === cur?.link),
    );
    this.persist();
  }
  /** Drop everything after now-playing. */
  clearUpcoming(): void {
    const i = this.currentIndex();
    if (i < 0) {
      this.reset();
      return;
    }
    this.queue.update((q) => q.slice(0, i + 1));
    const keep = new Set(this.queue().map((s) => s.link));
    this.originalList.update((q) => q.filter((s) => keep.has(s.link)));
    this.persist();
  }
  /** Move upcoming track up/down within the future slice. */
  moveUpcoming(link: string, dir: -1 | 1): void {
    const q = [...this.queue()];
    const i = this.currentIndex();
    const from = q.findIndex((s, idx) => idx > i && s.link === link);
    if (from < 0) return;
    const to = from + dir;
    if (to <= i || to >= q.length) return;
    const [row] = q.splice(from, 1);
    q.splice(to, 0, row);
    this.queue.set(q);
    // Keep shuffle source in sync so up-next edits survive shuffle toggle.
    this.originalList.set(reorderByLinks(this.originalList(), q));
    this.persist();
  }
  private applyPlaylist(songs: Song[]): void {
    const current = this.currentSong(); // capture before queue swap
    this.originalList.set(songs);
    if (this.settingsService.isShuffle()) {
      this.queue.set(keepCurrentFirst(songs, current));
      this.currentIndex.set(songs.length ? 0 : -1);
    } else {
      this.queue.set(songs);
      this.currentIndex.set(current ? indexByLink(songs, current.link) : -1);
    }
  }
  enableShuffle(): void {
    this.queue.set(keepCurrentFirst(this.originalList(), this.currentSong()));
    this.currentIndex.set(this.queue().length ? 0 : -1);
    this.persist();
  }
  disableShuffle(): void {
    const current = this.currentSong();
    const list = this.originalList();
    this.queue.set(list);
    this.currentIndex.set(current ? indexByLink(list, current.link) : -1);
    this.persist();
  }
  next(): Song | null {
    const q = this.queue();
    if (q.length === 0) return null;
    const nextIndex = this.currentIndex() + 1;
    if (nextIndex >= q.length) return null;
    this.currentIndex.set(nextIndex);
    this.persist(0);
    return q[nextIndex];
  }
  previous(): Song | null {
    const q = this.queue();
    if (q.length === 0) return null;
    const prevIndex = this.currentIndex() - 1;
    if (prevIndex < 0) return null;
    this.currentIndex.set(prevIndex);
    this.persist(0);
    return q[prevIndex];
  }
  jumpToIndex(index: number): void {
    if (index >= 0 && index < this.queue().length) {
      this.currentIndex.set(index);
      this.persist(0);
    }
  }
  reset(): void {
    this._radioActive.set(false);
    this.originalList.set([]);
    this.queue.set([]);
    this.currentIndex.set(-1);
    this.pendingSeek = 0;
    this.storage.removeItem(SESSION_KEY);
  }

  /** Keep now-playing / queue art in sync after vault cover persist. */
  patchSongImage(link: string, image: string): void {
    const patch = (list: Song[]) =>
      list.map((s) => (s.link === link ? { ...s, image } : s));
    this.queue.update(patch);
    this.originalList.update(patch);
    // ponytail: art patch doesn't need session rewrite — next persist picks it up
  }

  /** Restore queue + index from localStorage. Does not start audio. */
  restoreSession(): boolean {
    const s = this.storage.getItem<PlayerSession>(SESSION_KEY);
    if (!s?.songs?.length) return false;
    const index = Math.min(Math.max(0, s.index ?? 0), s.songs.length - 1);
    this._radioActive.set(!!s.radio);
    this.originalList.set(s.songs);
    this.queue.set(s.songs);
    this.currentIndex.set(index);
    this.pendingSeek = Number.isFinite(s.t) && s.t > 0 ? s.t : 0;
    return true;
  }

  /**
   * Snapshot queue for continue-listening.
   * Pass `t` when position is known; omit to keep previous offset (same track only).
   * Index/song changes must call with `t: 0`.
   */
  persist(t?: number): void {
    const songs = this.queue();
    const index = this.currentIndex();
    if (!songs.length || index < 0) return;
    const prev = this.storage.getItem<PlayerSession>(SESSION_KEY);
    const sameTrack = prev?.index === index && prev?.songs?.[index]?.link === songs[index]?.link;
    const time =
      t != null && Number.isFinite(t)
        ? t
        : sameTrack
          ? (prev?.t ?? 0)
          : 0;
    // Strip lyrics — vault-sized text doesn't belong in session snapshots.
    const slim = songs.map(slimSong);
    this.storage.setItem(SESSION_KEY, {
      songs: slim,
      index,
      radio: this._radioActive(),
      t: time,
    } satisfies PlayerSession);
  }
}

function slimSong(s: Song): Song {
  if (!s.lyrics) return s;
  const { lyrics: _drop, ...rest } = s;
  return rest;
}

function indexByLink(songs: Song[], link: string): number {
  if (!link) return -1;
  return songs.findIndex((s) => s.link === link);
}

/** Reorder `base` to follow `order` link sequence; append leftovers. */
function reorderByLinks(base: Song[], order: Song[]): Song[] {
  const byLink = new Map(base.map((s) => [s.link, s]));
  const out: Song[] = [];
  for (const s of order) {
    const hit = byLink.get(s.link);
    if (hit) {
      out.push(hit);
      byLink.delete(s.link);
    } else {
      out.push(s);
    }
  }
  for (const left of byLink.values()) out.push(left);
  return out;
}

/** Shuffle but pin now-playing at front (link identity — explore uuids churn). */
function keepCurrentFirst(songs: Song[], current: Song | null): Song[] {
  let shuffled = shuffleArray(songs);
  if (!current?.link) return shuffled;
  shuffled = shuffled.filter((s) => !sameSong(s, current));
  shuffled.unshift(current);
  return shuffled;
}
