import { directAudioEnabled, fetchSongAudio, streamUrl } from "./song-audio";
import { Song } from "../models/song.model";

describe("song audio fallback", () => {
  const song: Song = { id: "1", artist: "A & B", title: "A + song", image: "", link: "https://new.kachevo.org/get/music/song.mp3" };
  it("routes paused providers through active-source recovery", async () => {
    const old = { ...song, link: "https://retired.example/song.mp3" };
    expect(directAudioEnabled(old)).toBeFalse();
    const audio = new Response("audio", { headers: { "Content-Type": "audio/mpeg" } });
    const fetchSpy = spyOn(globalThis, "fetch").and.resolveTo(audio);
    expect(await fetchSongAudio(old)).toBe(audio);
    expect(fetchSpy).toHaveBeenCalledOnceWith(streamUrl(old), jasmine.any(Object));
  });
  it("encodes exact track identity and link", () => {
    const params = new URL(streamUrl(song)).searchParams;
    expect(params.get("artist")).toBe(song.artist);
    expect(params.get("title")).toBe(song.title);
    expect(params.get("link")).toBe(song.link);
  });
  it("falls back when direct audio is blocked by CORS", async () => {
    const audio = new Response("audio", { headers: { "Content-Type": "audio/mpeg" } });
    const fetchSpy = spyOn(globalThis, "fetch").and.returnValues(Promise.reject(new TypeError("CORS")), Promise.resolve(audio));
    expect(await fetchSongAudio(song)).toBe(audio);
    expect(fetchSpy.calls.argsFor(1)[0]).toBe(streamUrl(song));
  });
  it("rejects HTML error pages and uses readable audio", async () => {
    const audio = new Response("audio", { headers: { "Content-Type": "audio/mpeg" } });
    spyOn(globalThis, "fetch").and.returnValues(Promise.resolve(new Response("blocked", { headers: { "Content-Type": "text/html" } })), Promise.resolve(audio));
    expect(await fetchSongAudio(song)).toBe(audio);
  });
  it("reports failure when both sources fail", async () => {
    spyOn(globalThis, "fetch").and.callFake(async () => new Response("unavailable", { status: 502 }));
    await expectAsync(fetchSongAudio(song)).toBeRejectedWithError("Audio is unavailable from both sources");
  });
});
