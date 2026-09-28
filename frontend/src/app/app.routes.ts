import { Routes } from "@angular/router";
import { offlineGuard } from "./shared/guards/offline.guard";
export const routes: Routes = [
  {
    path: "",
    redirectTo: "explore",
    pathMatch: "full",
  },
  {
    path: "explore",
    loadComponent: () =>
      import("./features/explore/explore-page.component").then(
        (c) => c.ExplorePageComponent,
      ),
    canActivate: [offlineGuard],
    data: { animation: 1 },
  },
  {
    path: "songs",
    loadComponent: () =>
      import("./features/search/search-page.component").then(
        (c) => c.SearchPageComponent,
      ),
    canActivate: [offlineGuard],
    data: { animation: 1 },
  },
  {
    path: "songs/:query",
    loadComponent: () =>
      import("./features/search/search-page.component").then(
        (c) => c.SearchPageComponent,
      ),
    canActivate: [offlineGuard],
    data: { animation: 1 },
  },
  {
    path: "library",
    loadComponent: () =>
      import("./features/library/library.component").then(
        (c) => c.LibraryComponent,
      ),
    data: { animation: 2 },
  },
  {
    path: "recent",
    loadComponent: () =>
      import("./features/recent/recent.component").then(
        (c) => c.RecentComponent,
      ),
    data: { animation: 3 },
  },
  {
    path: "settings",
    loadComponent: () =>
      import("./features/settings/settings-page.component").then(
        (c) => c.SettingsPageComponent,
      ),
    data: { animation: 4 },
  },
  {
    path: "status",
    redirectTo: "settings",
    pathMatch: "full",
  },
];
