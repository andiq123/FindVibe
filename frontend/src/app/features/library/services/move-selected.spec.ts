import { Song } from "../../../core/models/song.model";
import { moveSelectedSongs } from "./library.service";

function songs(...ids: string[]): Song[] {
  return ids.map((id, i) => ({
    id,
    artist: "a",
    title: id,
    image: "",
    link: `https://x/${id}`,
    order: i + 1,
  }));
}

describe("moveSelectedSongs", () => {
  it("moves a block to top and keeps relative order", () => {
    const got = moveSelectedSongs(songs("a", "b", "c", "d"), ["c", "b"], "top");
    expect(got?.map((s) => s.id)).toEqual(["b", "c", "a", "d"]);
  });

  it("moves a block to bottom", () => {
    const got = moveSelectedSongs(songs("a", "b", "c", "d"), ["a", "b"], "bottom");
    expect(got?.map((s) => s.id)).toEqual(["c", "d", "a", "b"]);
  });

  it("moves up / down by one and no-ops at edges", () => {
    expect(
      moveSelectedSongs(songs("a", "b", "c"), ["b", "c"], "up")?.map((s) => s.id),
    ).toEqual(["b", "c", "a"]);
    expect(
      moveSelectedSongs(songs("a", "b", "c"), ["a", "b"], "down")?.map(
        (s) => s.id,
      ),
    ).toEqual(["c", "a", "b"]);
    expect(moveSelectedSongs(songs("a", "b"), ["a"], "up")).toBeNull();
    expect(moveSelectedSongs(songs("a", "b"), ["b"], "down")).toBeNull();
  });

  it("rewrites order 1..n", () => {
    const got = moveSelectedSongs(songs("a", "b", "c"), ["c"], "top");
    expect(got?.map((s) => s.order)).toEqual([1, 2, 3]);
  });
});
