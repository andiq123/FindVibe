import { Component, inject, ChangeDetectionStrategy } from "@angular/core";
import { StorageService } from "../../core/services/storage.service";
import { PlaylistService } from "../../core/services/playlist.service";
import { PageContentComponent } from "../../shared/components/page-content/page-content.component";
import { SongListComponent } from "../../shared/components/song-list/song-list.component";
import { PlayerService } from "../../core/services/player.service";

@Component({
  selector: "app-recent",
  standalone: true,
  imports: [PageContentComponent, SongListComponent],
  templateUrl: "./recent.component.html",
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class RecentComponent {
  private storageService = inject(StorageService);
  private playlistService = inject(PlaylistService);
  private playerService = inject(PlayerService);
  songs = this.storageService.recentSongs;

  onChangePlaylist() {
    this.playlistService.setCurrentPlaylist(this.songs());
  }

  playAll(): void {
    const list = this.songs();
    if (!list.length) return;
    void this.playerService.playFromList(list, list[0]);
  }

  clearHistory(): void {
    this.storageService.clearRecents();
  }
}
