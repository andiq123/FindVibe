import { Injectable } from "@angular/core";

type LyricsMiss = { message: string; hard: boolean };

/** Session lyrics cache — avoids rehitting /lyrics for the same track. */
@Injectable({ providedIn: "root" })
export class LyricsCacheService {
  private readonly hits = new Map<string, string>();
  private readonly misses = new Map<string, LyricsMiss>();

  key(artist: string, title: string): string {
    return `${artist.trim()}\0${title.trim()}`;
  }

  getHit(key: string): string | undefined {
    return this.hits.get(key);
  }

  getMiss(key: string): LyricsMiss | undefined {
    return this.misses.get(key);
  }

  setHit(key: string, text: string): void {
    this.misses.delete(key);
    this.hits.set(key, text);
    this.trim(this.hits);
  }

  setMiss(key: string, message: string, hard: boolean): void {
    this.hits.delete(key);
    this.misses.set(key, { message, hard });
    this.trim(this.misses);
  }

  // ponytail: cap at 80 — session only; vault already persists favorites
  private trim(map: Map<string, unknown>): void {
    if (map.size <= 80) return;
    const drop = map.size - 60;
    let i = 0;
    for (const k of map.keys()) {
      map.delete(k);
      if (++i >= drop) break;
    }
  }
}
