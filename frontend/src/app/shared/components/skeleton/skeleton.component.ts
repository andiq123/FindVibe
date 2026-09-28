import { Component, input, ChangeDetectionStrategy } from "@angular/core";
@Component({
  selector: "app-skeleton",
  standalone: true,
  template: `
    @switch (type()) {
      @case ("song") {
        @for (item of items; track $index) {
          <div class="flex items-center gap-4 py-3 px-5 animate-pulse motion-reduce:animate-none motion-reduce:opacity-50">
            <div class="w-12 h-12 rounded-lg bg-white/[0.03] shrink-0"></div>
            <div class="flex-1 space-y-2">
              <div
                class="h-4 bg-white/[0.03] rounded"
                [style.width]="randomWidth()"
              ></div>
              <div class="h-3 bg-white/[0.02] rounded w-24"></div>
            </div>
          </div>
        }
      }
      @case ("text") {
        <div
          class="h-4 bg-white/[0.03] rounded animate-pulse motion-reduce:animate-none motion-reduce:opacity-50"
          [style.width]="width()"
        ></div>
      }
      @case ("circle") {
        <div
          class="rounded-full bg-white/[0.03] animate-pulse motion-reduce:animate-none motion-reduce:opacity-50 shrink-0"
          [style.width]="size()"
          [style.height]="size()"
        ></div>
      }
      @case ("card") {
        @for (item of items; track $index) {
          <div class="glass-light rounded-2xl p-4 animate-pulse motion-reduce:animate-none motion-reduce:opacity-50 space-y-3">
            <div class="w-full aspect-square bg-white/[0.03] rounded-lg"></div>
            <div class="h-4 bg-white/[0.03] rounded w-3/4"></div>
            <div class="h-3 bg-white/[0.02] rounded w-1/2"></div>
          </div>
        }
      }
      @case ("rect") {
        <div
          class="bg-white/[0.03] rounded animate-pulse motion-reduce:animate-none motion-reduce:opacity-50"
          [style.width]="width()"
          [style.height]="height()"
        ></div>
      }
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class SkeletonComponent {
  type = input<"song" | "text" | "circle" | "card" | "rect">("song");
  count = input<number>(3);
  width = input<string>("100%");
  height = input<string>("auto");
  size = input<string>("48px");
  get items() {
    return Array(this.count()).fill(0);
  }
  randomWidth(): string {
    const widths = ["60%", "70%", "80%", "90%"];
    return widths[Math.floor(Math.random() * widths.length)];
  }
}
