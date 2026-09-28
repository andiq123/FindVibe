import { inject } from '@angular/core';
import { CanActivateFn, Router } from '@angular/router';
import { SettingsService } from '../../core/services/settings.service';
export const offlineGuard: CanActivateFn = () => {
  const router = inject(Router);
  const settings = inject(SettingsService);
  if (!navigator.onLine || settings.isServerDown()) {
    router.navigate(['/library']);
    return false;
  }
  return true;
};
