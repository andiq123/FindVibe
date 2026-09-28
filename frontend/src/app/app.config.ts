import {
  ApplicationConfig,
  isDevMode,
  provideZonelessChangeDetection,
} from "@angular/core";
import {
  provideRouter,
  withComponentInputBinding,
  withRouterConfig,
  RouteReuseStrategy,
  withInMemoryScrolling,
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

    ),
    { provide: RouteReuseStrategy, useClass: CustomReuseStrategy },
    provideHttpClient(withFetch(), withInterceptors([apiInterceptor])),
    provideServiceWorker("ngsw-worker.js", {
      enabled: !isDevMode(),
      registrationStrategy: "registerWhenStable:30000",
    }),
  ],
};
