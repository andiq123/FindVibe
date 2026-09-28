import { ChangeDetectionStrategy, Component, input } from "@angular/core";
import { PlayerStatus } from "../../features/player/models/player.model";
import { faPause, faPlay } from "../icons";
import { FontAwesomeModule } from "@fortawesome/angular-fontawesome";
@Component({
  selector: "app-player-button",
  standalone: true,
  imports: [FontAwesomeModule],
  templateUrl: "./player-button.component.html",
  styleUrl: "./player-button.component.scss",
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class PlayerButtonComponent {
  status = input.required<PlayerStatus>();
  playerStatus = PlayerStatus;
  faPlay = faPlay;
  faPause = faPause;
}
