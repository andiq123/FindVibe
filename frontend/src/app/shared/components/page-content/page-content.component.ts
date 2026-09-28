import { Component, ChangeDetectionStrategy } from "@angular/core";
import { PageLayoutComponent } from "../page-layout/page-layout.component";

@Component({
  selector: "app-page-content",
  standalone: true,
  imports: [PageLayoutComponent],
  template: `
    <app-page-layout>
      <ng-content></ng-content>
    </app-page-layout>
  `,
  styles: `
    :host {
      display: block;
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class PageContentComponent {}
