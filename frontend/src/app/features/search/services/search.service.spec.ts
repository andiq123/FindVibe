import { TestBed } from "@angular/core/testing";
import { HttpClient } from "@angular/common/http";
import { firstValueFrom } from "rxjs";
import { SearchService } from "./search.service";
import { StorageService } from "../../../core/services/storage.service";
import { SettingsService } from "../../../core/services/settings.service";
import { SearchStatus } from "../../../core/models/song.model";

describe("progressive search", () => {
  let service: SearchService;
  let streams: ReadableStreamDefaultController<Uint8Array>[];
  let fetchSpy: jasmine.Spy;
  const song = { id: "1", title: "Song", artist: "Artist", link: "https://mp3.pm/1", image: "" };
  const send = (index: number, event: object) => streams[index].enqueue(new TextEncoder().encode(JSON.stringify(event) + "\n"));
  const settle = () => new Promise<void>((resolve) => setTimeout(resolve, 0));
  beforeEach(() => {
    streams = [];
    fetchSpy = spyOn(globalThis, "fetch").and.callFake(async () => new Response(new ReadableStream({ start(controller) { streams.push(controller); } })));
    TestBed.configureTestingModule({ providers: [SearchService,
      { provide: HttpClient, useValue: {} },
      { provide: StorageService, useValue: { getItem: () => null, setItem: jasmine.createSpy(), removeItem: jasmine.createSpy() } },
      { provide: SettingsService, useValue: { setServerUp: jasmine.createSpy() } },
    ] });
    service = TestBed.inject(SearchService);
  });
  it("publishes playable songs before completion and removes duplicates", async () => {
    const result = firstValueFrom(service.searchSongs("query"));
    send(0, { type: "song", song }); send(0, { type: "song", song });
    await settle();
    expect(service.songs()).toEqual([song]);
    expect(service.status()).toBe(SearchStatus.Loading);
    send(0, { type: "done" });
    expect((await result).songs).toEqual([song]);
    expect(service.status()).toBe(SearchStatus.Finished);
  });
  it("aborts superseded searches and ignores their late results", async () => {
    const previous = service.searchSongs("old").subscribe();
    const latest = firstValueFrom(service.searchSongs("new"));
    expect(fetchSpy.calls.argsFor(0)[1].signal.aborted).toBeTrue();
    send(0, { type: "song", song });
    send(1, { type: "song", song: { ...song, title: "New" } });
    send(1, { type: "done" });
    await latest;
    expect(service.songs()[0].title).toBe("New");
    previous.unsubscribe();
  });
  it("keeps partial results when a stream ends unexpectedly", async () => {
    const result = firstValueFrom(service.searchSongs("query"));
    send(0, { type: "song", song }); streams[0].close();
    await result;
    expect(service.songs()).toEqual([song]);
    expect(service.searchWarning()).toContain("retry");
  });
  it("allows a canceled route search to restart instead of retaining a spinner", () => {
    const sub = service.searchSongs("query").subscribe();
    sub.unsubscribe();
    expect(fetchSpy.calls.argsFor(0)[1].signal.aborted).toBeTrue();
    expect(service.status()).toBe(SearchStatus.None);
    streams[0].close();
  });
  it("treats no matches as an empty result, not a connection error", async () => {
    const result = firstValueFrom(service.searchSongs("query"));
    send(0, { type: "error", error: "no songs found" });
    await result;
    expect(service.status()).toBe(SearchStatus.Finished);
    expect(service.songs()).toEqual([]);
  });
});
