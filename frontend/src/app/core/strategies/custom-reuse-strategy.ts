import {
  ActivatedRouteSnapshot,
  DetachedRouteHandle,
  RouteReuseStrategy,
} from "@angular/router";
import { Injectable } from "@angular/core";
interface StoredRoute {
  handle: DetachedRouteHandle;
  scrollPosition?: number;
}
@Injectable()
export class CustomReuseStrategy implements RouteReuseStrategy {
  private handlers = new Map<string, StoredRoute>();
  private routesToCache: string[] = [
    "library",
    "songs",
    "songs/:query",
    "recent",
  ];
  shouldDetach(route: ActivatedRouteSnapshot): boolean {
    const path = this.getRoutePath(route);
    return this.shouldCacheRoute(path);
  }
  store(
    route: ActivatedRouteSnapshot,
    handle: DetachedRouteHandle | null,
  ): void {
    if (!handle) return;
    const path = this.getRoutePath(route);
    const scrollPosition = window.scrollY;
    this.handlers.set(path, { handle, scrollPosition });
  }
  shouldAttach(route: ActivatedRouteSnapshot): boolean {
    const path = this.getRoutePath(route);
    return this.handlers.has(path);
  }
  retrieve(route: ActivatedRouteSnapshot): DetachedRouteHandle | null {
    const path = this.getRoutePath(route);
    const stored = this.handlers.get(path);
    if (stored) {
      requestAnimationFrame(() => {
        if (stored.scrollPosition !== undefined) {
          window.scrollTo({
            top: stored.scrollPosition,
            behavior: "instant",
          });
        }
      });
      return stored.handle;
    }
    return null;
  }
  shouldReuseRoute(
    future: ActivatedRouteSnapshot,
    curr: ActivatedRouteSnapshot,
  ): boolean {
    if (future.routeConfig === curr.routeConfig) return true;
    // /songs ↔ /songs/:query is the same screen (cancel/submit), not a tab change
    const a = future.routeConfig?.path ?? "";
    const b = curr.routeConfig?.path ?? "";
    return (
      (a === "songs" || a === "songs/:query") &&
      (b === "songs" || b === "songs/:query")
    );
  }
  private getRoutePath(route: ActivatedRouteSnapshot): string {
    if (!route.routeConfig?.path) return "";
    let path = route.routeConfig.path;
    if (route.params && Object.keys(route.params).length > 0) {
      Object.keys(route.params).forEach((key) => {
        path = path.replace(`:${key}`, route.params[key]);
      });
    }
    return path;
  }
  private shouldCacheRoute(path: string): boolean {
    return this.routesToCache.some((cachedPath) => {
      if (path === cachedPath) return true;
      if (cachedPath.includes(":")) {
        const base = cachedPath.split("/:")[0];
        return path.startsWith(base);
      }
      return false;
    });
  }
}
