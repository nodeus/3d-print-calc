import { writable } from 'svelte/store';

export interface ConfirmRequest {
  title: string;
  resolve: (v: boolean) => void;
}

// Единственный активный запрос подтверждения (null — нет).
export const confirmState = writable<ConfirmRequest | null>(null);

// askConfirm показывает модалку App.svelte и резолвится кнопками.
// Не блокирует поток (в отличие от native confirm с wails.localhost).
export function askConfirm(title: string): Promise<boolean> {
  return new Promise((resolve) => confirmState.set({ title, resolve }));
}
