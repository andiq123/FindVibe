import {
  Component,
  inject,
  computed,
  signal,
  effect,
  ChangeDetectionStrategy,
  viewChild,
  ElementRef,
  Renderer2,
} from "@angular/core";
import { CommonModule, DOCUMENT } from "@angular/common";
import { ModalService } from "../../../core/services/modal.service";
import { FontAwesomeModule } from "@fortawesome/angular-fontawesome";
import { SwipeDownDirective } from "../../../features/player/directives/swipe-down.directive";
@Component({
  selector: "app-global-modal",
  standalone: true,
  imports: [CommonModule, FontAwesomeModule, SwipeDownDirective],
  template: `
    @if (activeComponent()) {
      <div
        class="fixed inset-0 z-[199] w-full h-full bg-base-100/60 backdrop-blur-md"
        [class.anim-fade-in]="isOpening()"
        [class.anim-fade-out]="isClosing()"
        (click)="close()"
        role="presentation"
      ></div>
      <div
        class="fixed bottom-0 left-0 right-0 bg-base-200/80 rounded-t-[2.5rem] z-[200] max-h-[92vh] flex flex-col border-t border-black/40 backdrop-blur-[24px]"
        style="padding-bottom: max(1rem, var(--safe-bottom)); padding-left: var(--safe-left); padding-right: var(--safe-right);"
        [class.anim-slide-up]="isOpening()"
        [class.anim-slide-down]="isClosing()"
        #modalContainer
        (animationend)="onAnimationEnd()"
        role="dialog"
        aria-modal="true"
        appSwipeDown
        [handleSelector]="'.modal-handle-bar, .modal-content'"
        (closePanel)="close()"
      >
        <div
          class="modal-handle-bar w-full h-8 flex justify-center items-center cursor-pointer shrink-0 group"
          (click)="close()"
          (keydown.enter)="close()"
          (keydown.space)="close(); $event.preventDefault()"
          tabindex="0"
          role="button"
          aria-label="Close modal"
        >
          <div
            class="w-9 h-[5px] bg-white/20 rounded-full transition-colors group-hover:bg-white/30"
          ></div>
        </div>
        <div
          class="flex-1 overflow-y-auto px-5 pb-5 overscroll-contain"
          data-swipe-scroll
        >
          <ng-container
            *ngComponentOutlet="activeComponent(); inputs: componentInputs()"
          />
        </div>
      </div>
    }
  `,
  styles: [
    `
      :host {
        display: contents;
      }
    `,
  ],
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class GlobalModalComponent {
  private modalService = inject(ModalService);
  private renderer = inject(Renderer2);
  private document = inject(DOCUMENT);
  activeComponent = this.modalService.activeModal;
  isClosing = signal(false);
  isOpening = signal(false);
  componentInputs = computed(() => {
    const data = this.modalService.modalData();
    return data ? { data } : {};
  });
  modalContainer = viewChild<ElementRef>("modalContainer");
  constructor() {
    effect(() => {
      const isOpen = this.activeComponent() !== null;
      if (isOpen) {
        this.isOpening.set(true);
        this.renderer.setStyle(this.document.body, "overflow", "hidden");
      } else {
        this.renderer.removeStyle(this.document.body, "overflow");
      }
    });
  }
  onAnimationEnd() {
    if (this.isOpening()) {
      this.isOpening.set(false);
    }
    if (this.isClosing()) {
      this.modalService.close();
      this.isClosing.set(false);
    }
  }
  close() {
    this.isClosing.set(true);
  }
}
