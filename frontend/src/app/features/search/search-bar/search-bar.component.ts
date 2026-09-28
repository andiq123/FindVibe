import {
  Component,
  computed,
  input,
  signal,
  effect,
  inject,
  untracked,
  ChangeDetectionStrategy,
  HostListener,
  ElementRef,
  OnDestroy,
  DestroyRef,
} from "@angular/core";
import { takeUntilDestroyed } from "@angular/core/rxjs-interop";
import { FontAwesomeModule } from "@fortawesome/angular-fontawesome";
import { faMagnifyingGlass, faArrowUp } from "../../../shared/icons";
import { FormsModule } from "@angular/forms";
import { Router } from "@angular/router";
import { SearchService } from "../services/search.service";
import { SearchStatus } from "../../../core/models/song.model";
import {
  Subject,
  debounceTime,
  distinctUntilChanged,
  switchMap,
  of,
} from "rxjs";

@Component({
  selector: "app-search-bar",
  standalone: true,
  imports: [FontAwesomeModule, FormsModule],
  templateUrl: "./search-bar.component.html",
  styleUrl: "./search-bar.component.scss",
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class SearchBarComponent implements OnDestroy {
  query = input("");
  searchTerm = signal("");
  isFocused = signal(false);
  private searchSubject = new Subject<string>();
  private searchService = inject(SearchService);
  private router = inject(Router);
  private elementRef = inject(ElementRef);
  private destroyRef = inject(DestroyRef);
  suggestions = computed(() => this.searchService.suggestions());
  suggestionsLoading = computed(() => this.searchService.suggestionsLoading());
  showSuggestions = computed(
    () =>
      this.searchTerm().trim().length > 0 &&
      (this.suggestions().length > 0 || this.suggestionsLoading()),
  );
  showCancel = computed(
    () => this.isFocused() || this.searchTerm().length > 0,
  );
  faMagnifyingGlass = faMagnifyingGlass;
  faArrowUpLeft = faArrowUp;

  constructor() {
    effect(() => {
      const q = this.query();
      if (!q) {
        if (untracked(() => this.searchTerm()) !== "") {
          this.searchTerm.set("");
        }
        return;
      }
      this.searchTerm.set(q);
      const lastQuery = untracked(() => this.searchService.lastQuery());
      const searchStatus = untracked(() => this.searchService.status());
      if (q !== lastQuery || searchStatus === SearchStatus.None) {
        this.submitSearchSongs(1);
      }
      this.dismissSuggestions();
    });

    this.searchSubject
      .pipe(
        debounceTime(280),
        distinctUntilChanged(),
        switchMap((term) => {
          const q = term.trim();
          if (!q) {
            this.searchService.resetSuggestions();
            return of([]);
          }
          return this.searchService.getSuggestions(q);
        }),
        takeUntilDestroyed(),
      )
      .subscribe();
  }

  /** Fill the field only — keep focus, refresh suggests for the filled term. */
  fillSuggestion(suggestion: string, event: Event): void {
    event.preventDefault();
    event.stopPropagation();
    this.searchTerm.set(suggestion);
    this.searchSubject.next(suggestion);
  }

  async searchBySuggestion(suggestion: string): Promise<void> {
    this.searchTerm.set(suggestion);
    await this.submit();
  }

  onInput(event: Event): void {
    const value = (event.target as HTMLInputElement).value;
    this.searchTerm.set(value);
    // Always push — empty string cancels in-flight suggest via switchMap.
    this.searchSubject.next(value);
  }

  clearSearch(input: HTMLInputElement): void {
    this.searchTerm.set("");
    this.dismissSuggestions();
    input.focus();
  }

  async cancelSearch(): Promise<void> {
    this.searchTerm.set("");
    this.dismissSuggestions();
    this.isFocused.set(false);
    this.searchService.resetSearch();
    await this.router.navigate(["/explore"], { replaceUrl: true });
  }

  async submit(): Promise<void> {
    const term = this.searchTerm().trim();
    if (!term) return;
    this.dismissSuggestions();
    this.isFocused.set(false);
    if (this.query() !== term) {
      await this.router.navigate([`/songs/${term}`], { replaceUrl: true });
    } else {
      this.submitSearchSongs();
    }
  }

  @HostListener("document:click", ["$event"])
  onClickOutside(event: Event): void {
    if (!this.elementRef.nativeElement.contains(event.target)) {
      this.dismissSuggestions();
    }
  }

  private dismissSuggestions(): void {
    this.searchSubject.next("");
    this.searchService.resetSuggestions();
  }

  private submitSearchSongs(page = 1): void {
    this.searchService
      .searchSongs(this.searchTerm(), page)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe();
  }

  ngOnDestroy(): void {
    this.searchSubject.complete();
  }
}
