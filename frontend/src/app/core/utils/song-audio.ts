import { environment } from "../../../environments/environment";
import { Song } from "../models/song.model";
import { upgradeToHttps } from "./utils";

export function streamUrl(song: Song): string {
  const params = new URLSearchParams({
    artist: song.artist,
    title: song.title,
    link: upgradeToHttps(song.link),
  });
  return `${environment.API_URL}/stream?${params}`;
}

/** Cache/download only readable responses; opaque blobs contain no audio. */
export async function fetchSongAudio(song: Song): Promise<Response> {
  for (const url of [upgradeToHttps(song.link), streamUrl(song)]) {
    try {
      const response = await fetch(url, {
        credentials: "omit",
        signal: AbortSignal.timeout(30_000),
      });
      const type = response.headers.get("Content-Type") ?? "";
      if (response.ok && (!type || /^(audio\/|application\/octet-stream)/i.test(type))) {
        return response;
      }
      await response.body?.cancel();
    } catch { /* Try the server fallback. */ }
  }
  throw new Error("Audio is unavailable from both sources");
}
