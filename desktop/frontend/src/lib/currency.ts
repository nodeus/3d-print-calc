import { writable } from 'svelte/store';

// Базовая валюта для подстановки {C} в подписях (i18n.ts).
export const SYMBOLS: Record<string, string> = {
  RUB: '₽',
  USD: '$',
  EUR: '€',
  KZT: '₸',
  BYN: 'Br',
};

export const baseSym = writable<string>('₽');

export function setBaseSym(code: string): void {
  baseSym.set(SYMBOLS[code] ?? code);
}
