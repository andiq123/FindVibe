import {
  mediaArtworkSrc,
  mediaPlaybackState,
  tabTitle,
} from "./media-session.service";
import { playbackBusy } from "./app-update.service";
import { PlayerStatus } from "../../features/player/models/player.model";

describe("mediaArtworkSrc", () => {
  it("upgrades http and rejects placeholders", () => {
    expect(mediaArtworkSrc("http://cdn.example/a.jpg")).toBe(
      "https://cdn.example/a.jpg",
    );
    expect(mediaArtworkSrc("https://cdn.example/a.jpg")).toBe(
      "https://cdn.example/a.jpg",
    );
    expect(mediaArtworkSrc("")).toBe("");
    expect(mediaArtworkSrc("no_album_art.jpg")).toBe("");
  });
});

describe("mediaPlaybackState", () => {
  it("keeps Now Playing alive across track gaps", () => {
    expect(mediaPlaybackState(PlayerStatus.Playing)).toBe("playing");
    expect(mediaPlaybackState(PlayerStatus.Loading)).toBe("playing");
    expect(mediaPlaybackState(PlayerStatus.Ended)).toBe("playing");
    expect(mediaPlaybackState(PlayerStatus.Paused)).toBe("paused");
    expect(mediaPlaybackState(PlayerStatus.Stopped)).toBe("none");
    expect(mediaPlaybackState(PlayerStatus.Error)).toBe("none");
    expect(mediaPlaybackState(PlayerStatus.Error, true)).toBe("paused");
  });
});

describe("playbackBusy", () => {
  it("keeps SW update from reloading across track gaps", () => {
    expect(playbackBusy(PlayerStatus.Playing)).toBe(true);
    expect(playbackBusy(PlayerStatus.Loading)).toBe(true);
    expect(playbackBusy(PlayerStatus.Ended)).toBe(true);
    expect(playbackBusy(PlayerStatus.Paused)).toBe(false);
    expect(playbackBusy(PlayerStatus.Stopped)).toBe(false);
  });
});

describe("tabTitle", () => {
  const song = { title: "Hello", artist: "Adele" };

  it("shows now-playing and resets when stopped", () => {
    expect(tabTitle(song, PlayerStatus.Playing)).toBe(
      "Hello · Adele · FindVibe",
    );
    expect(tabTitle(song, PlayerStatus.Paused)).toBe(
      "❚❚ Hello · Adele · FindVibe",
    );
    expect(tabTitle(song, PlayerStatus.Stopped)).toBe("FindVibe");
    expect(tabTitle(null, PlayerStatus.Playing)).toBe("FindVibe");
  });

  it("strips control chars", () => {
    expect(
      tabTitle(
        { title: "Hi\nThere", artist: "A\u0000B" },
        PlayerStatus.Playing,
      ),
    ).toBe("HiThere · AB · FindVibe");
  });
});
