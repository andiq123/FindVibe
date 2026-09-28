import {
  Component,
  effect,
  computed,
  signal,
  OnInit,
  inject,
  ChangeDetectionStrategy,
} from "@angular/core";
import { HttpClient } from "@angular/common/http";
import { environment } from "../../../environments/environment";
import { RouterLink } from "@angular/router";
import { FaIconComponent } from "@fortawesome/angular-fontawesome";
import { PageContentComponent } from "../../shared/components/page-content/page-content.component";
import { EmptyStateComponent } from "../../shared/empty-state/empty-state.component";
import { SearchBarComponent } from "../search/search-bar/search-bar.component";
import { PlayerService } from "../../core/services/player.service";
import { RadioService } from "../../core/services/radio.service";
import { StorageService } from "../../core/services/storage.service";
import { ToastService } from "../../core/services/toast.service";
import { LibraryService } from "../library/services/library.service";
import { ExploreService, ExploreSection } from "./explore.service";
import { Song, songKey } from "../../core/models/song.model";
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
  private readonly http = inject(HttpClient);
  readonly featuredImage = signal("");
  readonly featuredSong = computed(() => {
    const songs = this.explore.sections()[0]?.songs ?? [];
    return songs.find((song) => song.image?.trim() && !song.image.includes("no_album_art")) ?? songs[0];
  });
  readonly radio = inject(RadioService);
  private readonly player = inject(PlayerService);
  private readonly library = inject(LibraryService);
  private readonly storage = inject(StorageService);
  private readonly toast = inject(ToastService);

  readonly faArrowRotateRight = faArrowRotateRight;
  readonly faCompass = faCompass;
  readonly faTriangleExclamation = faTriangleExclamation;
  readonly faWaveSquare = faWaveSquare;
  readonly skeletons = [0, 1, 2];

  constructor() {
    effect((onCleanup) => {
      const song = this.featuredSong();
      this.featuredImage.set("");
      if (!song) return;
      // Enrich only the prominent cover; never delay discovery or fan out per card.
      const sub = this.http.get<{ image?: string }>(`${environment.API_URL}/cover`, {
        params: { q: `${song.artist} ${song.title}` },
      }).subscribe({
        next: (result) => this.featuredImage.set(result.image || ""),
        error: () => { /* Retain provider art when enrichment is unavailable. */ },
      });
      onCleanup(() => sub.unsubscribe());
    });
  }

  ngOnInit(): void {
    void this.explore.load();
  }

  onCoverError(event: Event): void {
    const image = event.target as HTMLImageElement;
    if (!image.src.endsWith("/no_album_art.jpg")) image.src = "no_album_art.jpg";
  }

  onRefresh(): void {
    void this.explore.refresh();
  }

  /** Shelf songs are already search-resolved — play the list as-is. */
  playFromShelf(section: ExploreSection, song: Song): void {
    void this.player.playFromList(section.songs, song);
  }

  /** Because title seed — play that track alone. */
  playSeed(song: Song): void {
    void this.player.playFromList([song], song);
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
