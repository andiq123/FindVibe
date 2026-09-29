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

/** Allow direct audio only from the configured music sources. */
export function directAudioEnabled(song: Song): boolean {
  try {
    const url = new URL(upgradeToHttps(song.link));
    return url.protocol === "https:" && !url.username && !url.password &&
      (!url.port || url.port === "443") &&
      ((["new.kachevo.org", "eu.hitmoz.com"].includes(url.hostname) && url.pathname.startsWith("/get/music/")) ||
      ["mp3.pm", "mp3mn.net", "musify.club", "sunproxy.net"].some(host => url.hostname === host || url.hostname.endsWith("." + host)));
  } catch { return false; }
}

/** Cache/download only readable responses; opaque blobs contain no audio. */
export async function fetchSongAudio(song: Song): Promise<Response> {
  for (const url of directAudioEnabled(song) ? [upgradeToHttps(song.link), streamUrl(song)] : [streamUrl(song)]) {
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
