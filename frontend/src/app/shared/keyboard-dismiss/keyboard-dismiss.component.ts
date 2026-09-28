import {
  Component,
  ChangeDetectionStrategy,
  DestroyRef,
  inject,
  signal,
  OnInit,
} from "@angular/core";

/**
 * Floating Done control while a text field is focused.
 * Hides the keyboard / blurs without submitting the form (iOS parity).
 */
@Component({
  selector: "app-keyboard-dismiss",
  standalone: true,
  template: `
    @if (visible()) {
      <div
        class="keyboard-dismiss"
        [style.bottom.px]="bottomPx()"
        role="toolbar"
        aria-label="Keyboard"
      >
        <button
          type="button"
          class="keyboard-dismiss-btn"
          (mousedown)="$event.preventDefault()"
          (click)="dismiss()"
        >
          Done
        </button>
      </div>
    }
  `,
  styles: `
    .keyboard-dismiss {
      position: fixed;
      left: 0;
      right: 0;
      z-index: var(--z-keyboard, 90);
      display: flex;
      justify-content: flex-end;
      align-items: center;
      padding: 0.35rem 0.85rem
        calc(0.35rem + env(safe-area-inset-bottom, 0px));
      background: color-mix(in oklab, var(--color-base-200) 92%, transparent);
      border-top: 1px solid
        color-mix(in oklab, var(--color-base-content) 10%, transparent);
      backdrop-filter: blur(12px);
      -webkit-backdrop-filter: blur(12px);
      animation: keyboard-dismiss-in 180ms var(--anim-ease-ios, ease) both;
      pointer-events: auto;
    }

    .keyboard-dismiss-btn {
      appearance: none;
      border: 0;
      background: transparent;
      color: var(--color-primary);
      font-size: 0.95rem;
      font-weight: 650;
      line-height: 1;
      padding: 0.55rem 0.35rem;
      min-height: 44px;
      min-width: 44px;
      touch-action: manipulation;
      -webkit-tap-highlight-color: transparent;
    }

    .keyboard-dismiss-btn:active {
      opacity: 0.65;
    }

    @keyframes keyboard-dismiss-in {
      from {
        opacity: 0;
        transform: translateY(6px);
      }
      to {
        opacity: 1;
        transform: translateY(0);
      }
    }

    @media (prefers-reduced-motion: reduce) {
      .keyboard-dismiss {
        animation: none;
      }
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class KeyboardDismissComponent implements OnInit {
  private readonly destroyRef = inject(DestroyRef);
  readonly visible = signal(false);
  readonly bottomPx = signal(0);

  private blurTimer: ReturnType<typeof setTimeout> | null = null;

  ngOnInit(): void {
    const onFocusIn = (event: FocusEvent) => this.handleFocusIn(event);
    const onFocusOut = () => this.handleFocusOut();
    const onViewport = () => this.syncViewport();

    document.addEventListener("focusin", onFocusIn);
    document.addEventListener("focusout", onFocusOut);
    window.visualViewport?.addEventListener("resize", onViewport);
    window.visualViewport?.addEventListener("scroll", onViewport);
    window.addEventListener("resize", onViewport);

    this.destroyRef.onDestroy(() => {
      document.removeEventListener("focusin", onFocusIn);
      document.removeEventListener("focusout", onFocusOut);
      window.visualViewport?.removeEventListener("resize", onViewport);
      window.visualViewport?.removeEventListener("scroll", onViewport);
      window.removeEventListener("resize", onViewport);
      if (this.blurTimer != null) clearTimeout(this.blurTimer);
    });
  }

  dismiss(): void {
    const active = document.activeElement;
    if (active instanceof HTMLElement) active.blur();
    this.visible.set(false);
    this.bottomPx.set(0);
  }

  private handleFocusIn(event: FocusEvent): void {
    if (!isTextInput(event.target)) return;
    if (this.blurTimer != null) {
      clearTimeout(this.blurTimer);
      this.blurTimer = null;
    }
    this.syncViewport();
    if (this.shouldShow()) this.visible.set(true);
  }

  private handleFocusOut(): void {
    // Let Done's mousedown+click run before we hide on blur.
    this.blurTimer = setTimeout(() => {
      this.blurTimer = null;
      if (!isTextInput(document.activeElement)) {
        this.visible.set(false);
        this.bottomPx.set(0);
      }
    }, 0);
  }

  private syncViewport(): void {
    const vv = window.visualViewport;
    if (!vv) {
      this.bottomPx.set(0);
      return;
    }
    const inset = Math.max(0, window.innerHeight - vv.height - vv.offsetTop);
    this.bottomPx.set(inset);
    if (isTextInput(document.activeElement)) {
      this.visible.set(this.shouldShow());
    }
  }

  /** Coarse pointer (phones) or a real keyboard inset — skip desktop mouse focus chrome. */
  private shouldShow(): boolean {
    const coarse =
      typeof matchMedia !== "undefined" &&
      matchMedia("(pointer: coarse)").matches;
    return coarse || this.bottomPx() > 40;
  }
}

function isTextInput(el: EventTarget | null): boolean {
  if (!(el instanceof HTMLElement)) return false;
  if (el.isContentEditable) return true;
  if (el instanceof HTMLTextAreaElement) return true;
  if (!(el instanceof HTMLInputElement)) return false;
  const type = (el.type || "text").toLowerCase();
  return (
    type === "text" ||
    type === "search" ||
    type === "url" ||
    type === "email" ||
    type === "tel" ||
    type === "password" ||
    type === "number"
  );
}
