import { Song } from "../../../core/models/song.model";

export const artworkKey = (song: Song): string =>
  `${song.artist.replace(/\s*\(nightcore[^)]*\)/gi, "").trim()} ${song.title.trim()}`.replace(/\s+/g, " ").toLowerCase();

// MusicBoss also serves its generic music-note image under this cover ID.
export const missingArtwork = (song: Song): boolean =>
  !song.image?.trim() || /no[-_]cover|no_album_art|\/covers\/a\/5f3\/c54\/406641\.jpg(?:$|\?)/i.test(song.image);

/** Three bounded workers; duplicate source results share one cover lookup. */
export async function fillSearchArtwork(
  songs: Song[], api: string, signal: AbortSignal,
  update: (key: string, image: string) => void,
): Promise<void> {
  const queries = [...new Set(songs.filter(missingArtwork).map(artworkKey))];
  let cursor = 0;
  const worker = async () => {
    while (!signal.aborted && cursor < queries.length) {
      const key = queries[cursor++];
      try {
        const response = await fetch(`${api}/cover?${new URLSearchParams({ q: key })}`, {
          signal: AbortSignal.any([signal, AbortSignal.timeout(6000)]),
        });
        if (!response.ok) continue;
        const { image } = await response.json() as { image?: string };
        if (!signal.aborted && image?.startsWith("https://")) update(key, image);
      } catch { /* Artwork is optional; playback and search stay available. */ }
    }
  };
  await Promise.all(Array.from({ length: Math.min(3, queries.length) }, worker));
}
