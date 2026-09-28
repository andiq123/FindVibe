import {
  HttpErrorResponse,
  HttpEventType,
  HttpInterceptorFn,
} from "@angular/common/http";
import { inject } from "@angular/core";
import { catchError, retry, tap, throwError, timeout, timer, TimeoutError } from "rxjs";
import { environment } from "../../../environments/environment";
import { SettingsService } from "../services/settings.service";

/** Render cold start: gateway 504s until the process listens, then 503 "starting" until DB mounts. */
function isTransient(err: unknown): boolean {
  if (err instanceof TimeoutError) return true;
  return (
    err instanceof HttpErrorResponse &&
    [0, 408, 429, 503, 504].includes(err.status)
  );
}

/**
 * Single HTTP policy for all API calls: timeout + transient GET retries.
 * /health is excluded — wakeUntilUp() owns its own retry loop.
 * /lyrics + /recommend: longer one-shot timeout, never retry TimeoutError
 * (retries turned one slow radio into ~minute waits). Still retry 503/504 cold starts.
 */
export const apiInterceptor: HttpInterceptorFn = (req, next) => {
  if (!req.url.startsWith(environment.API_URL)) return next(req);
  const settings = inject(SettingsService);
  const isHealth = req.url.includes("/health");
  const isSlow =
    req.url.includes("/lyrics") || req.url.includes("/recommend");
  // ponytail: mutations are not retried — a duplicated POST is worse than a failed one
  const retries = req.method === "GET" && !isHealth ? (isSlow ? 2 : 3) : 0;
  return next(req).pipe(
    timeout(isSlow ? 22_000 : 15_000),
    retry({
      count: retries,
      delay: (err, n) => {
        if (!isTransient(err)) throw err;
        // Don't triple-pay a slow LRCLIB / Last.fm resolve.
        if (isSlow && err instanceof TimeoutError) throw err;
        return timer(1000 * 2 ** (n - 1));
      },
    }),
    tap((event) => {
      if (event.type === HttpEventType.Response) settings.setServerUp();
    }),
    catchError((err) => {
      // Exhausted cold-start / network — surface the chip. 4xx stays quiet.
      if (!isHealth && isTransient(err)) settings.setServerDown();
      return throwError(() => err);
    }),
  );
};
