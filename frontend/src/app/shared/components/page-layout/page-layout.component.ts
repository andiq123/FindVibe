import { Component, ChangeDetectionStrategy } from '@angular/core';
@Component({
  selector: 'app-page-layout',
  template: `
    <div class="w-full max-w-[1440px] mx-auto min-h-[calc(100dvh-var(--safe-top))] relative overflow-x-clip">
      <div
        class="px-4 sm:px-5 lg:px-6 pt-3 lg:pt-5"
        style="padding-bottom: calc(11.5rem + var(--safe-bottom));"
      >
        <ng-content></ng-content>
      </div>
    </div>
  `,
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class PageLayoutComponent {}
