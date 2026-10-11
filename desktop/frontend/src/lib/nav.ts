import { writable } from 'svelte/store';

// Межвкладочная навигация:
// - openOrderId: заказ, который надо открыть на вкладке заказов (из долгов);
// - openMaterial: материал/катушка для вкладки материалов (из отчётов);
// - goTab: вкладка, на которую надо переключиться (потребляется App.svelte).
// - customerFilter: фильтр заказов по заказчику (из вкладки заказчиков, 0 — все).
export const openOrderId = writable<number | null>(null);
export const openMaterial = writable<{ code: string; spoolNo: number } | null>(null);
export const goTab = writable<string | null>(null);
export const customerFilter = writable<number>(0);

export function gotoMaterial(code: string, spoolNo: number): void {
  openMaterial.set({ code, spoolNo });
  goTab.set('materials');
}

export function gotoOrder(id: number): void {
  openOrderId.set(id);
  goTab.set('orders');
}

export function gotoCustomerOrders(customerId: number): void {
  customerFilter.set(customerId);
  openOrderId.set(null);
  goTab.set('orders');
}
