import { signal } from "@angular/core";
import { TestBed } from "@angular/core/testing";
import { provideHttpClient } from "@angular/common/http";
import { provideHttpClientTesting, HttpTestingController } from "@angular/common/http/testing";
import { RadioService } from "./radio.service";
import { PlaylistService } from "./playlist.service";
import { ToastService } from "./toast.service";
import { Song } from "../models/song.model";

const song = (id: string): Song => ({ id, title: id, artist: "Artist", image: "", link: `https://music/${id}` });
describe("radio station lifecycle", () => {
  let radio: RadioService;
  let http: HttpTestingController;
  const active = signal(false);
  let append: jasmine.Spy;
  beforeEach(() => {
    active.set(false);
    append = jasmine.createSpy();
    TestBed.configureTestingModule({ providers: [provideHttpClient(), provideHttpClientTesting(), RadioService,
      { provide: PlaylistService, useValue: {
        radioActive: active, remaining: () => 10, currentSong: () => song("seed"),
        queueLinks: () => [song("seed").link], queueSongs: () => [song("seed")],
        setRadioPlaylist: () => active.set(true), setCurrentSong: () => undefined, appendSongs: append,
      } },
      { provide: ToastService, useValue: { hold: () => undefined, show: () => undefined, clear: () => undefined } },
    ] });
    radio = TestBed.inject(RadioService); http = TestBed.inject(HttpTestingController); TestBed.tick();
  });
  afterEach(() => http.verify());
  async function start() {
    const result = radio.start(song("seed"));
    http.expectOne(r => r.url.endsWith("/recommend")).flush([song("neighbor")]);
    expect(await result).toBeTrue(); TestBed.tick();
  }
  it("advances a full candidate window instead of one track", async () => {
    await start();
    const pending = radio["extend"]();
    const request = http.expectOne(r => r.url.endsWith("/recommend"));
    expect(request.request.params.get("offset")).toBe("12");
    request.flush([song("fresh")]); await pending;
    expect(append).toHaveBeenCalledOnceWith([song("fresh")]);
  });
  it("discards in-flight results after radio is stopped", async () => {
    await start();
    const pending = radio["extend"]();
    const request = http.expectOne(r => r.url.endsWith("/recommend"));
    active.set(false); TestBed.tick();
    request.flush([song("late")]); await pending;
    expect(append).not.toHaveBeenCalled();
  });
});
