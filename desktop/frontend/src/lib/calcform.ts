import { writable } from 'svelte/store';

// Состояние формы калькулятора, переживающее переключение вкладок
// (страницы размонтируются из-за {#key tab} в App.svelte).
export interface CalcForm {
  matCode: string;
  spoolId: number;
  printerId: number;
  timeH: number | null;
  timeM: number | null;
  weightG: number | null;
  filamentMM: number | null;
  tariff: number | null;
  powerKW: number | null;
  amortPerHour: number | null;
  opRate: number | null;
  prepH: number | null;
  postH: number | null;
  designerRate: number | null;
  modelH: number | null;
  modelPrice: number | null;
  extraMat: number | null;
  failPct: number | null;
  marginProfitPct: number | null;
  marginDevPct: number | null;
  vatPct: number | null;
  roundStep: number | null;
  fees: number | null;
  maintPerHour: number | null;
  purgeG: number | null;
  filename: string;
  customerId: number;
  orderId: number;
  orderTitle: string;
  orderQty: number;
  historyId: number;
}

export const calcFormDefaults: CalcForm = {
  matCode: 'PETG-NIT-W4',
  spoolId: 0,
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

// Заводские значения + пользовательские (из файла настроек).
// Сброс «к заводским» возвращает calcFormDefaults, «к моим» — factoryWithUser.
let userDefaults: Partial<CalcForm> = {};

export function factoryValues(): CalcForm {
  return { ...calcFormDefaults, ...userDefaults };
}

// applyCalcDefaults кладёт дефолты пользователя поверх заводских
// (только известные ключи, типы — как в форме).
export function applyCalcDefaults(m: Record<string, unknown>): void {
  const next: Partial<CalcForm> = {};
  for (const k of Object.keys(calcFormDefaults) as (keyof CalcForm)[]) {
    if (k in m && m[k] != null) {
      (next as Record<string, unknown>)[k] = m[k];
    }
  }
  userDefaults = next;
}

// resetCalcForm возвращает форму к дефолтам (заводским или пользовательским).
export function resetCalcForm(): void {
  calcForm.set(factoryValues());
}

const LS_KEY = 'calcform.v1';

// Загрузка сохранённого (пережить перезапуск приложения).
try {
  const raw = localStorage.getItem(LS_KEY);
  if (raw) {
    const saved = { ...calcFormDefaults, ...JSON.parse(raw) };
    calcForm.set(saved);
  }
} catch {
  // битый localStorage — остаёмся на дефолтах
}

// Каждое изменение формы — сразу в localStorage.
calcForm.subscribe((v) => {
  try {
    localStorage.setItem(LS_KEY, JSON.stringify(v));
  } catch {
    // переполнен/запрещён — молча пропускаем
  }
});
