import {
  ChangeDetectionStrategy,
  Component,
  OnDestroy,
  computed,
  inject,
  signal,
} from "@angular/core";
import { HttpClient, HttpErrorResponse } from "@angular/common/http";
import {
  EmptyError,
  Observable,
  Subject,
  firstValueFrom,
  takeUntil,
} from "rxjs";
import { FontAwesomeModule } from "@fortawesome/angular-fontawesome";
import { environment } from "../../../../../environments/environment";
import { Song, songKey } from "../../../../core/models/song.model";
import { LibraryService } from "../../services/library.service";
import { UserService } from "../../services/user.service";
import { faLink, faXmark } from "../../../../shared/icons";

type RowStatus =
  | "pending"
  | "looking"
  | "saving"
  | "added"
  | "skipped"
  | "miss"
  | "failed"
  | "cancelled";

type ImportRow = {
  artist: string;
  title: string;
  status: RowStatus;
  /** Matched catalog title when it differs from Spotify. */
  matched?: string;
};

type PlaylistPayload = {
  name: string;
  tracks: { artist: string; title: string }[];
};

@Component({
  selector: "app-spotify-import",
  standalone: true,
  imports: [FontAwesomeModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: {
    class: "block",
  },
  template: `
    @if (!open()) {
      <button
        type="button"
        class="w-full h-10 px-3.5 rounded-xl text-xs font-semibold bg-base-content/[0.06] text-base-content/70 active:bg-primary/15 active:text-primary inline-flex items-center justify-center gap-2"
        (click)="open.set(true)"
      >
        <fa-icon [icon]="faLink" class="text-[11px]" />
        Import Spotify
      </button>
    } @else {
      <div
        class="premium-card p-3.5 space-y-3"
        aria-label="Import Spotify playlist"
      >
        <div class="flex items-start justify-between gap-2">
          <div class="min-w-0">
            <h2 class="text-sm font-bold tracking-tight">Import Spotify</h2>
            <p class="text-[11px] text-base-content/45 mt-0.5">
              Public playlist · cancel anytime
            </p>
          </div>
          <button
            type="button"
            class="w-8 h-8 rounded-full text-base-content/40 active:bg-base-content/10"
            (click)="close()"
            aria-label="Close import"
          >
            <fa-icon [icon]="faXmark" />
          </button>
        </div>

        <div class="flex gap-2">
          <input
            type="url"
            class="input input-sm grow bg-base-content/[0.06] border-0 focus:outline-none focus:ring-1 focus:ring-primary/40 rounded-xl"
            placeholder="https://open.spotify.com/playlist/…"
            [value]="url()"
            [disabled]="running()"
            enterkeyhint="done"
            (input)="url.set($any($event.target).value)"
            (keydown.enter)="start()"
          />
          @if (running()) {
            <button
              type="button"
              class="shrink-0 h-8 px-3 rounded-xl text-xs font-semibold bg-error/15 text-error"
              (click)="cancel()"
            >
              Cancel
            </button>
          } @else {
            <button
              type="button"
              class="shrink-0 h-8 px-3 rounded-xl text-xs font-semibold bg-primary text-primary-content disabled:opacity-40"
              [disabled]="!url().trim()"
              (click)="start()"
            >
              Import
            </button>
          }
        </div>

        @if (error()) {
          <p class="text-xs text-error/90">{{ error() }}</p>
        }

        @if (rows().length) {
          <div class="space-y-1.5" aria-label="Import progress">
            <div class="flex items-center justify-between gap-2">
              <p
                class="text-[11px] font-medium text-base-content/50 truncate min-w-0"
              >
                {{ playlistName() || "Playlist" }}
              </p>
              <p
                class="shrink-0 text-[11px] font-semibold tabular-nums text-base-content/60"
              >
                {{ progress().settled }}/{{ progress().total }}
                · {{ progress().pct }}%
              </p>
            </div>

            <div
              class="h-2 w-full rounded-full bg-base-content/10 overflow-hidden flex"
              role="progressbar"
              [attr.aria-valuemin]="0"
              [attr.aria-valuemax]="progress().total"
              [attr.aria-valuenow]="progress().settled"
              [attr.aria-valuetext]="progressText()"
              [attr.aria-busy]="running()"
            >
              @if (progress().addedPct) {
                <span
                  class="h-full bg-success transition-[width] duration-300 ease-out"
                  [style.width.%]="progress().addedPct"
                ></span>
              }
              @if (progress().skippedPct) {
                <span
                  class="h-full bg-base-content/35 transition-[width] duration-300 ease-out"
                  [style.width.%]="progress().skippedPct"
                ></span>
              }
              @if (progress().missPct) {
                <span
                  class="h-full bg-warning/80 transition-[width] duration-300 ease-out"
                  [style.width.%]="progress().missPct"
                ></span>
              }
              @if (progress().failedPct) {
                <span
                  class="h-full bg-error/70 transition-[width] duration-300 ease-out"
                  [style.width.%]="progress().failedPct"
                ></span>
              }
              @if (progress().cancelledPct) {
                <span
                  class="h-full bg-error/40 transition-[width] duration-300 ease-out"
                  [style.width.%]="progress().cancelledPct"
                ></span>
              }
              @if (progress().activePct) {
                <span
                  class="h-full bg-primary/70 animate-pulse transition-[width] duration-300 ease-out"
                  [style.width.%]="progress().activePct"
                ></span>
              }
            </div>

            <p class="text-[10px] text-base-content/40 truncate">
              {{ progressLabel() }}
            </p>
          </div>

          @if (done() && !running()) {
            <div
              class="rounded-xl bg-base-content/[0.05] px-3 py-2.5 space-y-1"
              role="status"
              aria-label="Import report"
            >
              <p class="text-xs font-bold tracking-tight">
                {{ reportTitle() }}
              </p>
              <p class="text-[11px] text-base-content/60 leading-relaxed">
                {{ reportBody() }}
              </p>
            </div>
          }

          <ul
            class="max-h-56 overflow-y-auto space-y-1 pr-0.5 overscroll-contain touch-pan-y"
            aria-live="polite"
          >
            @for (row of rows(); track $index) {
              <li
                class="flex items-start gap-2 text-xs py-1.5 px-2 rounded-lg bg-base-content/[0.04]"
              >
                <span
                  class="shrink-0 w-[4.5rem] pt-0.5 text-[10px] font-bold uppercase tracking-wide"
                  [class.text-base-content/35]="row.status === 'pending'"
                  [class.text-primary]="
                    row.status === 'looking' || row.status === 'saving'
                  "
                  [class.text-success]="row.status === 'added'"
                  [class.text-base-content/45]="row.status === 'skipped'"
                  [class.text-warning/80]="row.status === 'miss'"
                  [class.text-error/80]="row.status === 'failed'"
                  [class.text-error/60]="row.status === 'cancelled'"
                >
                  {{ statusLabel(row.status) }}
                </span>
                <span class="min-w-0 flex-1">
                  <span class="block truncate text-base-content/75">
                    {{ row.artist }} — {{ row.title }}
                  </span>
                  @if (row.matched) {
                    <span
                      class="block truncate text-[10px] text-base-content/40 mt-0.5"
                    >
                      Matched {{ row.matched }}
                    </span>
                  }
                </span>
              </li>
            }
          </ul>
        }
      </div>
    }
  `,
})
export class SpotifyImportComponent implements OnDestroy {
  private http = inject(HttpClient);
  private library = inject(LibraryService);
  private user = inject(UserService);

  faLink = faLink;
  faXmark = faXmark;

  open = signal(false);
  url = signal("");
  error = signal("");
  playlistName = signal("");
  rows = signal<ImportRow[]>([]);
  running = signal(false);
  done = signal(false);

  private stop$ = new Subject<void>();
  private gen = 0;

  ngOnDestroy(): void {
    this.stop$.next();
    this.stop$.complete();
  }

  progress = computed(() => {
    const rows = this.rows();
    const total = rows.length;
    let added = 0;
    let skipped = 0;
    let miss = 0;
    let failed = 0;
    let cancelled = 0;
    let active = 0;
    for (const r of rows) {
      switch (r.status) {
        case "added":
          added++;
          break;
        case "skipped":
          skipped++;
          break;
        case "miss":
          miss++;
          break;
        case "failed":
          failed++;
          break;
        case "cancelled":
          cancelled++;
          break;
        case "looking":
        case "saving":
          active++;
          break;
      }
    }
    const settled = added + skipped + miss + failed + cancelled;
    const pct = total ? Math.round((settled / total) * 100) : 0;
    const share = (n: number) => (total ? (n / total) * 100 : 0);
    return {
      total,
      added,
      skipped,
      miss,
      failed,
      cancelled,
      active,
      settled,
      pct,
      addedPct: share(added),
      skippedPct: share(skipped),
      missPct: share(miss),
      failedPct: share(failed),
      cancelledPct: share(cancelled),
      activePct: share(active),
    };
  });

  progressText(): string {
    const p = this.progress();
    if (!p.total) return "";
    return `${p.settled} of ${p.total} tracks processed, ${p.pct} percent`;
  }

  progressLabel(): string {
    const p = this.progress();
    if (!p.total) return "";
    if (this.running()) {
      if (p.active) return `Working… ${p.settled}/${p.total}`;
      return "Starting…";
    }
    return this.reportBody();
  }

  reportTitle(): string {
    const p = this.progress();
    if (p.cancelled && p.settled < p.total) return "Import stopped";
    if (p.added === 0 && p.skipped === 0) return "Nothing added";
    if (p.added === p.total) return "All tracks added";
    return "Import complete";
  }

  reportBody(): string {
    const p = this.progress();
    if (!p.total) return "";
    const parts: string[] = [];
    parts.push(`${p.added} added`);
    if (p.skipped) parts.push(`${p.skipped} already in vault`);
    if (p.miss) parts.push(`${p.miss} not found`);
    if (p.failed) parts.push(`${p.failed} failed to save`);
    if (p.cancelled) parts.push(`${p.cancelled} cancelled`);
    return parts.join(" · ");
  }

  statusLabel(s: RowStatus): string {
    switch (s) {
      case "pending":
        return "Queued";
      case "looking":
        return "Looking";
      case "saving":
        return "Saving";
      case "added":
        return "Added";
      case "skipped":
        return "In vault";
      case "miss":
        return "Not found";
      case "failed":
        return "Failed";
      case "cancelled":
        return "Stopped";
    }
  }

  cancel(): void {
    this.gen++;
    this.stop$.next();
    this.rows.update((list) =>
      list.map((r) =>
        r.status === "pending" ||
        r.status === "looking" ||
        r.status === "saving"
          ? { ...r, status: "cancelled" as const }
          : r,
      ),
    );
    this.running.set(false);
    this.done.set(true);
  }

  close(): void {
    if (this.running()) this.cancel();
    this.open.set(false);
    this.reset();
  }

  private reset() {
    this.gen++;
    this.stop$.next();
    this.url.set("");
    this.error.set("");
    this.playlistName.set("");
    this.rows.set([]);
    this.done.set(false);
    this.running.set(false);
  }

  async start() {
    const link = this.url().trim();
    const user = this.user.user();
    if (!link || !user || this.running()) return;

    this.gen++;
    const gen = this.gen;
    this.stop$.next();

    this.running.set(true);
    this.done.set(false);
    this.error.set("");
    this.rows.set([]);
    this.playlistName.set("");

    try {
      const pl = await this.req(
        this.http.get<PlaylistPayload>(
          `${environment.API_URL}/spotify/playlist`,
          { params: { url: link } },
        ),
      );
      if (gen !== this.gen) return;
      if (!pl) {
        this.running.set(false);
        return;
      }

      this.playlistName.set(pl.name || "Playlist");
      const rows: ImportRow[] = (pl.tracks ?? []).map((t) => ({
        artist: t.artist,
        title: t.title,
        status: "pending",
      }));
      this.rows.set(rows);

      const vaultKeys = new Set(
        this.library
          .songs()
          .map((s) => songKey(s))
          .filter(Boolean),
      );
      const vaultLinks = new Set(
        this.library
          .songs()
          .map((s) => s.link)
          .filter(Boolean),
      );
      const orders = this.library.reserveTopOrders(rows.length);
      let orderIdx = 0;

      for (let i = 0; i < rows.length; i++) {
        if (gen !== this.gen) break;

        const row = this.rows()[i];
        if (!row || row.status === "cancelled") break;

        const key = songKey(row);
        if (key && vaultKeys.has(key)) {
          this.patch(i, "skipped");
          continue;
        }

        this.patch(i, "looking");
        let song: Song | null;
        try {
          song = await this.req(
            this.http.get<Song>(`${environment.API_URL}/resolve`, {
              params: {
                artist: row.artist,
                title: row.title,
                strict: "1",
              },
            }),
          );
        } catch (err: unknown) {
          if (gen !== this.gen) break;
          this.patch(i, httpStatus(err) === 404 ? "miss" : "failed");
          continue;
        }

        if (gen !== this.gen) break;
        if (song === null) {
          this.patch(i, "cancelled");
          break;
        }
        if (!isPlayableMatch(row, song)) {
          this.patch(i, "miss");
          continue;
        }

        const sk = songKey(song);
        if (
          (sk && vaultKeys.has(sk)) ||
          (song.link && vaultLinks.has(song.link))
        ) {
          this.patch(i, "skipped");
          continue;
        }

        const matched =
          songKey(row) !== sk
            ? `${song.artist} — ${song.title}`
            : undefined;
        this.patch(i, "saving", matched);

        try {
          const saved = await this.req(
            this.library.addToFavorites(song, user.id, {
              order: orders[orderIdx++] ?? this.library.reserveTopOrders(1)[0],
            }),
          );
          if (gen !== this.gen) break;
          if (saved === null) {
            this.patch(i, "cancelled");
            break;
          }
          if (sk) vaultKeys.add(sk);
          if (song.link) vaultLinks.add(song.link);
          this.patch(i, "added", matched);
        } catch (err: unknown) {
          if (gen !== this.gen) break;
          if (httpStatus(err) === 409) {
            if (sk) vaultKeys.add(sk);
            if (song.link) vaultLinks.add(song.link);
            this.patch(i, "skipped");
          } else {
            this.patch(i, "failed");
          }
        }
      }

      if (gen === this.gen) this.done.set(true);
    } catch (e: unknown) {
      if (gen !== this.gen) return;
      const err = e as { error?: { error?: string } };
      this.error.set(err?.error?.error || "Couldn't load that playlist");
    } finally {
      if (gen === this.gen) this.running.set(false);
    }
  }

  /** firstValueFrom + takeUntil(stop$). null = cancelled mid-request. */
  private async req<T>(source: Observable<T>): Promise<T | null> {
    try {
      return await firstValueFrom(source.pipe(takeUntil(this.stop$)));
    } catch (e) {
      if (e instanceof EmptyError) return null;
      throw e;
    }
  }

  private patch(i: number, status: RowStatus, matched?: string) {
    this.rows.update((list) => {
      if (i < 0 || i >= list.length) return list;
      const cur = list[i];
      if (cur.status === status && cur.matched === matched) return list;
      const next = list.slice();
      next[i] = { ...cur, status, matched: matched ?? cur.matched };
      return next;
    });
  }
}

function httpStatus(err: unknown): number | undefined {
  if (err instanceof HttpErrorResponse) return err.status;
  return (err as { status?: number })?.status;
}

/** Playable + artist/title overlap — belt-and-suspenders with Fiber strict=1. */
function isPlayableMatch(
  want: Pick<Song, "artist" | "title">,
  got: Song | null | undefined,
): boolean {
  if (!got?.link?.trim() || !got.title?.trim() || !got.artist?.trim()) {
    return false;
  }
  const wantKey = songKey(want);
  const gotKey = songKey(got);
  if (wantKey && gotKey && wantKey === gotKey) return true;

  const wantArt = (want.artist || "").toLowerCase().trim();
  const gotArt = (got.artist || "").toLowerCase().trim();
  const wantTitle = wantKey.split("|")[1] || "";
  const gotTitle = gotKey.split("|")[1] || "";
  if (!wantArt || !gotArt || !wantTitle || !gotTitle) return false;

  const artistOK =
    gotArt === wantArt ||
    gotArt.includes(wantArt) ||
    wantArt.includes(gotArt);
  const titleOK =
    gotTitle === wantTitle ||
    gotTitle.includes(wantTitle) ||
    wantTitle.includes(gotTitle);
  return artistOK && titleOK;
}
