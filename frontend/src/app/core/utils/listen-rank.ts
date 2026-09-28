import { Song, songKey } from "../models/song.model";

export type ListenStat = { ms: number; plays: number; lastAt: number };
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
