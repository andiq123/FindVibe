import {
  Component,
  computed,
  signal,
  OnInit,
  inject,
  ChangeDetectionStrategy,
} from "@angular/core";
import { RouterLink } from "@angular/router";
import { FaIconComponent } from "@fortawesome/angular-fontawesome";
import { PageContentComponent } from "../../shared/components/page-content/page-content.component";
import { EmptyStateComponent } from "../../shared/empty-state/empty-state.component";
import { SearchBarComponent } from "../search/search-bar/search-bar.component";
import { PlaylistService } from "../../core/services/playlist.service";
import { PlayerStatus } from "../player/models/player.model";
import { PlayerButtonComponent } from "../../shared/player-button/player-button.component";
import { PlayerService } from "../../core/services/player.service";
import { RadioService } from "../../core/services/radio.service";
import { StorageService } from "../../core/services/storage.service";
import { ToastService } from "../../core/services/toast.service";
import { LibraryService } from "../library/services/library.service";
import { ExploreService, ExploreSection } from "./explore.service";
import { Song, songKey, sameSong, sourceHost } from "../../core/models/song.model";
import {
  faArrowRotateRight,
  faCompass,
  faTriangleExclamation,
  faWaveSquare,
} from "../../shared/icons";

@Component({
  selector: "app-explore-page",
  standalone: true,
  imports: [
    PageContentComponent,
    PlayerButtonComponent,
    SearchBarComponent,
    FaIconComponent,
    EmptyStateComponent,
    RouterLink,
  ],
  templateUrl: "./explore-page.component.html",
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ExplorePageComponent implements OnInit {
  readonly explore = inject(ExploreService);
  private readonly pinnedPicks = signal<{ section: ExploreSection; song: Song }[]>([]);
  readonly featuredPicks = computed(() => {
    const pinned = this.pinnedPicks();
    if (pinned.some(pick => sameSong(this.playlist.currentSong(), pick.song))) return pinned;
    const used = new Set<string>();
    return this.explore.sections().flatMap(section => {
      const available = section.songs.filter(s => !used.has(songKey(s)));
      const song = available.find(s => s.image?.trim() && !s.image.includes("no_album_art")) ?? available[0];
      if (!song || used.size >= 3) return [];
      used.add(songKey(song));
      return [{ section, song }];
    });
  });
  readonly radio = inject(RadioService);
  private readonly player = inject(PlayerService);
  private readonly playlist = inject(PlaylistService);
  readonly playerStatus = PlayerStatus;
  readonly providerLabel = sourceHost;
  private readonly library = inject(LibraryService);
  private readonly storage = inject(StorageService);
  private readonly toast = inject(ToastService);

  readonly faArrowRotateRight = faArrowRotateRight;
  readonly faCompass = faCompass;
  readonly faTriangleExclamation = faTriangleExclamation;
  readonly faWaveSquare = faWaveSquare;
  readonly skeletons = [0, 1, 2];

  ngOnInit(): void {
    void this.explore.load();
  }

  onCoverError(event: Event): void {
    const image = event.target as HTMLImageElement;
    if (!image.src.endsWith("/no_album_art.jpg")) image.src = "no_album_art.jpg";
  }

  onRefresh(): void {
    this.pinnedPicks.set([]);
    void this.explore.refresh();
  }

  statusFor(song: Song): PlayerStatus {
    return sameSong(this.playlist.currentSong(), song) ? this.player.status() : PlayerStatus.Stopped;
  }

  actionLabel(song: Song): string {
    const status = this.statusFor(song);
    const action = status === PlayerStatus.Playing ? "Pause" : status === PlayerStatus.Loading ? "Loading" : status === PlayerStatus.Error ? "Retry" : "Play";
    return `${action} ${song.title} by ${song.artist}`;
  }

  playQuickPick(section: ExploreSection, song: Song): void {
    this.pinnedPicks.set(this.featuredPicks());
    this.playFromShelf(section, song);
  }

  playFromShelf(section: ExploreSection, song: Song): void {
    if (sameSong(this.playlist.currentSong(), song)) {
      if (this.player.status() === PlayerStatus.Playing) this.player.pause();
      else if (this.player.status() !== PlayerStatus.Loading) void this.player.play();
      return;
    }
    void this.player.playFromList(section.songs, song);
  }

  playSeed(song: Song): void {
    this.playFromShelf({ id: "seed", title: "", subtitle: "", songs: [song] }, song);
  }

  radioBusy(section: ExploreSection): boolean {
    const seed = section.seedSong ?? section.songs[0];
    if (!seed) return this.radio.loading();
    if (section.id === "vault") return this.radio.isLoadingStation();
    return this.radio.isLoadingSong(seed);
  }

  async startShelfRadio(section: ExploreSection, event: Event): Promise<void> {
    event.stopPropagation();
    if (this.radio.loading()) return;

    if (section.id === "vault") {
      const seed = await this.radio.startFromVault(
        this.library.songs(),
        this.storage.listenStats(),
      );
      if (!seed) {
        this.toast.show(this.radio.error() || "Couldn't start radio");
        return;
      }
      this.toast.show(`Radio · ${seed.artist}`);
      await this.player.setSong(seed, { fromQueue: true });
      return;
    }

    const seed = section.seedSong ?? section.songs[0];
    if (!seed) return;
    const vault = this.library.songs();
    const ok = await this.radio.start(seed, {
      excludeLinks: vault.map((s) => s.link).filter(Boolean),
      excludeKeys: vault.map((s) => songKey(s)).filter(Boolean),
      loadingKey: songKey(seed),
    });
    if (!ok) {
      this.toast.show(this.radio.error() || "Couldn't start radio");
      return;
    }
    this.toast.show(`Radio · ${seed.artist}`);
    await this.player.setSong(seed, { fromQueue: true });
  }

  cover(song: Song): string {
    return song.image?.trim() || "no_album_art.jpg";
  }
}
