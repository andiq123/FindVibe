import {
  Directive,
  ElementRef,
  OnDestroy,
  OnInit,
  NgZone,
  inject,
  input,
  output,
} from "@angular/core";
import { decideSheetSnap } from "./sheet-drag.snap";

/** Keep in sync with `--anim-duration-normal` (+ tiny paint buffer). */
export const SHEET_ANIM_MS = 320;

/**
 * Bottom-sheet drag via translateY (fixed height): mid ↔ expanded, swipe down to dismiss.
 * Grabber-only so list/lyrics panes keep scrolling.
 */
@Directive({
  selector: "[appSheetDrag]",
  standalone: true,
  exportAs: "sheetDrag",
})
export class SheetDragDirective implements OnInit, OnDestroy {
  private readonly el = inject(ElementRef<HTMLElement>);
  private readonly zone = inject(NgZone);

  handleSelector = input(".up-next-handle");
  midRatio = input(0.5);
  maxRatio = input(0.92);
  dismiss = output<void>();

  private startY = 0;
  private startTy = 0;
  private dragging = false;
  private expanded = false;
  private dismissMode = false;
  private raf: number | null = null;
  private pendingY = 0;
  private sheetH = 0;
  private exitPrepared = false;
  private enterHandler: ((e: AnimationEvent) => void) | null = null;

  private readonly onStart = (e: TouchEvent) => this.handleStart(e);
  private readonly onMove = (e: TouchEvent) => this.handleMove(e);
  private readonly onEnd = () => this.handleEnd();

  ngOnInit(): void {
    const node = this.el.nativeElement;
    this.enterHandler = (e: AnimationEvent) => {
      if (!(e.animationName ?? "").includes("up-next-sheet-in")) return;
      if (this.enterHandler) {
        node.removeEventListener("animationend", this.enterHandler);
        this.enterHandler = null;
      }
      this.settleReady(node);
    };
    node.addEventListener("animationend", this.enterHandler);
    this.zone.runOutsideAngular(() => {
      node.addEventListener("touchstart", this.onStart, { passive: true });
      document.addEventListener("touchmove", this.onMove, { passive: false });
      document.addEventListener("touchend", this.onEnd, { passive: true });
      document.addEventListener("touchcancel", this.onEnd, { passive: true });
    });
  }

  ngOnDestroy(): void {
    this.cancelRaf();
    const node = this.el.nativeElement;
    if (this.enterHandler) {
      node.removeEventListener("animationend", this.enterHandler);
      this.enterHandler = null;
    }
    node.removeEventListener("touchstart", this.onStart);
    document.removeEventListener("touchmove", this.onMove);
    document.removeEventListener("touchend", this.onEnd);
    document.removeEventListener("touchcancel", this.onEnd);
  }

  /** Handle tap: expanded → mid; mid → dismiss. */
  collapseOrDismiss(): void {
    if (this.exitPrepared) return;
    if (this.expanded) {
      this.snapToMid();
      return;
    }
    this.zone.run(() => this.dismiss.emit());
  }

  /**
   * Snapshot current translateY and clear inline locks so `.is-out` can run
   * from the real position (mid or expanded), not a mid keyframe jump.
   */
  prepareExit(): void {
    if (this.exitPrepared) return;
    this.exitPrepared = true;
    this.cancelRaf();
    this.dragging = false;
    const node = this.el.nativeElement;
    const ty = Math.max(0, Math.round(this.readTy(node)));
    node.style.setProperty("--sheet-ty", `${ty}px`);
    // Class-only animation lock — never leave inline animation:none (blocks exit).
    node.style.removeProperty("animation");
    node.style.removeProperty("transition");
    node.style.removeProperty("height");
    node.style.transform = `translate3d(0, ${ty}px, 0)`;
    node.classList.remove("is-ready", "sheet-expanded");
  }

  private maxPx(): number {
    const cap = window.innerHeight - 32;
    return Math.round(Math.min(window.innerHeight * this.maxRatio(), cap));
  }

  private midTy(): number {
    const max = this.sheetH || this.maxPx();
    const mid = Math.round(window.innerHeight * this.midRatio());
    return Math.max(0, max - mid);
  }

  private readTy(node: HTMLElement): number {
    const t = getComputedStyle(node).transform;
    if (!t || t === "none") return this.midTy();
    const m = t.match(/matrix\(([^)]+)\)/);
    if (!m) return this.midTy();
    const parts = m[1].split(",").map((v) => Number(v.trim()));
    return Number.isFinite(parts[5]) ? parts[5] : this.midTy();
  }

  private settleReady(node: HTMLElement): void {
    if (this.exitPrepared) return;
    // Class lock only — inline animation:none was killing `.is-out`.
    node.classList.add("is-ready");
    this.sheetH = node.getBoundingClientRect().height || this.maxPx();
    if (!node.style.transform) {
      node.style.transform = `translate3d(0, ${this.midTy()}px, 0)`;
    }
  }

  private handleStart(e: TouchEvent): void {
    if (this.dragging || this.exitPrepared || !e.touches[0]) return;
    const target = e.target as HTMLElement | null;
    if (!target) return;
    const sel = this.handleSelector();
    if (sel && !target.closest(sel)) return;
    const btn = target.closest("button");
    if (btn && !btn.querySelector(".up-next-handle-bar")) return;

    const node = this.el.nativeElement;
    this.dragging = true;
    this.dismissMode = false;
    this.startY = e.touches[0].clientY;
    this.sheetH = node.getBoundingClientRect().height || this.maxPx();
    this.startTy = this.readTy(node);
    this.pendingY = 0;
    this.settleReady(node);
    node.style.setProperty("transition", "none", "important");
    node.style.height = `${this.sheetH}px`;
    node.style.transform = `translate3d(0, ${this.startTy}px, 0)`;
  }

  private handleMove(e: TouchEvent): void {
    if (!this.dragging || !e.touches[0]) return;
    const dy = e.touches[0].clientY - this.startY;
    if (Math.abs(dy) > 4 && e.cancelable) e.preventDefault();
    this.pendingY = dy;
    if (this.raf == null) {
      this.raf = requestAnimationFrame(() => this.paint());
    }
  }

  private paint(): void {
    this.raf = null;
    if (!this.dragging) return;
    const dy = this.pendingY;
    const mid = this.midTy();
    const node = this.el.nativeElement;
    let ty = this.startTy + dy;
    if (ty < 0) ty = ty * 0.35;
    if (ty > mid + 8) this.dismissMode = true;
    else if (ty < mid - 8) this.dismissMode = false;
    node.style.transform = `translate3d(0, ${ty}px, 0)`;
  }

  private handleEnd(): void {
    if (!this.dragging) return;
    this.cancelRaf();
    this.dragging = false;
    if (this.exitPrepared) return;
    const dy = this.pendingY;
    const mid = this.midTy();
    const node = this.el.nativeElement;
    const ty = this.readTy(node);
    const snap = decideSheetSnap({
      expanded: this.expanded,
      dismissMode: this.dismissMode,
      dy,
      ty,
      midTy: mid,
      vh: window.innerHeight,
    });
    if (snap === "dismiss") {
      this.zone.run(() => this.dismiss.emit());
      return;
    }
    if (snap === "expanded") this.snapToExpanded();
    else this.snapToMid();
  }

  private snapToMid(): void {
    this.expanded = false;
    this.dismissMode = false;
    const node = this.el.nativeElement;
    node.classList.remove("sheet-expanded");
    this.animateTo(this.midTy());
  }

  private snapToExpanded(): void {
    this.expanded = true;
    this.dismissMode = false;
    const node = this.el.nativeElement;
    node.classList.add("sheet-expanded");
    this.animateTo(0);
  }

  private animateTo(ty: number): void {
    if (this.exitPrepared) return;
    const node = this.el.nativeElement;
    this.settleReady(node);
    node.style.setProperty(
      "transition",
      "transform 0.38s cubic-bezier(0.32, 0.72, 0, 1)",
      "important",
    );
    node.style.transform = `translate3d(0, ${ty}px, 0)`;
    const clear = (e: TransitionEvent) => {
      if (e.propertyName !== "transform") return;
      node.removeEventListener("transitionend", clear);
      if (!this.dragging && !this.exitPrepared) {
        node.style.removeProperty("transition");
      }
    };
    node.addEventListener("transitionend", clear);
  }

  private cancelRaf(): void {
    if (this.raf != null) {
      cancelAnimationFrame(this.raf);
      this.raf = null;
    }
  }
}
