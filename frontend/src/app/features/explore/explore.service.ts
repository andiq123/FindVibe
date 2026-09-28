import { Injectable, computed, inject, signal } from "@angular/core";
import { HttpClient } from "@angular/common/http";
import { firstValueFrom } from "rxjs";
import { Song, songKey } from "../../core/models/song.model";
import { environment } from "../../../environments/environment";
import { LibraryService } from "../library/services/library.service";
import { StorageService } from "../../core/services/storage.service";
import { rankByListen, rotateIndex } from "../../core/utils/listen-rank";

/** Client-only shelf ids — never passed back into Fiber chart merge. */
const LOCAL_SHELF = new Set(["vault", "recents", "because", "because2"]);
const BECAUSE_KEY = "exploreBecause";
const CHARTS_KEY = "exploreCharts";
/** Match Fiber exploreTTL / recommendTTL. */
const CHARTS_TTL_MS = 24 * 60 * 60 * 1000;
/** ponytail: /recommend is the expensive personalization hit — once/day per seed. */
const BECAUSE_TTL_MS = 24 * 60 * 60 * 1000;
const DAY_MS = 86_400_000;
const BECAUSE_SLOTS = 2;
/** Soft cap after Fiber resolve (handler returns up to recommendResolveCap). */
const BECAUSE_RAIL = 10;

export type ExploreSection = {
  id: string;
  title: string;
  subtitle: string;
  /** Already Fiber-resolved search songs (same Song as /search). */
  songs: Song[];
  /** Because shelf: seed track shown in the title (click to play). */
  seedSong?: Song;
  /** Show Start radio on the shelf header. */
  radio?: boolean;
};

export type ExploreResponse = {
  country: string;
  sections: ExploreSection[];
  cached?: boolean;
};

type BecauseCache = {
  seed: string;
  at: number;
  title: string;
  subtitle: string;
  songs: Song[];
  seedSong?: Song;
  slot?: number;
};

type BecauseStore = {
  at: number;
  items: BecauseCache[];
};

type ChartsCache = {
  at: number;
  country: string;
  sections: ExploreSection[];
};

@Injectable({ providedIn: "root" })
export class ExploreService {
  private readonly http = inject(HttpClient);
  private readonly library = inject(LibraryService);
  private readonly storage = inject(StorageService);

  readonly sections = signal<ExploreSection[]>([]);
  readonly country = signal("Romania");
  readonly loading = signal(false);
  readonly error = signal("");
  /** Personal shelves present — header says For you, not Charts. */
  readonly forYou = computed(() =>
    this.sections().some(
      (s) =>
        s.id === "because" ||
        s.id === "because2" ||
        s.id === "vault" ||
        s.id === "recents",
    ),
  );
  /** Thin library — nudge toward search/vault. */
  readonly needsTaste = computed(() => {
    const vault = this.library.songs().length;
    const recent = this.storage.recentSongs().length;
    return vault < 3 && recent < 3;
  });

  private loaded = false;
  /** Last chart payload from API (no local shelves). */
  private charts: ExploreSection[] = [];
  /** Personalized shelves — memory + localStorage 24h (up to 2 seeds). */
  private becauseSlots: (ExploreSection | null)[] = [null, null];
  private becauseSeeds: (string | null)[] = [null, null];
  private becauseFetchKey: string | null = null;
  private becauseFetchGen = 0;
  /** Manual refresh bumps seed rotation within the same day. */
  private seedBump = 0;

  /**
   * Paint from disk instantly, then revalidate /explore in the background.
   * Warm re-entry still ensures Because shelves (cache → /recommend).
   */
  async load(refresh = false): Promise<void> {
    if (this.loading()) {
      if (this.charts.length || this.hasBecause()) {
        this.hydrateBecauseFromCache();
        this.sections.set(this.merge(this.charts));
      }
      return;
    }

    if (!refresh && this.loaded && this.charts.length) {
      this.hydrateBecauseFromCache();
      this.sections.set(this.merge(this.charts));
      void this.ensureBecause(false);
      return;
    }

    if (!refresh) this.paintFromDisk();

    const painted = this.sections().length > 0;
    if (!painted) this.loading.set(true);
    this.error.set("");
    try {
      const r = await firstValueFrom(
        this.http.get<ExploreResponse>(`${environment.API_URL}/explore`, {
          params: refresh ? { refresh: "1" } : {},
        }),
      );
      this.charts = (r?.sections ?? []).filter((s) => !LOCAL_SHELF.has(s.id));
      this.country.set(r?.country || "Romania");
      this.persistCharts();
      this.hydrateBecauseFromCache();
      this.sections.set(this.merge(this.charts));
      this.loaded = true;
      if (refresh) await this.ensureBecause(true);
      else void this.ensureBecause(false);
    } catch {
      if (!this.charts.length) this.hydrateChartsFromCache(true);
      this.hydrateBecauseFromCache();
      const local = this.merge(this.charts);
      if (local.length) {
        this.sections.set(local);
        this.loaded = true;
      } else if (!this.sections().length) {
        this.error.set("Couldn't load charts");
      }
      void this.ensureBecause(false);
    } finally {
      this.loading.set(false);
    }
  }

  /** Manual refresh — bust caches, rotate seeds. */
  refresh(): Promise<void> {
    this.storage.removeItem(BECAUSE_KEY);
    this.storage.removeItem(CHARTS_KEY);
    this.becauseSlots = [null, null];
    this.becauseSeeds = [null, null];
    this.becauseFetchKey = null;
    this.becauseFetchGen++;
    this.seedBump++;
    this.charts = [];
    this.loaded = false;
    return this.load(true);
  }

  private hasBecause(): boolean {
    return this.becauseSlots.some((s) => !!s?.songs.length);
  }

  private paintFromDisk(): void {
    const hadCharts =
      this.hydrateChartsFromCache(false) || this.hydrateChartsFromCache(true);
    this.hydrateBecauseFromCache();
    if (hadCharts || this.hasBecause() || this.hasLocalShelves()) {
      this.sections.set(this.merge(this.charts));
    }
  }

  private hasLocalShelves(): boolean {
    return (
      this.library.songs().length >= 3 ||
      this.storage.recentSongs().length >= 3
    );
  }

  private hydrateChartsFromCache(allowStale: boolean): boolean {
    const c = this.storage.getItem<ChartsCache>(CHARTS_KEY);
    if (!c?.sections?.length) return false;
    if (!allowStale && Date.now() - c.at > CHARTS_TTL_MS) return false;
    this.charts = c.sections.filter((s) => !LOCAL_SHELF.has(s.id));
    if (c.country) this.country.set(c.country);
    return this.charts.length > 0;
  }

  private persistCharts(): void {
    if (!this.charts.length) return;
    this.storage.setItem(CHARTS_KEY, {
      at: Date.now(),
      country: this.country(),
      sections: this.charts,
    } satisfies ChartsCache);
  }

  /**
   * Because → Because2 → vault → recents → charts.
   * Cross-rail dedupe by link + soft songKey (first shelf wins).
   */
  private merge(charts: ExploreSection[]): ExploreSection[] {
    const head: ExploreSection[] = [];
    for (let i = 0; i < BECAUSE_SLOTS; i++) {
      const slot = this.becauseSlots[i];
      if (slot?.songs.length) head.push(slot);
    }
    const day = Math.floor(Date.now() / DAY_MS);
    const vault = rankByListen(
      this.library.songs(),
      this.storage.listenStats(),
      day,
    ).slice(0, 10);
    if (vault.length >= 3) {
      head.push({
        id: "vault",
        title: "From your vault",
        subtitle: "Favorites",
        songs: vault,
        seedSong: vault[0],
        radio: true,
      });
    }
    const recent = takeUnique(this.storage.recentSongs(), 10);
    if (recent.length >= 3) {
      head.push({
        id: "recents",
        title: "Jump back in",
        subtitle: "Recent listens",
        songs: recent,
      });
    }
    const seenIds = new Set(head.map((s) => s.id));
    const body = charts.filter((s) => !seenIds.has(s.id));
    return dedupeSections([...head, ...body]);
  }

  private seedKeyOf(song: Song): string {
    return `${song.artist.trim()}\0${song.title.trim()}`;
  }

  private slotId(slot: number): string {
    return slot === 0 ? "because" : "because2";
  }

  /**
   * Rotate seed daily across vault; slot 1 picks a different seed.
   */
  private pickSeedSong(slot: number, excludeKeys: Set<string>): Song | null {
    const day = Math.floor(Date.now() / DAY_MS);
    const vault = this.library.songs();
    const pool = vault.length
      ? rankByListen(vault, this.storage.listenStats(), day)
      : takeUnique(this.storage.recentSongs(), 50);
    if (!pool.length) return null;

    for (let attempt = 0; attempt < pool.length; attempt++) {
      const idx = rotateIndex(pool.length, day, this.seedBump + slot * 5 + attempt);
      const pick = pool[idx];
      if (!pick?.artist?.trim() || !pick?.title?.trim()) continue;
      const key = this.seedKeyOf(pick);
      if (excludeKeys.has(key)) continue;
      return pick;
    }
    return null;
  }

  private hydrateBecauseFromCache(): void {
    const store = this.readBecauseStore();
    const exclude = new Set<string>();
    for (let slot = 0; slot < BECAUSE_SLOTS; slot++) {
      const seed = this.pickSeedSong(slot, exclude);
      const key = seed ? this.seedKeyOf(seed) : null;
      if (!key || !seed) {
        this.becauseSlots[slot] = null;
        this.becauseSeeds[slot] = null;
        continue;
      }
      exclude.add(key);

      if (
        this.becauseSeeds[slot] === key &&
        this.becauseSlots[slot]?.songs.length
      ) {
        continue;
      }

      const hit = store.items.find(
        (c) => c.seed === key && this.cacheFresh(c),
      );
      if (hit) {
        this.becauseSlots[slot] = this.sectionFromCache(hit, slot);
        this.becauseSeeds[slot] = key;
        continue;
      }

      if (this.becauseSeeds[slot] && this.becauseSeeds[slot] !== key) {
        this.becauseSlots[slot] = null;
        this.becauseSeeds[slot] = null;
      }
    }
  }

  private async ensureBecause(force: boolean): Promise<void> {
    // Sequential so slot 1 excludes slot 0’s seed (parallel would race).
    const exclude = new Set<string>();
    for (let slot = 0; slot < BECAUSE_SLOTS; slot++) {
      await this.ensureBecauseSlot(slot, force, exclude);
    }
  }

  private async ensureBecauseSlot(
    slot: number,
    force: boolean,
    exclude: Set<string>,
  ): Promise<void> {
    const seed = this.pickSeedSong(slot, exclude);
    const key = seed ? this.seedKeyOf(seed) : null;
    if (!seed || !key) return;
    exclude.add(key);

    if (
      !force &&
      this.becauseSeeds[slot] === key &&
      this.becauseSlots[slot]?.songs.length
    ) {
      return;
    }

    if (!force) {
      const store = this.readBecauseStore();
      const hit = store.items.find(
        (c) => c.seed === key && this.cacheFresh(c),
      );
      if (hit) {
        this.becauseSlots[slot] = this.sectionFromCache(hit, slot);
        this.becauseSeeds[slot] = key;
        this.sections.set(this.merge(this.charts));
        return;
      }
    }

    const fetchKey = `${slot}:${key}`;
    if (!force && this.becauseFetchKey === fetchKey) return;

    const gen = ++this.becauseFetchGen;
    this.becauseFetchKey = fetchKey;
    try {
      const songs = await firstValueFrom(
        this.http.get<Song[]>(`${environment.API_URL}/recommend`, {
          params: force
            ? { artist: seed.artist, title: seed.title, refresh: "1" }
            : { artist: seed.artist, title: seed.title },
        }),
      );
      if (gen !== this.becauseFetchGen) return;
      // Seed may have rotated while in flight — still accept if key matches pick.
      const still = this.pickSeedSong(slot, this.excludeExcept(slot));
      if (!still || this.seedKeyOf(still) !== key) return;
      if (!songs?.length) return;

      const section: ExploreSection = {
        id: this.slotId(slot),
        title: `Because you liked ${seed.title}`,
        subtitle: seed.artist,
        songs: songs.slice(0, BECAUSE_RAIL),
        seedSong: seed,
        radio: true,
      };
      this.becauseSlots[slot] = section;
      this.becauseSeeds[slot] = key;
      this.persistBecauseSlot(slot, {
        seed: key,
        at: Date.now(),
        title: section.title,
        subtitle: section.subtitle,
        songs: section.songs,
        seedSong: seed,
        slot,
      });
      this.sections.set(this.merge(this.charts));
    } catch {
      if (gen !== this.becauseFetchGen) return;
      const store = this.readBecauseStore();
      const stale = store.items.find((c) => c.seed === key);
      if (stale?.songs.length) {
        this.becauseSlots[slot] = this.sectionFromCache(stale, slot);
        this.becauseSeeds[slot] = key;
        this.sections.set(this.merge(this.charts));
      } else if (
        this.becauseSeeds[slot] === key &&
        this.becauseSlots[slot]?.songs.length
      ) {
        this.sections.set(this.merge(this.charts));
      }
    } finally {
      if (gen === this.becauseFetchGen) this.becauseFetchKey = null;
    }
  }

  private excludeExcept(keepSlot: number): Set<string> {
    const exclude = new Set<string>();
    for (let i = 0; i < BECAUSE_SLOTS; i++) {
      if (i === keepSlot) continue;
      const k = this.becauseSeeds[i];
      if (k) exclude.add(k);
    }
    return exclude;
  }

  private cacheFresh(c: BecauseCache): boolean {
    return Date.now() - c.at <= BECAUSE_TTL_MS;
  }

  private sectionFromCache(c: BecauseCache, slot: number): ExploreSection {
    return {
      id: this.slotId(slot),
      title: c.title,
      subtitle: c.subtitle,
      songs: c.songs.slice(0, BECAUSE_RAIL),
      seedSong: c.seedSong ?? this.songFromSeedKey(c.seed),
      radio: true,
    };
  }

  private readBecauseStore(): BecauseStore {
    const raw = this.storage.getItem<BecauseStore | BecauseCache>(BECAUSE_KEY);
    if (!raw) return { at: 0, items: [] };
    // Legacy single-shelf cache → store shape.
    if ("songs" in raw && "seed" in raw && !("items" in raw)) {
      const legacy = raw as BecauseCache;
      return { at: legacy.at, items: [{ ...legacy, slot: 0 }] };
    }
    const store = raw as BecauseStore;
    return { at: store.at ?? 0, items: store.items ?? [] };
  }

  private persistBecauseSlot(slot: number, item: BecauseCache): void {
    const store = this.readBecauseStore();
    const items = store.items.filter(
      (c) => c.slot !== slot && c.seed !== item.seed,
    );
    items.push({ ...item, slot });
    // Keep at most BECAUSE_SLOTS freshest.
    items.sort((a, b) => b.at - a.at);
    this.storage.setItem(BECAUSE_KEY, {
      at: Date.now(),
      items: items.slice(0, BECAUSE_SLOTS + 2),
    } satisfies BecauseStore);
  }

  private songFromSeedKey(key: string): Song | undefined {
    for (const s of this.library.songs()) {
      if (this.seedKeyOf(s) === key) return s;
    }
    for (const s of this.storage.recentSongs()) {
      if (this.seedKeyOf(s) === key) return s;
    }
    return undefined;
  }
}

function takeUnique(songs: Song[], n: number): Song[] {
  const out: Song[] = [];
  const seen = new Set<string>();
  for (const s of songs) {
    if (!s.link || seen.has(s.link)) continue;
    seen.add(s.link);
    out.push(s);
    if (out.length >= n) break;
  }
  return out;
}

/** First shelf keeps a track; later shelves drop link/songKey dupes. */
function dedupeSections(sections: ExploreSection[]): ExploreSection[] {
  const seenLink = new Set<string>();
  const seenKey = new Set<string>();
  return sections
    .map((sec) => {
      const songs = sec.songs.filter((s) => {
        if (!s.link || seenLink.has(s.link)) return false;
        const k = songKey(s);
        if (k && seenKey.has(k)) return false;
        seenLink.add(s.link);
        if (k) seenKey.add(k);
        return true;
      });
      return songs.length ? { ...sec, songs } : null;
    })
    .filter((s): s is ExploreSection => !!s);
}
