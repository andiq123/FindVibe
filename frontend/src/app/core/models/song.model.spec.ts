import { sameSong, songKey } from "./song.model";

describe("song identity helpers", () => {
  it("sameSong keys on link (explore uuids churn)", () => {
    expect(
      sameSong(
        { link: "https://a.mp3" },
        { link: "https://a.mp3" },
      ),
    ).toBe(true);
    expect(
      sameSong(
        { link: "https://a.mp3" },
        { link: "https://b.mp3" },
      ),
    ).toBe(false);
  });

  it("songKey strips remix noise like Fiber coreTitle", () => {
    expect(
      songKey({ artist: "Adele", title: "Hello (Extended Mix)" }),
    ).toBe("adele|hello");
    expect(
      songKey({ artist: "Adele", title: "Hello feat. Someone" }),
    ).toBe("adele|hello");
  });
});
