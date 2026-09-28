import { Component, ChangeDetectionStrategy } from "@angular/core";
import { FaIconComponent } from "@fortawesome/angular-fontawesome";
import {
  faCompass,
  faMagnifyingGlass,
  faHeart,
  faGear,
} from "../../shared/icons";
import { RouterLink, RouterLinkActive } from "@angular/router";

@Component({
  selector: "app-navigation",
  standalone: true,
  imports: [FaIconComponent, RouterLink, RouterLinkActive],
  templateUrl: "./navigation.component.html",
  styleUrl: "./navigation.component.scss",
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class NavigationComponent {
  /** Short labels — tab bar space is tight on mobile. */
  readonly navList = [
    {
      name: "Explore",
      icon: faCompass,
      link: "/explore",
    },
    { name: "Search", icon: faMagnifyingGlass, link: "/songs" },
    {
      name: "Vault",
      icon: faHeart,
      link: "/library",
    },
    {
      name: "Settings",
      icon: faGear,
      link: "/settings",
    },
  ];
}
