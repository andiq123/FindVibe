import {
  Component,
  computed,
  input,
  inject,
  ChangeDetectionStrategy,
  DestroyRef,
} from "@angular/core";
import { takeUntilDestroyed } from "@angular/core/rxjs-interop";
import {
  faHeartSolid as favoritedHeart,
  faHeartRegular as unFavoritedHeart,
} from "../icons";
import { UserService } from "../../features/library/services/user.service";
import { LibraryService } from "../../features/library/services/library.service";
import { Song } from "../../core/models/song.model";
import { FontAwesomeModule } from "@fortawesome/angular-fontawesome";
@Component({
  selector: "app-favorite-button",
  standalone: true,
  imports: [FontAwesomeModule],
  templateUrl: "./favorite-button.component.html",
  styleUrl: "./favorite-button.component.scss",
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class FavoriteButtonComponent {
  private userService = inject(UserService);
  private libraryService = inject(LibraryService);
  private destroyRef = inject(DestroyRef);
  forPlayer = input<boolean>(false);
  song = input.required<Song>();
  unFavoritedHeart = unFavoritedHeart;
  favoritedHeart = favoritedHeart;
  isLoadingFavorite = computed(() =>
    this.libraryService
      .currentLoadingFavoriteSongIds()
      .includes(this.song().id),
  );
  isAbleToAddToFav = computed(() => !!this.userService.user());
  isFavorited = computed(() =>
    this.libraryService
      .songs()
      .some((song: Song) => song.link === this.song().link),
  );
  onToggle(event: Event) {
    event.stopPropagation();
    event.preventDefault();
    this.toggleAddToFavorite();
  }
  toggleAddToFavorite() {
    const user = this.userService.user();
    if (!user) return;
    if (this.isFavorited()) {
      this.libraryService
        .removeFromFavorites(this.song().id, this.song().link)
        .pipe(takeUntilDestroyed(this.destroyRef))
        .subscribe();
    } else {
      this.libraryService.addToFavorites(this.song(), user.id)
        .pipe(takeUntilDestroyed(this.destroyRef))
        .subscribe();
    }
  }
}
