import { TestBed } from "@angular/core/testing";
import { AudioService } from "./audio.service";
import { StorageService } from "./storage.service";
import { PlaylistService } from "./playlist.service";
import { PlayerStatus } from "../../features/player/models/player.model";

describe("AudioService source lifecycle", () => {
  let service: AudioService;
  let elements: HTMLAudioElement[];
  beforeEach(() => {
    elements = [];
    spyOn(window, "Audio").and.callFake(function () {
      const audio = document.createElement("audio");
      spyOn(audio, "load");
      spyOn(audio, "pause");
      spyOn(audio, "play").and.resolveTo();
      elements.push(audio);
      return audio;
    });
    TestBed.configureTestingModule({ providers: [
      AudioService,
      { provide: StorageService, useValue: {} },
      { provide: PlaylistService, useValue: { currentSong: () => null } },
    ] });
    service = TestBed.inject(AudioService);
    service.initialize();
  });
  afterEach(() => service.ngOnDestroy());
  it("ignores a failed play promise belonging to a replaced source", async () => {
    let reject!: (reason: Error) => void;
    (elements[0].play as jasmine.Spy).and.returnValue(new Promise<void>((_, fail) => { reject = fail; }));
    const playing = service.playSource("https://mp3.pm/old.mp3");
    service.setSource("https://mp3.pm/new.mp3");
    reject(new Error("old source failed"));
    await playing;
    expect(service.status()).toBe(PlayerStatus.Loading);
  });
  it("clears stale timing and removes the source without fetching the page", () => {
    service.currentTime.set(90); service.duration.set(180);
    service.setSource("");
    elements[0].dispatchEvent(new Event("loadstart"));
    expect(elements[0].hasAttribute("src")).toBeFalse();
    expect(service.currentTime()).toBe(0);
    expect(service.duration()).toBe(0);
    expect(service.status()).toBe(PlayerStatus.Stopped);
  });
  it("preserves paused session restoration during loadstart", () => {
    service.setSource("https://mp3.pm/song.mp3");
    service.markPaused();
    elements[0].dispatchEvent(new Event("loadstart"));
    expect(service.status()).toBe(PlayerStatus.Paused);
  });
});
