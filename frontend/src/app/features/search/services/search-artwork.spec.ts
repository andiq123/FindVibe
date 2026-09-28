import { fillSearchArtwork } from "./search-artwork";
import { Song } from "../../../core/models/song.model";

const track = (title: string, image = ""): Song => ({ id: title, artist: "Starset", title, image, link: title });

describe("search artwork", () => {
  it("shares lookups across sources and preserves existing artwork", async () => {
    const fetcher = spyOn(globalThis, "fetch").and.callFake(async () =>
      new Response(JSON.stringify({ image: "https://art.test/cover.jpg" })),
    );
    const update = jasmine.createSpy();
    await fillSearchArtwork([track("Back To The Earth"), { ...track("Back To The Earth"), artist: "Starset (Nightcore by EIW)" }, track("Back To The Earth", "https://new.kachevo.org/covers/a/5f3/c54/406641.jpg"), track("Other", "https://art.test/other.jpg")], "https://api.test", new AbortController().signal, update);
    expect(fetcher.calls.count()).toBe(1);
    expect(update).toHaveBeenCalledOnceWith("starset back to the earth", "https://art.test/cover.jpg");
  });
  it("caps concurrency and ignores cancelled responses", async () => {
    const pending: (() => void)[] = [];
    const fetcher = spyOn(globalThis, "fetch").and.callFake(() => new Promise<Response>((resolve) => {
      pending.push(() => resolve(new Response(JSON.stringify({ image: "https://art.test/a.jpg" }))));
    }));
    const controller = new AbortController();
    const update = jasmine.createSpy();
    const work = fillSearchArtwork(Array.from({ length: 8 }, (_, i) => track(String(i))), "https://api.test", controller.signal, update);
    expect(fetcher.calls.count()).toBe(3);
    controller.abort();
    pending.forEach((resolve) => resolve());
    await work;
    expect(update).not.toHaveBeenCalled();
    expect(fetcher.calls.count()).toBe(3);
  });
  it("keeps artwork failures optional", async () => {
    spyOn(globalThis, "fetch").and.rejectWith(new TypeError("offline"));
    const update = jasmine.createSpy();
    await fillSearchArtwork([track("Song")], "https://api.test", new AbortController().signal, update);
    expect(update).not.toHaveBeenCalled();
  });
});
