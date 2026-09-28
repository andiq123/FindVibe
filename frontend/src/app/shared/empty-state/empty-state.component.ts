import { Component, input, computed, ChangeDetectionStrategy } from "@angular/core";
import { IconDefinition } from "../icons";
import { FontAwesomeModule } from "@fortawesome/angular-fontawesome";
@Component({
  selector: "app-empty-state",
  standalone: true,
  imports: [FontAwesomeModule],
  template: `
    <div
      class="flex flex-col items-center justify-center py-24 px-8 text-center"
    >
      <div
        [class]="'w-20 h-20 rounded-2xl flex items-center justify-center mb-6 ring-1 ' + iconBgClass()"
      >
        <fa-icon [icon]="icon()" class="text-4xl" [class]="iconColorClass()" />
      </div>
      <h2 [class]="'text-2xl font-bold mb-3 tracking-tight ' + titleColorClass()">
        {{ title() }}
      </h2>
      <p class="text-base-content/50 text-base max-w-[280px] leading-relaxed">
        {{ description() }}
      </p>
    </div>
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class EmptyStateComponent {
  icon = input.required<IconDefinition>();
  title = input.required<string>();
  description = input.required<string>();
  variant = input<"default" | "error">("default");
  iconBgClass = computed(() =>
    this.variant() === "error"
      ? "bg-error/10 ring-error/20"
      : "bg-primary/10 ring-primary/15"
  );
  iconColorClass = computed(() =>
    this.variant() === "error" ? "text-error" : "text-primary/70"
  );
  titleColorClass = computed(() =>
    this.variant() === "error" ? "text-error" : ""
  );
}
