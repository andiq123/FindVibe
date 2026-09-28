import { WritableSignal } from '@angular/core';
export function trackLoadingState(
  signal: WritableSignal<string[]>,
  id: string,
  isLoading: boolean
): void {
  signal.update((prevIds) =>
    isLoading
      ? prevIds.includes(id) ? prevIds : [...prevIds, id]
      : prevIds.filter((songId) => songId !== id)
  );
}
