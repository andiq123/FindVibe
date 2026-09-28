import {
  Component,
  inject,
  computed,
  ChangeDetectionStrategy,
} from "@angular/core";
import { SettingsService } from "../../core/services/settings.service";

@Component({
  selector: "app-connection-status",
  standalone: true,
  templateUrl: "./connection-status.component.html",
  styleUrl: "./connection-status.component.scss",
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ConnectionStatusComponent {
  private readonly settings = inject(SettingsService);

  /** Single status chip: waking / offline / unreachable. Hidden when healthy. */
  readonly label = computed(() => {
    if (this.settings.isNavigatorOffline()) return "You're offline";
    if (!this.settings.isCheckedServer()) return "Waking server…";
    if (this.settings.isServerDown()) return "Can't reach server";
    return null;
  });

  readonly waking = computed(
    () => !this.settings.isNavigatorOffline() && !this.settings.isCheckedServer(),
  );
}
