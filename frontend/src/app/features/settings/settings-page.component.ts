import {
  ChangeDetectionStrategy,
  Component,
  computed,
  inject,
  resource,
  signal,
} from "@angular/core";
import { HttpClient } from "@angular/common/http";
import { Router } from "@angular/router";
import { firstValueFrom } from "rxjs";
import { FontAwesomeModule } from "@fortawesome/angular-fontawesome";
import { PageContentComponent } from "../../shared/components/page-content/page-content.component";
import { StorageInfoComponent } from "../library/components/storage-info/storage-info.component";
import { SpotifyImportComponent } from "../library/components/spotify-import/spotify-import.component";
import { UserService } from "../library/services/user.service";
import { LibraryService } from "../library/services/library.service";
import { OfflineStorageService } from "../library/services/offline-storage.service";
import { PlayerService } from "../../core/services/player.service";
import {
  SettingsService,
  SuggestRegion,
} from "../../core/services/settings.service";
import { StorageService } from "../../core/services/storage.service";
import { ToastService } from "../../core/services/toast.service";
import { ExploreService } from "../explore/explore.service";
import { environment } from "../../../environments/environment";
import {
  faCircleNotch,
  faCheckCircle,
  faTriangleExclamation,
  faRightFromBracket,
} from "../../shared/icons";

interface SourceStatus {
  name: string;
  host: string;
  ok: boolean;
  ms: number;
}

interface SourcesResponse {
  sources: SourceStatus[];
}

@Component({
  selector: "app-settings-page",
  standalone: true,
  imports: [
    PageContentComponent,
    StorageInfoComponent,
    SpotifyImportComponent,
    FontAwesomeModule,
  ],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <app-page-content>
      <div class="space-y-4 pt-1 max-w-xl mx-auto">
        <header class="mb-1">
          <span
            class="text-[10px] font-bold text-base-content/50 uppercase tracking-[0.18em]"
          >
            App
          </span>
          <h1
            class="font-bold text-2xl tracking-tight bg-gradient-to-br from-base-content via-base-content/95 to-primary/80 bg-clip-text text-transparent"
          >
            Settings
          </h1>
        </header>

        <section class="premium-card p-3.5 space-y-3" aria-label="Offline vault">
          <div class="flex items-center justify-between gap-3">
            <div class="min-w-0">
              <h2 class="text-sm font-bold tracking-tight">Auto-save offline</h2>
              <p class="text-[11px] text-base-content/45 mt-0.5">
                Cache audio when you add to vault or import from Spotify.
              </p>
            </div>
            <button
              type="button"
              role="switch"
              class="shrink-0 w-11 h-6 rounded-full transition-colors relative"
              [class.bg-primary]="autoOfflineCache()"
              [class.bg-base-content/15]="!autoOfflineCache()"
              [attr.aria-checked]="autoOfflineCache()"
              aria-label="Auto-save vault songs for offline"
              (click)="toggleAutoOfflineCache()"
            >
              <span
                class="absolute top-0.5 left-0.5 w-5 h-5 rounded-full bg-base-100 transition-transform"
                [class.translate-x-5]="autoOfflineCache()"
              ></span>
            </button>
          </div>
          <div class="pt-1 border-t border-base-content/8">
            <app-storage-info />
          </div>
        </section>

        @if (isLoggedIn()) {
          <app-spotify-import />
        }

        <section class="premium-card p-3.5 space-y-3" aria-label="Player">
          <div class="flex items-center justify-between gap-3">
            <div class="min-w-0">
              <h2 class="text-sm font-bold tracking-tight">Download button</h2>
              <p class="text-[11px] text-base-content/45 mt-0.5">
                Show Save MP3 on the full player.
              </p>
            </div>
            <button
              type="button"
              role="switch"
              class="shrink-0 w-11 h-6 rounded-full transition-colors relative"
              [class.bg-primary]="showPlayerDownload()"
              [class.bg-base-content/15]="!showPlayerDownload()"
              [attr.aria-checked]="showPlayerDownload()"
              aria-label="Show download button in full player"
              (click)="toggleShowPlayerDownload()"
            >
              <span
                class="absolute top-0.5 left-0.5 w-5 h-5 rounded-full bg-base-100 transition-transform"
                [class.translate-x-5]="showPlayerDownload()"
              ></span>
            </button>
          </div>
          <div class="flex items-center justify-between gap-3 pt-1 border-t border-base-content/8">
            <div class="min-w-0">
              <h2 class="text-sm font-bold tracking-tight">Clear history</h2>
              <p class="text-[11px] text-base-content/45 mt-0.5">
                Remove local listen history ({{ recentCount() }} tracks).
              </p>
            </div>
            <button
              type="button"
              class="shrink-0 h-9 px-3 rounded-xl text-xs font-semibold text-error/80 active:bg-error/10 disabled:opacity-40"
              [disabled]="!recentCount()"
              (click)="clearHistory()"
            >
              Clear
            </button>
          </div>
          <div class="flex items-center justify-between gap-3 pt-1 border-t border-base-content/8">
            <div class="min-w-0">
              <h2 class="text-sm font-bold tracking-tight">Refresh Explore</h2>
              <p class="text-[11px] text-base-content/45 mt-0.5">
                Re-fetch charts + personalized shelves (bypasses 24h cache).
              </p>
            </div>
            <button
              type="button"
              class="shrink-0 h-9 px-3 rounded-xl text-xs font-semibold bg-primary/15 text-primary disabled:opacity-40"
              [disabled]="exploreRefreshing()"
              (click)="refreshExplore()"
            >
              @if (exploreRefreshing()) {
                Refreshing…
              } @else {
                Refresh
              }
            </button>
          </div>
        </section>

        <section class="premium-card p-3.5 space-y-2.5" aria-label="Suggestions">
          <div>
            <h2 class="text-sm font-bold tracking-tight">Suggestion region</h2>
            <p class="text-[11px] text-base-content/45 mt-0.5">
              Music autocomplete bias. Default Romania.
            </p>
          </div>
          <div
            class="grid grid-cols-2 gap-1 p-1 rounded-xl bg-base-content/[0.06]"
            role="group"
            aria-label="Suggestion region"
          >
            <button
              type="button"
              class="h-9 rounded-lg text-xs font-semibold transition-colors"
              [class.bg-base-100]="suggestRegion() === 'ro'"
              [class.text-base-content]="suggestRegion() === 'ro'"
              [class.text-base-content/50]="suggestRegion() !== 'ro'"
              (click)="setSuggestRegion('ro')"
            >
              Romania
            </button>
            <button
              type="button"
              class="h-9 rounded-lg text-xs font-semibold transition-colors"
              [class.bg-base-100]="suggestRegion() === 'device'"
              [class.text-base-content]="suggestRegion() === 'device'"
              [class.text-base-content/50]="suggestRegion() !== 'device'"
              (click)="setSuggestRegion('device')"
            >
              My device
            </button>
          </div>
        </section>

        <section class="space-y-2" aria-label="Source health">
          <div class="flex items-center justify-between gap-3 px-0.5">
            <h2 class="text-sm font-bold tracking-tight">Source health</h2>
            <button
              type="button"
              class="h-9 px-3 rounded-xl text-xs font-semibold bg-base-content/[0.06] text-base-content/70 disabled:opacity-50"
              (click)="reload()"
              [disabled]="sources.isLoading()"
            >
              Refresh
            </button>
          </div>

          @if (sources.isLoading() && !sources.value()) {
            <div class="py-6 flex justify-center">
              <fa-icon
                [icon]="faCircleNotch"
                class="text-lg text-primary animate-spin"
              />
            </div>
          } @else if (sources.error()) {
            <div class="px-1 py-4 text-center space-y-1">
              <fa-icon
                [icon]="faTriangleExclamation"
                class="text-error text-base"
              />
              <p class="text-base-content/70 text-xs">
                Could not load source health.
              </p>
            </div>
          } @else {
            <ul class="flex flex-col">
              @for (s of sources.value()?.sources ?? []; track s.name) {
                <li
                  class="px-2.5 py-2.5 rounded-xl flex items-center justify-between gap-3 hover:bg-base-content/[0.04]"
                >
                  <div class="min-w-0">
                    <p class="text-sm font-semibold truncate">{{ s.name }}</p>
                    <p class="text-[11px] text-base-content/40 truncate">
                      {{ s.host }} · {{ s.ms }}ms
                    </p>
                  </div>
                  @if (s.ok) {
                    <span
                      class="shrink-0 inline-flex items-center gap-1 text-success text-[10px] font-bold uppercase tracking-wide"
                    >
                      <fa-icon [icon]="faCheckCircle" class="text-[10px]" />
                      Up
                    </span>
                  } @else {
                    <span
                      class="shrink-0 inline-flex items-center gap-1 text-error text-[10px] font-bold uppercase tracking-wide"
                    >
                      <fa-icon
                        [icon]="faTriangleExclamation"
                        class="text-[10px]"
                      />
                      Down
                    </span>
                  }
                </li>
              }
            </ul>
            @if (checkedAt()) {
              <p class="text-center text-[10px] text-base-content/30">
                Checked {{ checkedAt() }}
              </p>
            }
          }
        </section>

        <section class="premium-card p-3.5 space-y-1.5" aria-label="Privacy">
          <h2 class="text-sm font-bold tracking-tight">Account note</h2>
          <p class="text-[11px] text-base-content/45 leading-relaxed">
            Username is your only sign-in. Anyone who knows it can load that
            vault. Prefer a unique username; sign out on shared devices.
          </p>
        </section>

        @if (isLoggedIn()) {
          <button
            type="button"
            class="w-full h-10 rounded-xl border border-error/25 text-error font-semibold text-xs flex items-center justify-center gap-1.5"
            (click)="signOut()"
          >
            <fa-icon [icon]="faRightFromBracket" class="text-[11px]" />
            <span>Sign out{{ username() ? " · " + username() : "" }}</span>
          </button>
        }
      </div>
    </app-page-content>
  `,
})
export class SettingsPageComponent {
  private http = inject(HttpClient);
  private userService = inject(UserService);
  private library = inject(LibraryService);
  private offline = inject(OfflineStorageService);
  private playerService = inject(PlayerService);
  private settingsService = inject(SettingsService);
  private storage = inject(StorageService);
  private explore = inject(ExploreService);
  private toast = inject(ToastService);
  private router = inject(Router);
  exploreRefreshing = this.explore.loading;
  faCircleNotch = faCircleNotch;
  faCheckCircle = faCheckCircle;
  faTriangleExclamation = faTriangleExclamation;
  faRightFromBracket = faRightFromBracket;
  checkedAt = signal("");
  isLoggedIn = computed(() => !!this.userService.user());
  username = computed(() => this.userService.user()?.username || "");
  suggestRegion = this.settingsService.suggestRegion;
  showPlayerDownload = this.settingsService.showPlayerDownload;
  autoOfflineCache = this.settingsService.autoOfflineCache;
  recentCount = computed(() => this.storage.recentSongs().length);

  sources = resource({
    loader: async () => {
      const data = await firstValueFrom(
        this.http.get<SourcesResponse>(`${environment.API_URL}/health/sources`),
      );
      this.checkedAt.set(new Date().toLocaleTimeString());
      return data;
    },
  });

  setSuggestRegion(region: SuggestRegion) {
    this.settingsService.setSuggestRegion(region);
  }

  toggleShowPlayerDownload() {
    this.settingsService.toggleShowPlayerDownload();
  }

  toggleAutoOfflineCache() {
    const turningOn = !this.autoOfflineCache();
    this.settingsService.toggleAutoOfflineCache();
    if (turningOn) {
      // Backfill vault audio so existing likes become offline-ready.
      const vault = this.library.songs();
      if (vault.length) {
        void this.offline.cacheAllSongs(vault).then((r) => {
          this.toast.show(
            r.saved
              ? `Saved ${r.saved} offline`
              : r.skipped
                ? "Vault already offline"
                : "Couldn't save offline",
          );
        });
      }
    }
  }

  clearHistory() {
    this.storage.clearRecents();
  }

  async refreshExplore() {
    if (this.explore.loading()) return;
    await this.explore.refresh();
    this.toast.show(
      this.explore.error() ? "Explore refresh failed" : "Explore refreshed",
    );
  }

  reload() {
    this.sources.reload();
  }

  signOut() {
    this.playerService.reset();
    this.userService.resetUser();
    void this.router.navigateByUrl("/library");
  }
}
