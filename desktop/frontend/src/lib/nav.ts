import { writable } from 'svelte/store';

// Межвкладочная навигация:
// - openOrderId: заказ, который надо открыть на вкладке заказов (из долгов);
// - goTab: вкладка, на которую надо переключиться (потребляется App.svelte).
export const openOrderId = writable<number | null>(null);
export const goTab = writable<string | null>(null);

export function gotoOrder(id: number): void {
  openOrderId.set(id);
  goTab.set('orders');
}
