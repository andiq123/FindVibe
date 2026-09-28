import { Injectable, signal, computed, inject } from "@angular/core";
import { bytesToGB, upgradeToHttps } from "../../../core/utils/utils";
import { Song } from "../../../core/models/song.model";
import { StorageService } from "../../../core/services/storage.service";
import { trackLoadingState } from "../../../core/utils/loading-state.util";

const CACHE_NAME = "library-vault";
const DOWNLOAD_CONCURRENCY = 3;

export interface CacheSongsResult {
  saved: number;
  skipped: number;
  failed: number;
}

@Injectable({
  providedIn: "root",
})
export class OfflineStorageService {
  readonly storageTotal = signal(0);
  readonly storageUsed = signal(0);
  private readonly _currentLoadingDownloadSongIds = signal<string[]>([]);
  private readonly _availableOfflineSongIds = signal<string[]>([]);
  private readonly storageService = inject(StorageService);
  readonly currentLoadingDownloadSongIds =
    this._currentLoadingDownloadSongIds.asReadonly();
  readonly availableOfflineSongIds = this._availableOfflineSongIds.asReadonly();
  readonly offlineCount = computed(() => this._availableOfflineSongIds().length);
  readonly isSyncing = computed(
    () => this._currentLoadingDownloadSongIds().length > 0,
  );
  private cachePromise?: Promise<Cache>;

  initialize(): void {
    void this.setUpStorage();
    void this.syncOfflineSongs();
  }

  async syncOfflineSongs(songs?: Song[]): Promise<void> {
    const list =
      songs ?? this.storageService.getItem<Song[]>("library") ?? [];
    if (!list.length) {
      this._availableOfflineSongIds.set([]);
      return;
    }
    const cache = await this.getCache();
    const availableIds: string[] = [];
    await Promise.all(
      list.map(async (song) => {
        const match = await cache.match(upgradeToHttps(song.link));
        if (match) availableIds.push(song.id);
      }),
    );
    this._availableOfflineSongIds.set(availableIds);
  }

  async setUpStorage(): Promise<void> {
    try {
      if (!navigator.storage?.estimate) return;
      const { quota, usage } = await navigator.storage.estimate();
      this.storageTotal.set(bytesToGB(quota ?? 0));
      this.storageUsed.set(bytesToGB(usage ?? 0));
    } catch (error) {
      console.error("[OfflineStorage] Failed to estimate storage:", error);
    }
  }

  async cacheSong(song: Song): Promise<boolean> {
    const { saved, skipped } = await this.cacheAllSongs([song]);
    return saved > 0 || skipped > 0;
  }

  /** Store an already-fetched response (e.g. MP3 export) into the vault. */
  async rememberResponse(song: Song, response: Response): Promise<void> {
    const url = upgradeToHttps(song.link);
    if (!url) return;
    await (await this.getCache()).put(url, response);
    this.addAvailableOfflineSongId(song.id);
    void this.setUpStorage();
  }

  async cacheAllSongs(songs: Song[]): Promise<CacheSongsResult> {
    const result: CacheSongsResult = { saved: 0, skipped: 0, failed: 0 };
    if (!songs.length) return result;

    const cache = await this.getCache();
    let i = 0;

    const worker = async () => {
      while (i < songs.length) {
        const song = songs[i++];
        const url = upgradeToHttps(song.link);
        if (!url) {
          result.failed++;
          continue;
        }
        if (await cache.match(url)) {
          this.addAvailableOfflineSongId(song.id);
          result.skipped++;
          continue;
        }

        this.trackProgress(song.id, true);
        try {
          const ok = await putInCache(cache, url);
          if (ok) {
            this.addAvailableOfflineSongId(song.id);
            result.saved++;
          } else {
            result.failed++;
          }
        } catch (error) {
          console.error("[OfflineStorage] Failed to cache song:", error);
          result.failed++;
        } finally {
          this.trackProgress(song.id, false);
        }
      }
    };

    const n = Math.min(DOWNLOAD_CONCURRENCY, songs.length);
    await Promise.all(Array.from({ length: n }, () => worker()));
    await this.setUpStorage();
    return result;
  }

  async isAvailableOffline(
    songLink: string,
    existingCache?: Cache,
  ): Promise<Response | undefined> {
    const cache = existingCache ?? (await this.getCache());
    return cache.match(upgradeToHttps(songLink));
  }

  async removeCache(): Promise<void> {
    await caches.delete(CACHE_NAME);
    this.cachePromise = undefined;
    this.emptyAvailableOfflineSongIds();
    await this.setUpStorage();
  }

  addAvailableOfflineSongId(songId: string): void {
    this._availableOfflineSongIds.update((ids) =>
      ids.includes(songId) ? ids : [...ids, songId],
    );
  }

  emptyAvailableOfflineSongIds(): void {
    this._availableOfflineSongIds.set([]);
  }

  async removeSongFromCache(songId: string, songLink: string): Promise<void> {
    try {
      const cache = await this.getCache();
      await cache.delete(upgradeToHttps(songLink));
      this._availableOfflineSongIds.update((ids) =>
        ids.filter((id) => id !== songId),
      );
      await this.setUpStorage();
    } catch (error) {
      console.error("[OfflineStorage] Failed to remove song:", error);
    }
  }

  private getCache(): Promise<Cache> {
    if (!this.cachePromise) {
      this.cachePromise = caches.open(CACHE_NAME);
    }
    return this.cachePromise;
  }

  private trackProgress(id: string, isLoading: boolean): void {
    trackLoadingState(this._currentLoadingDownloadSongIds, id, isLoading);
  }
}

/** CORS first; no-cors opaque fallback so CDNs without ACAO still vault. */
async function putInCache(cache: Cache, url: string): Promise<boolean> {
  try {
    const res = await fetch(url, { mode: "cors", credentials: "omit" });
    if (res.ok) {
      await cache.put(url, res);
      return true;
    }
  } catch {
    // fall through
  }
  try {
    const res = await fetch(url, { mode: "no-cors", credentials: "omit" });
    // opaque responses are status 0 but cacheable + playable via blob URL
    if (res.type === "opaque" || res.ok) {
      await cache.put(url, res);
      return true;
    }
  } catch {
    return false;
  }
  return false;
}
