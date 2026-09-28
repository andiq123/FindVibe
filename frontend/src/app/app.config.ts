import {
  ApplicationConfig,
  inject,
  isDevMode,
  provideZonelessChangeDetection,
} from "@angular/core";
import { DOCUMENT } from "@angular/common";
import {
  provideRouter,
  withComponentInputBinding,
  withRouterConfig,
  withViewTransitions,
  RouteReuseStrategy,
  withInMemoryScrolling,
  Router,
} from "@angular/router";
import { routes } from "./app.routes";
import {
  provideHttpClient,
  withFetch,
  withInterceptors,
} from "@angular/common/http";
import { apiInterceptor } from "./core/interceptors/api.interceptor";
import { provideServiceWorker } from "@angular/service-worker";
import { CustomReuseStrategy } from "./core/strategies/custom-reuse-strategy";
import type { ActivatedRouteSnapshot } from "@angular/router";

function routeOrder(path: string): number {
  if (!path) return 0;
  if (path.startsWith("songs")) return 1;
  if (path === "library" || path === "recent") return 2;
  if (path === "settings") return 3;
  return 0;
}

function getSegmentPath(route: ActivatedRouteSnapshot | undefined): string {
  if (!route) return "";
  let r: ActivatedRouteSnapshot | null = route;
  while (r) {
    const seg = r.url?.[0]?.path;
    if (seg) return seg;
    const p = r.routeConfig?.path;
    if (p && p !== "") return p.startsWith("songs") ? "songs" : p;
    r = r.firstChild;
  }
  return "";
}

export const appConfig: ApplicationConfig = {
  providers: [
    provideZonelessChangeDetection(),
    provideRouter(
      routes,
      withComponentInputBinding(),
      withRouterConfig({
        paramsInheritanceStrategy: "always",
      }),
      withInMemoryScrolling({
        scrollPositionRestoration: "enabled",
        anchorScrolling: "enabled",
      }),
      withViewTransitions({
        skipInitialTransition: true,
        onViewTransitionCreated: ({ transition, to }) => {
          const doc = inject(DOCUMENT);
          const router = inject(Router);
          const fromPath =
            router.url.split("?")[0].split("/").filter(Boolean)[0] ?? "songs";
          const toPath = getSegmentPath(to) || "songs";
          // ponytail: cancel/search query changes stay on Explore — no page slide
          if (fromPath === toPath) {
            transition.skipTransition();
            return;
          }
          const isBack = routeOrder(toPath) < routeOrder(fromPath);
          let styleEl: HTMLStyleElement | null = null;
          if (isBack) {
            styleEl = doc.createElement("style");
            styleEl.setAttribute("data-page-transition", "back");
            styleEl.textContent = `@keyframes page-slide-out-back{to{transform:translateX(100%);opacity:0}}@keyframes page-slide-in-back{from{transform:translateX(-100%);opacity:0.95}to{transform:translateX(0);opacity:1}}@keyframes page-slide-out-back-desktop{to{transform:translateY(100%);opacity:0.9}}@keyframes page-slide-in-back-desktop{from{transform:translateY(-100%);opacity:0.9}to{transform:translateY(0);opacity:1}}::view-transition-old(page){animation:page-slide-out-back 0.28s cubic-bezier(0.32,0.72,0,1) forwards!important}::view-transition-new(page){animation:page-slide-in-back 0.28s cubic-bezier(0.32,0.72,0,1) forwards!important}@media(min-width:1024px){::view-transition-old(page){animation:page-slide-out-back-desktop 0.24s cubic-bezier(0.32,0.72,0,1) forwards!important}::view-transition-new(page){animation:page-slide-in-back-desktop 0.24s cubic-bezier(0.32,0.72,0,1) forwards!important}}`;
            doc.head.appendChild(styleEl);
          }
          transition.finished.finally(() => {
            styleEl?.remove();
          });
        },
      }),
    ),
    { provide: RouteReuseStrategy, useClass: CustomReuseStrategy },
    provideHttpClient(withFetch(), withInterceptors([apiInterceptor])),
    provideServiceWorker("ngsw-worker.js", {
      enabled: !isDevMode(),
      registrationStrategy: "registerWhenStable:30000",
    }),
  ],
};
