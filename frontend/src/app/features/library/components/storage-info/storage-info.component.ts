import {
  Component,
  computed,
  OnInit,
  signal,
  inject,
  ChangeDetectionStrategy,
} from "@angular/core";
import { DecimalPipe } from "@angular/common";
import { OfflineStorageService } from "../../services/offline-storage.service";
import { LibraryService } from "../../services/library.service";
import { FontAwesomeModule } from "@fortawesome/angular-fontawesome";
import { faTrash, faCloudArrowDown } from "../../../../shared/icons";

@Component({
  selector: "app-storage-info",
  standalone: true,
  imports: [FontAwesomeModule, DecimalPipe],
  templateUrl: "./storage-info.component.html",
  styleUrl: "./storage-info.component.scss",
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class StorageInfoComponent implements OnInit {
  private offlineStorageService = inject(OfflineStorageService);
  private libraryService = inject(LibraryService);
  storageTotal = this.offlineStorageService.storageTotal;
  storageUsed = this.offlineStorageService.storageUsed;
  offlineCount = this.offlineStorageService.offlineCount;
  libraryCount = computed(() => this.libraryService.songs().length);
  progressPercent = computed(() => {
    const total = this.libraryCount() || 1;
    return Math.min(100, (this.offlineCount() / total) * 100);
  });
  loadingDownloading = signal(false);
  loadingClearing = signal(false);
  lastResult = signal("");
  showRemoveCacheButton = computed(() => this.offlineCount() > 0);
  faTrash = faTrash;
  faCloudArrowDown = faCloudArrowDown;

  ngOnInit(): void {
    void this.offlineStorageService.setUpStorage();
    void this.offlineStorageService.syncOfflineSongs(this.libraryService.songs());
  }

  async downloadAll(): Promise<void> {
    try {
      this.loadingDownloading.set(true);
      this.lastResult.set("");
      const result = await this.offlineStorageService.cacheAllSongs(
        this.libraryService.songs(),
      );
      const parts: string[] = [];
      if (result.saved) parts.push(`${result.saved} saved`);
      if (result.skipped) parts.push(`${result.skipped} already offline`);
      if (result.failed) parts.push(`${result.failed} failed`);
      this.lastResult.set(parts.join(" · ") || "Nothing to download");
    } catch (error) {
      console.error("[StorageInfo] Failed to download songs:", error);
      this.lastResult.set("Download failed");
    } finally {
      this.loadingDownloading.set(false);
    }
  }

  async removeCache(): Promise<void> {
    try {
      this.loadingClearing.set(true);
      this.lastResult.set("");
      await this.offlineStorageService.removeCache();
      this.lastResult.set("Offline cache cleared");
    } catch (error) {
      console.error("[StorageInfo] Failed to clear cache:", error);
      this.lastResult.set("Clear failed");
    } finally {
      this.loadingClearing.set(false);
    }
  }
}
