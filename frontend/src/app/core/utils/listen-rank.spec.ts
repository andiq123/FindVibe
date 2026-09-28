import {
  rankByListen,
  rotateIndex,
  pickVaultRadioSeed,
  ListenStats,
} from "./listen-rank";
import { Song, songKey } from "../models/song.model";

const song = (n: number, order = n): Song => ({
  id: String(n),
  artist: `A${n}`,
  title: `T${n}`,
  image: "",
  link: `https://x/${n}.mp3`,
  order,
});

describe("listen-rank", () => {
  it("ranks by listen ms, injects cold every 4th slot", () => {
    const songs = [1, 2, 3, 4, 5, 6, 7, 8].map((n) => song(n));
    const stats: ListenStats = {
      "https://x/1.mp3": { ms: 100_000, plays: 5, lastAt: 8 },
      "https://x/2.mp3": { ms: 80_000, plays: 4, lastAt: 7 },
      "https://x/3.mp3": { ms: 60_000, plays: 3, lastAt: 6 },
      "https://x/4.mp3": { ms: 40_000, plays: 2, lastAt: 5 },
      "https://x/5.mp3": { ms: 20_000, plays: 1, lastAt: 4 },
      "https://x/6.mp3": { ms: 5_000, plays: 1, lastAt: 3 },
      "https://x/7.mp3": { ms: 1_000, plays: 0, lastAt: 2 },
      "https://x/8.mp3": { ms: 0, plays: 0, lastAt: 0 },
    };
    const ranked = rankByListen(songs, stats, 0);
    expect(ranked[0].link).toBe("https://x/1.mp3");
    // slot 3 (0-based) is cold — least heard among unused
    expect(ranked[3].link).toBe("https://x/8.mp3");
  });

  it("rotateIndex wraps and respects bump", () => {
    expect(rotateIndex(5, 2, 0)).toBe(2);
    expect(rotateIndex(5, 2, 1)).toBe(3);
    expect(rotateIndex(5, 4, 2)).toBe(1);
  });

  it("pickVaultRadioSeed rotates and skips last key", () => {
    const songs = [1, 2, 3, 4].map((n) => song(n));
    const stats: ListenStats = {
      "https://x/1.mp3": { ms: 100_000, plays: 5, lastAt: 8 },
      "https://x/2.mp3": { ms: 80_000, plays: 4, lastAt: 7 },
      "https://x/3.mp3": { ms: 10_000, plays: 1, lastAt: 1 },
      "https://x/4.mp3": { ms: 0, plays: 0, lastAt: 0 },
    };
    const a = pickVaultRadioSeed(songs, stats, 0);
    expect(a).toBeTruthy();
    const b = pickVaultRadioSeed(songs, stats, 1, songKey(a!));
    expect(b).toBeTruthy();
    expect(b!.link).not.toBe(a!.link);
  });
});
