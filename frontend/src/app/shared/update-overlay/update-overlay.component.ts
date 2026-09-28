import { Component, inject, ChangeDetectionStrategy } from '@angular/core';
import { AppUpdateService } from '../../core/services/app-update.service';
@Component({
  selector: 'app-update-overlay',
  standalone: true,
  imports: [],
  templateUrl: './update-overlay.component.html',
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class UpdateOverlayComponent {
  private readonly appUpdateService = inject(AppUpdateService);
  readonly secondsToUpdate = this.appUpdateService.secondsToUpdate;
  readonly updateLoading = this.appUpdateService.updateLoading;
  reloadNow(): void {
    this.appUpdateService.applyUpdate();
  }
}
