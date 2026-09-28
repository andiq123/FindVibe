import { Injectable, signal } from "@angular/core";

/** Tiny app-wide flash toast (skip feedback, queue confirms, radio refill). */
@Injectable({ providedIn: "root" })
export class ToastService {
  readonly message = signal("");
  /** Sticky spinner toast — cleared by `show` / `clear`. */
  readonly loading = signal(false);
  private timer: ReturnType<typeof setTimeout> | null = null;

  show(msg: string, ms = 2400): void {
    if (!msg) return;
    if (this.timer != null) clearTimeout(this.timer);
    this.loading.set(false);
    this.message.set(msg);
    this.timer = setTimeout(() => {
      this.timer = null;
      this.message.set("");
    }, ms);
  }

  /** Sticky message with spinner until replaced or cleared. */
  hold(msg: string): void {
    if (!msg) return;
    if (this.timer != null) clearTimeout(this.timer);
    this.timer = null;
    this.loading.set(true);
    this.message.set(msg);
  }

  clear(): void {
    if (this.timer != null) clearTimeout(this.timer);
    this.timer = null;
    this.loading.set(false);
    this.message.set("");
  }
}
