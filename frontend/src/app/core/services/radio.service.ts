import {
  Injectable,
  inject,
  signal,
  effect,
  untracked,
} from "@angular/core";
import { HttpClient } from "@angular/common/http";
import { firstValueFrom } from "rxjs";
import { Song, songKey } from "../models/song.model";
import { PlaylistService } from "./playlist.service";
import { environment } from "../../../environments/environment";
import {
  ListenStats,
  pickVaultRadioSeed,
} from "../utils/listen-rank";
import { ToastService } from "./toast.service";

/** Prefetch early — backgrounded iOS may stall /recommend near empty queue. */
const EXTEND_WHEN_LEFT = 5;
/** How many first-batch neighbors stay in the style pool (plus the anchor). */
const STYLE_POOL = 6;

@Injectable({ providedIn: "root" })
export class RadioService {
  private readonly http = inject(HttpClient);
  private readonly playlist = inject(PlaylistService);
  private readonly toast = inject(ToastService);
  private extending = false;
  private lastExtendKey = "";
  /** Station origin — never abandon this vibe for whatever is currently playing. */
  private anchor: Song | null = null;
  /** Locked neighborhood: anchor + first recommend batch (cycle for extends). */
  private styleSeeds: Song[] = [];
  private seedCursor = 0;
  private extendRound = 0;
  /** Soft-exclude vault tracks so radio discovers new songs. */
  private excludeLinks = new Set<string>();
  private excludeKeys = new Set<string>();
  private lastVaultSeedKey = "";
  private vaultBump = 0;

  readonly loading = signal(false);
  /** Non-empty while a specific track’s radio button is starting (not header/station). */
  readonly loadingKey = signal("");
  readonly error = signal("");

  clearError(): void {
    this.error.set("");
  }

  /** True while this track’s row radio control should show a spinner. */
  isLoadingSong(song: Song): boolean {
    const key = songKey(song);
    return !!key && this.loading() && this.loadingKey() === key;
  }

  /** Header / full-player radio — busy when loading without a per-song focus. */
  isLoadingStation(): boolean {
    return this.loading() && !this.loadingKey();
  }

  constructor() {
    // Infinite radio — prefetch while staying in the anchored style pool.
    effect(() => {
      if (!this.playlist.radioActive()) {
        untracked(() => this.resetStation());
        return;
      }
      const left = this.playlist.remaining();
      if (left > EXTEND_WHEN_LEFT) return;
      untracked(() => void this.extend());
    });
  }

  /**
   * Vault radio: pick a smart liked seed, exclude vault tracks from the queue,
   * then discover similar songs (same station logic as player Radio).
   */
  async startFromVault(
    vault: Song[],
    stats: ListenStats,
  ): Promise<Song | null> {
    if (this.loading()) return null;
    if (!vault.length) {
      this.error.set("Add songs to your vault first");
      return null;
    }
    this.vaultBump++;
    const seed = pickVaultRadioSeed(
      vault,
      stats,
      this.vaultBump,
      this.lastVaultSeedKey,
    );
    if (!seed?.artist?.trim() || !seed?.title?.trim()) {
      this.error.set("Couldn't pick a seed from your vault");
      return null;
    }
    this.lastVaultSeedKey = songKey(seed);

    const excludeLinks = vault.map((s) => s.link).filter(Boolean);
    const excludeKeys = vault.map((s) => songKey(s)).filter(Boolean);
    // Station start — spinner on vault header, not a single row.
    const ok = await this.start(seed, { excludeLinks, excludeKeys, loadingKey: "" });
    return ok ? seed : null;
  }

  /** Replace playlist with seed + recommendations; arms radio mode. */
  async start(
    seed: Song,
    opts?: {
      excludeLinks?: string[];
      excludeKeys?: string[];
      /** Row focus key; omit/`""` = station control (header / full player). */
      loadingKey?: string;
    },
  ): Promise<boolean> {
    if (this.loading()) return false;
    if (!seed.artist?.trim() || !seed.title?.trim()) {
      this.error.set("Missing artist or title");
      return false;
    }
    this.loading.set(true);
    this.loadingKey.set(opts?.loadingKey ?? "");
    this.error.set("");

    const excludeLinks = new Set(opts?.excludeLinks?.filter(Boolean) ?? []);
    const excludeKeys = new Set(opts?.excludeKeys?.filter(Boolean) ?? []);
    // Anchor itself must stay playable even if it's in the vault.
    excludeLinks.delete(seed.link);
    const sk = songKey(seed);
    if (sk) excludeKeys.delete(sk);

    try {
      const seen = new Set<string>([seed.link, ...excludeLinks]);
      // Temporarily apply excludes for this fetch (station not committed yet).
      const prevLinks = this.excludeLinks;
      const prevKeys = this.excludeKeys;
      this.excludeLinks = excludeLinks;
      this.excludeKeys = excludeKeys;
      let next: Song[];
      try {
        next = await this.fetchUnique(seed, seen, 0);
      } finally {
        this.excludeLinks = prevLinks;
        this.excludeKeys = prevKeys;
      }
      if (!next.length) {
        this.error.set("No new radio tracks found");
        return false;
      }
      // Commit new station only after we have discoveries.
      this.resetStation();
      this.anchor = seed;
      this.excludeLinks = excludeLinks;
      this.excludeKeys = excludeKeys;
      this.styleSeeds = [seed, ...next.slice(0, STYLE_POOL - 1)];
      this.seedCursor = 0;
      this.playlist.setRadioPlaylist([seed, ...next]);
      this.playlist.setCurrentSong(seed);
      return true;
    } catch (e: unknown) {
      this.error.set(apiError(e));
      return false;
    } finally {
      this.loading.set(false);
      this.loadingKey.set("");
    }
  }

  private resetStation(): void {
    this.anchor = null;
    this.styleSeeds = [];
    this.seedCursor = 0;
    this.extendRound = 0;
    this.lastExtendKey = "";
    this.extending = false;
    this.excludeLinks = new Set();
    this.excludeKeys = new Set();
  }

  /** Cycle style seeds (anchor first) — never re-seed from a random now-playing track. */
  private pickExtendSeed(): Song | null {
    if (this.anchor) {
      // Every other extend prefers the original station seed.
      if (this.extendRound % 2 === 0) return this.anchor;
    }
    if (!this.styleSeeds.length) {
      return this.playlist.currentSong();
    }
    const s = this.styleSeeds[this.seedCursor % this.styleSeeds.length];
    this.seedCursor++;
    return s;
  }

  /**
   * End-of-queue while radio is on: wait for in-flight extend or kick one
   * so background playback doesn't hard-pause while /recommend is mid-flight.
   */
  async ensureMoreTracks(timeoutMs = 20_000): Promise<boolean> {
    if (!this.playlist.radioActive()) return false;
    if (this.playlist.remaining() > 0) return true;

    const deadline = Date.now() + timeoutMs;
    while (this.extending && Date.now() < deadline) {
      await sleep(150);
      if (this.playlist.remaining() > 0) return true;
    }
    if (this.playlist.remaining() > 0) return true;

    // Fresh attempts — clear key so extend isn't skipped as a duplicate.
    this.lastExtendKey = "";
    await this.extend();
    if (this.playlist.remaining() > 0) return true;

    if (Date.now() < deadline) {
      this.lastExtendKey = "";
      await this.extend();
    }
    return this.playlist.remaining() > 0;
  }

  private async extend(): Promise<void> {
    if (!this.playlist.radioActive() || this.extending) return;
    const seed = this.pickExtendSeed();
    if (!seed?.link || !seed.artist?.trim() || !seed.title?.trim()) return;

    const offset = this.extendRound;
    const extendKey = `${seed.link}\0${offset}`;
    if (extendKey === this.lastExtendKey) return;

    this.extending = true;
    this.lastExtendKey = extendKey;
    const showFinding = this.playlist.remaining() <= 1;
    if (showFinding) this.toast.hold("Finding more radio tracks…");
    try {
      const seen = new Set([
        ...this.playlist.queueLinks().filter(Boolean),
        ...this.excludeLinks,
      ] as string[]);
      let next = await this.fetchUnique(seed, seen, offset);
      // Same slice exhausted — nudge offset and try the anchor once more.
      if (!next.length && this.anchor && seed.link !== this.anchor.link) {
        next = await this.fetchUnique(this.anchor, seen, offset + 1);
      }
      if (next.length) {
        this.playlist.appendSongs(next);
        this.extendRound++;
        this.toast.show(
          next.length === 1
            ? "Added 1 song to radio"
            : `Added ${next.length} songs to radio`,
        );
      } else {
        this.lastExtendKey = "";
        this.extendRound++;
        // Leave end-of-queue copy to the player when the queue is drained.
        if (showFinding) this.toast.clear();
      }
    } catch {
      this.lastExtendKey = "";
      if (showFinding) this.toast.clear();
    } finally {
      this.extending = false;
    }
  }

  private async fetchUnique(
    seed: Song,
    seenLinks: Set<string>,
    offset: number,
  ): Promise<Song[]> {
    const songs = await firstValueFrom(
      this.http.get<Song[]>(`${environment.API_URL}/recommend`, {
        params: {
          artist: seed.artist,
          title: seed.title,
          mode: "radio",
          offset: String(Math.max(0, offset)),
        },
      }),
    );
    const keys = new Set<string>(
      [
        songKey(seed),
        ...this.excludeKeys,
        ...this.playlist.queueSongs().map(songKey),
      ].filter(Boolean),
    );
    const out: Song[] = [];
    for (const s of songs ?? []) {
      if (!s.link || seenLinks.has(s.link)) continue;
      const k = songKey(s);
      if (!k || keys.has(k)) continue;
      seenLinks.add(s.link);
      keys.add(k);
      out.push(s);
    }
    return out;
  }
}

function apiError(e: unknown): string {
  const err = e as {
    error?: { error?: string };
    message?: string;
    status?: number;
  };
  if (err?.status === 0) return "Can't reach the server";
  return err?.error?.error || "Couldn't start radio — try another track";
}

function sleep(ms: number): Promise<void> {
  return new Promise((r) => setTimeout(r, ms));
}
