import { writable } from 'svelte/store';

// Состояние формы калькулятора, переживающее переключение вкладок
// (страницы размонтируются из-за {#key tab} в App.svelte).
export interface CalcForm {
  matCode: string;
  printerId: number;
  timeH: number;
  timeM: number;
  weightG: number | null;
  filamentMM: number | null;
  tariff: number;
  powerKW: number;
  amortPerHour: number;
  opRate: number;
  prepH: number;
  postH: number;
  designerRate: number;
  modelH: number;
  modelPrice: number;
  extraMat: number;
  failPct: number;
  marginProfitPct: number;
  marginDevPct: number;
  copies: number;
  vatPct: number;
  roundStep: number;
  fees: number;
  maintPerHour: number;
  purgeG: number;
  filename: string;
  customerId: number;
  orderId: number;
  orderTitle: string;
  orderQty: number;
  historyId: number;
}

export const calcFormDefaults: CalcForm = {
  matCode: 'PETG-NIT-W4',
  printerId: 0,
  timeH: 3,
  timeM: 54,
  weightG: 134.94,
  filamentMM: null,
  tariff: 7.08,
  powerKW: 0.35,
  amortPerHour: 5.41,
  opRate: 100,
  prepH: 0,
  postH: 0,
  designerRate: 600,
  modelH: 0,
  modelPrice: 0,
  extraMat: 50,
  failPct: 0,
  marginProfitPct: 80,
  marginDevPct: 35,
  copies: 1,
  vatPct: 0,
  roundStep: 0,
  fees: 100,
  maintPerHour: 0,
  purgeG: 0,
  filename: '',
  customerId: 0,
  orderId: 0,
  orderTitle: '',
  orderQty: 1,
  historyId: 0,
};

export const calcForm = writable<CalcForm>({ ...calcFormDefaults });
