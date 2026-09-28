export interface Song {
  id: string;
  artist: string;
  title: string;
  image: string;
  link: string;
  /** Vault reorder only — omitted on search/explore/recommend. */
  order?: number;
  /** Cached lyrics for vault tracks (filled after first explicit open). */
  lyrics?: string;
  /** Search source name from API (e.g. MuzJam, Mp3mn). */
  provider?: string;
}

const PROVIDER_HOST: Record<string, string> = {
  MuzJam: "muzjam.org",
  Mp3mn: "mp3mn.net",
  Musify: "musify.club",
};

/** Short host label for source badges (supports merged e.g. MuzJam+Mp3mn). */
export function sourceHost(provider?: string): string {
  if (!provider) return "";
  return provider
    .split("+")
    .map((p) => PROVIDER_HOST[p] ?? p)
    .join(" · ");
}

/**
 * Playback identity. Explore/recommend mint new uuids per resolve —
 * same track across shelves shares `link`, not `id`.
 */
export function sameSong(
  a: Pick<Song, "link"> | null | undefined,
  b: Pick<Song, "link"> | null | undefined,
): boolean {
  return !!a?.link && !!b?.link && a.link === b.link;
}

/**
 * Soft dedupe key (artist|coreTitle). Matches Fiber `songKey` / `coreTitle`
 * so radio/explore don't re-queue remix variants of the same cut.
 */
export function songKey(s: Pick<Song, "artist" | "title">): string {
  const artist = (s.artist || "").toLowerCase().trim();
  let title = (s.title || "").toLowerCase().trim();
  title = title
    .replace(/\([^)]*\)|\[[^\]]*\]/g, " ")
    .replace(/\s*(feat\.?|ft\.?|featuring)\s+.*/g, " ")
    .replace(
      /\b(original\s+mix|extended\s+mix|radio\s+edit|club\s+mix|remix|bootleg|edit|mix|version|remaster(ed)?|instrumental|karaoke|live|acoustic|dub)\b/g,
      " ",
    )
    .replace(/\s+/g, " ")
    .trim();
  return artist && title ? `${artist}|${title}` : "";
}

export interface PaginationInfo {
  currentPage: number;
  totalResults: number;
  hasNextPage: boolean;
  hasPrevPage: boolean;
  totalPages: number;
}
export interface SearchResponse {
  songs: Song[];
  pagination?: PaginationInfo | null;
}
export enum SearchStatus {
  None,
  Loading,
  Error,
  Finished,
}
