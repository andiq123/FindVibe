import { Component, ChangeDetectionStrategy } from '@angular/core';
@Component({
  selector: 'app-page-layout',
  template: `
    <div class="w-full max-w-3xl lg:max-w-4xl xl:max-w-5xl mx-auto min-h-[calc(100dvh-var(--safe-top))] relative overflow-x-clip">
      <div
        class="px-4 sm:px-5 lg:px-8 xl:px-10 pt-3 lg:pt-8"
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
