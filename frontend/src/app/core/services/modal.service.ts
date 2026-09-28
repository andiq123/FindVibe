import { Injectable, signal, Type } from "@angular/core";
@Injectable({
  providedIn: "root",
})
export class ModalService {
  activeModal = signal<Type<unknown> | null>(null);
  modalData = signal<unknown>(null);
  open<T>(component: Type<T>, data?: unknown) {
    this.modalData.set(data);
    this.activeModal.set(component);
  }
  close() {
    this.activeModal.set(null);
    this.modalData.set(null);
  }
  isOpen() {
    return this.activeModal() !== null;
  }
}
