import {
  Component,
  ChangeDetectionStrategy,
  ElementRef,
  OnDestroy,
  computed,
  effect,
  input,
  output,
  signal,
  untracked,
  viewChild,
} from "@angular/core";
import { Song } from "../../../core/models/song.model";
import { SongComponent } from "../../song/song.component";
import { EmptyStateComponent } from "../../empty-state/empty-state.component";
import { SkeletonComponent } from "../skeleton/skeleton.component";
import { faMusic, IconDefinition } from "../../icons";

@Component({
  selector: "app-song-list",
  template: `
    @if (isLoading()) {
      <app-skeleton type="song" [count]="skeletonCount()" />
    } @else {
      <ul class="flex flex-col">
        @for (song of visibleSongs(); track song.link) {
          <li class="group/item list-none">
            <app-song
              [song]="song"
              [selectMode]="selectMode()"
              [selected]="selectedIds().has(song.id)"
              [radioAction]="radioAction()"
              (reorder)="reorder.emit($event)"
              (playlistChange)="playlistChange.emit()"
              (toggleSelect)="toggleSelect.emit($event)"
            />
          </li>
        } @empty {
          <app-empty-state
            [icon]="emptyStateIcon()"
            [title]="emptyStateTitle()"
            [description]="emptyStateDescription()"
          />
        }
        @if (hasMore()) {
          <li
            #sentinel
            class="h-8 list-none"
            aria-hidden="true"
          ></li>
        }
      </ul>
    }
  `,
  styles: [
    `
      :host {
        display: block;
      }
    `,
  ],
  standalone: true,
  imports: [SongComponent, EmptyStateComponent, SkeletonComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class SongListComponent implements OnDestroy {
  songs = input.required<Song[]>();
  isLoading = input(false);
  selectMode = input(false);
  /** Vault rows: radio mini-button instead of add-to-queue. */
  radioAction = input(false);
  selectedIds = input<ReadonlySet<string>>(new Set());
  /** 0 = render all. Vault passes e.g. 24 and grows on scroll. */
  chunkSize = input(0);
  skeletonCount = input(5);
  emptyStateIcon = input<IconDefinition>(faMusic);
  emptyStateTitle = input("No songs found");
  emptyStateDescription = input("Try searching for something else.");
  reorder = output<{ from: string; to: string }>();
  playlistChange = output<void>();
  toggleSelect = output<string>();

  private sentinel = viewChild<ElementRef<HTMLElement>>("sentinel");
  private visibleCount = signal(0);
  private observer: IntersectionObserver | null = null;

  visibleSongs = computed(() => {
    const all = this.songs();
    const n = this.chunkSize();
    if (n <= 0) return all;
    const cap = this.visibleCount();
    return all.slice(0, cap > 0 ? Math.min(cap, all.length) : Math.min(n, all.length));
  });

  hasMore = computed(() => {
    const n = this.chunkSize();
    if (n <= 0) return false;
    return this.visibleSongs().length < this.songs().length;
  });

  constructor() {
    effect(() => {
      const all = this.songs();
      const n = this.chunkSize();
      const loading = this.isLoading();
      if (loading || n <= 0) {
        untracked(() => this.teardownObserver());
        return;
      }
      untracked(() => {
        const cur = this.visibleCount();
        if (cur === 0) this.visibleCount.set(Math.min(n, all.length));
        else if (cur > all.length) this.visibleCount.set(all.length);
      });
      // Sentinel remounts when hasMore flips — rebind after paint.
      queueMicrotask(() => this.watchSentinel());
    });
  }

  ngOnDestroy(): void {
    this.teardownObserver();
  }

  private revealMore() {
    const n = this.chunkSize();
    const total = this.songs().length;
    if (n <= 0 || !total) return;
    this.visibleCount.update((c) => Math.min(total, (c || n) + n));
    // Rebind after layout so a still-visible sentinel can request the next page.
    requestAnimationFrame(() => this.watchSentinel());
  }

  private watchSentinel() {
    this.teardownObserver();
    if (!this.hasMore()) return;
    const el = this.sentinel()?.nativeElement;
    if (!el) return;
    this.observer = new IntersectionObserver(
      (entries) => {
        if (!entries.some((e) => e.isIntersecting)) return;
        // One page per intersection — avoid sync multi-fire dumping the whole vault.
        this.teardownObserver();
        this.revealMore();
      },
      { root: null, rootMargin: "120px", threshold: 0 },
    );
    this.observer.observe(el);
  }

  private teardownObserver() {
    this.observer?.disconnect();
    this.observer = null;
  }
}
