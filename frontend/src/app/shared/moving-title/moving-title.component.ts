import {
  Component,
  input,
  signal,
  ElementRef,
  viewChild,
  AfterViewInit,
  OnDestroy,
  effect,
  ChangeDetectionStrategy,
} from '@angular/core';
@Component({
  selector: 'app-moving-title',
  standalone: true,
  imports: [],
  templateUrl: './moving-title.component.html',
  styleUrl: './moving-title.component.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class MovingTitleComponent implements AfterViewInit, OnDestroy {
  title = input.required<string>();
  classes = input<string>('text-md font-bold');
  isActive = input<boolean>(true);
  isBold = input<boolean>(false);
  fontSize = input<string>('1rem');
  color = input<string>('inherit');
  container = viewChild<ElementRef<HTMLDivElement>>('container');
  content = viewChild<ElementRef<HTMLDivElement>>('content');
  isOverflowing = signal<boolean>(false);
  animationDuration = signal<number>(10);
  private resizeObserver?: ResizeObserver;
  private rafId: number | null = null;
  constructor() {
    effect((onCleanup) => {
      this.title();
      if (this.rafId !== null) {
        cancelAnimationFrame(this.rafId);
      }
      this.rafId = requestAnimationFrame(() => {
        this.rafId = null;
        this.checkOverflow();
      });
      onCleanup(() => {
        if (this.rafId !== null) {
          cancelAnimationFrame(this.rafId);
          this.rafId = null;
        }
      });
    });
  }
  ngAfterViewInit() {
    this.checkOverflow();
    const containerEl = this.container()?.nativeElement;
    if (containerEl && typeof ResizeObserver !== 'undefined') {
      this.resizeObserver = new ResizeObserver(() => this.checkOverflow());
      this.resizeObserver.observe(containerEl);
    }
  }
  ngOnDestroy() {
    if (this.rafId !== null) {
      cancelAnimationFrame(this.rafId);
      this.rafId = null;
    }
    this.resizeObserver?.disconnect();
  }
  private checkOverflow() {
    const containerEl = this.container()?.nativeElement;
    const contentEl = this.content()?.nativeElement;
    if (containerEl && contentEl) {
      const hasOverflow = contentEl.scrollWidth > containerEl.clientWidth;
      if (this.isOverflowing() !== hasOverflow) {
        this.isOverflowing.set(hasOverflow);
        const scrollDistance = contentEl.scrollWidth / 2;
        const duration = Math.max(12, scrollDistance / 18);
        this.animationDuration.set(duration);
      }
    }
  }
}
