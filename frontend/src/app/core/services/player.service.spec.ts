import { signal } from "@angular/core";
import { TestBed } from "@angular/core/testing";
import { PlayerService } from "./player.service";
import { AudioService } from "./audio.service";
import { PlaylistService } from "./playlist.service";
import { SettingsService } from "./settings.service";
import { StorageService } from "./storage.service";
import { RadioService } from "./radio.service";
import { ToastService } from "./toast.service";
import { OfflineStorageService } from "../../features/library/services/offline-storage.service";
import { PlayerStatus } from "../../features/player/models/player.model";
import { Song } from "../models/song.model";
import { streamUrl } from "../utils/song-audio";

describe("PlayerService recovery", () => {
  const song: Song = { id: "1", artist: "Artist", title: "Song", image: "", link: "https://new.kachevo.org/get/music/song.mp3" };
  let player: PlayerService;
  let status: ReturnType<typeof signal<PlayerStatus>>;
  let playSource: jasmine.Spy;
  let offline: ReturnType<typeof signal<boolean>>;
  beforeEach(() => {
    status = signal(PlayerStatus.Stopped);
    offline = signal(false);
    const current = signal<Song | null>(song);
    playSource = jasmine.createSpy("playSource").and.callFake(async () => status.set(PlayerStatus.Loading));
    TestBed.configureTestingModule({ providers: [
      PlayerService,
      { provide: AudioService, useValue: {
        status, currentTime: signal(0), duration: signal(0), playSource,
        clearPreload: jasmine.createSpy(), setSource: jasmine.createSpy(),
        markPaused: () => status.set(PlayerStatus.Paused),
        pause: () => status.set(PlayerStatus.Paused),
        fail: () => status.set(PlayerStatus.Error),
      } },
      { provide: PlaylistService, useValue: {
        currentSong: current, setCurrentSong: (value: Song) => current.set(value),
        upcoming: signal([]), persist: jasmine.createSpy(),
        restoreSession: () => true, takePendingSeek: () => 0,
      } },
      { provide: SettingsService, useValue: { isNavigatorOffline: offline } },
      { provide: OfflineStorageService, useValue: { isAvailableOffline: async () => undefined } },
      { provide: StorageService, useValue: {} },
      { provide: RadioService, useValue: {} },
      { provide: ToastService, useValue: {} },
    ] });
    player = TestBed.inject(PlayerService);
    TestBed.tick();
  });
  it("tries the stream once, then reports a failed track without looping", async () => {
    await player.setSong(song);
    TestBed.tick();
    status.set(PlayerStatus.Error); TestBed.tick();
    expect(playSource.calls.mostRecent().args[0]).toBe(streamUrl(song));
    status.set(PlayerStatus.Error); TestBed.tick();
    expect(playSource.calls.count()).toBe(2);
    expect(player.playError()).toContain("Couldn't play");
  });
  it("resolves paused-provider songs without contacting their old host", async () => {
    const old = { ...song, link: "https://mp3.pm/song.mp3" };
    await player.setSong(old); TestBed.tick();
    expect(playSource).toHaveBeenCalledOnceWith(streamUrl(old));
    status.set(PlayerStatus.Error); TestBed.tick();
    expect(playSource.calls.count()).toBe(1);
    expect(player.playError()).toContain("Couldn't play");
  });
  it("does not autoplay when a restored paused source fails", async () => {
    await player.restoreSession(); TestBed.tick();
    status.set(PlayerStatus.Error); TestBed.tick();
    expect(playSource).not.toHaveBeenCalled();
  });
  it("does not request a network fallback for missing offline audio", async () => {
    offline.set(true);
    await player.setSong(song); TestBed.tick();
    expect(playSource).not.toHaveBeenCalled();
    expect(player.playError()).toBe("Not available offline");
  });
});
