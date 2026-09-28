import { Song, songKey } from "../models/song.model";

export interface ListenStat { ms: number; plays: number; lastAt: number }
export type ListenStats = Record<string, ListenStat>;

const DAY_MS = 86_400_000;

/** Hot first; every 4th slot surfaces a cold (least-heard) track so neglected favorites stay visible. */
export function rankByListen(
  songs: Song[],
  stats: ListenStats,
  /** Stable salt (e.g. day bucket) — same day → same interleave, no flicker. */
  salt = 0,
): Song[] {
  if (songs.length <= 1) return [...songs];

  const score = (s: Song) => {
    const st = stats[s.link];
    return (st?.ms ?? 0) + (st?.plays ?? 0) * 30_000;
  };
  const lastAt = (s: Song) => stats[s.link]?.lastAt ?? 0;

  const byHot = [...songs].sort((a, b) => {
    const d = score(b) - score(a);
    if (d) return d;
    const t = lastAt(b) - lastAt(a);
    if (t) return t;
    return (a.order ?? 0) - (b.order ?? 0);
  });
  // Cold: least listen time, prefer never/long-ago — rotate start by salt.
  const byCold = [...songs].sort((a, b) => {
    const d = score(a) - score(b);
    if (d) return d;
    return lastAt(a) - lastAt(b);
  });
  const coldStart = ((salt % byCold.length) + byCold.length) % byCold.length;
  const coldRotated = [
    ...byCold.slice(coldStart),
    ...byCold.slice(0, coldStart),
  ];

  const out: Song[] = [];
  const used = new Set<string>();
  let hi = 0;
  let ci = 0;
  for (let i = 0; i < songs.length; i++) {
    const wantCold = songs.length >= 5 && i > 0 && i % 4 === 3;
    if (wantCold) {
      while (ci < coldRotated.length && used.has(coldRotated[ci].link)) ci++;
      if (ci < coldRotated.length) {
        const pick = coldRotated[ci++];
        out.push(pick);
        used.add(pick.link);
        continue;
      }
    }
    while (hi < byHot.length && used.has(byHot[hi].link)) hi++;
    if (hi < byHot.length) {
      const pick = byHot[hi++];
      out.push(pick);
      used.add(pick.link);
    }
  }
  return out;
}

/** Day-stable index into a pool; `bump` rotates on manual refresh. */
export function rotateIndex(length: number, day: number, bump = 0): number {
  if (length <= 0) return 0;
  return (((day + bump) % length) + length) % length;
}

/**
 * Vault → radio seed: rotate through the taste-core (hot half of rankByListen)
 * so each press discovers from a different liked anchor, not always songs[0].
 */
export function pickVaultRadioSeed(
  vault: Song[],
  stats: ListenStats,
  bump = 0,
  lastKey = "",
): Song | null {
  if (!vault.length) return null;
  const day = Math.floor(Date.now() / DAY_MS);
  const ranked = rankByListen(vault, stats, day + bump);
  const coreLen = Math.max(
    1,
    Math.min(ranked.length, Math.max(3, Math.ceil(ranked.length / 2))),
  );
  const core = ranked.slice(0, coreLen);
  let idx = rotateIndex(core.length, day, bump);
  let pick = core[idx];
  if (lastKey && core.length > 1 && songKey(pick) === lastKey) {
    idx = rotateIndex(core.length, day, bump + 1);
    pick = core[idx];
  }
  return pick ?? null;
}

/** Recent listening drives discovery; old heavy rotation decays over a week. */
export function recommendationSeeds(recents: Song[], favorites: Song[], stats: ListenStats, now = Date.now()): Song[] {
  const recentIndex = new Map<string, number>();
  const liked = new Set(favorites.map(songKey));
  const unique = new Map<string, Song>();
  const combined = new Map<string, ListenStat>();
  const seenLinks = new Set<string>();
  for (const [i, s] of recents.entries()) {
    const key = songKey(s);
    if (key && !recentIndex.has(key)) recentIndex.set(key, i);
  }
  for (const s of [...recents, ...favorites]) {
    const key = songKey(s);
    if (!key) continue;
    if (!unique.has(key)) unique.set(key, s);
    const stat = stats[s.link];
    if (stat && !seenLinks.has(s.link)) {
      const prev = combined.get(key);
      combined.set(key, { ms: (prev?.ms ?? 0) + stat.ms, plays: (prev?.plays ?? 0) + stat.plays, lastAt: Math.max(prev?.lastAt ?? 0, stat.lastAt) });
    }
    seenLinks.add(s.link);
  }
  const score = (s: Song) => {
    const key = songKey(s);
    const index = recentIndex.get(key);
    const stat = combined.get(key);
    const age = stat ? Math.max(0, now - stat.lastAt) / DAY_MS : 0;
    const decay = Math.pow(0.5, age / 7);
    // A click is a weak signal; 30 seconds heard gives full recent-listen weight.
    const confidence = stat ? Math.min(1, stat.ms / 30_000) : 0.5;
    return (liked.has(key) ? 1.5 : 0) + decay * (
      (index === undefined ? 0 : 6 * confidence / (1 + index / 3)) +
      Math.log1p((stat?.ms ?? 0) / 180_000)
    );
  };
  return [...unique.values()].sort((a, b) => score(b) - score(a));
}
