import { writable, derived, get } from 'svelte/store';
import ru from './i18n/ru.json';
import en from './i18n/en.json';
import { wails } from './wails';
import { baseSym, setBaseSym } from './currency';

const dicts: Record<string, Record<string, string>> = { ru, en };

export const locale = writable<string>('ru');

// t — перевод ключа ({C} подставляется символом базовой валюты);
// при отсутствии ключа возвращает сам ключ.
export const t = derived([locale, baseSym], ([$locale, $sym]) => {
  const d = dicts[$locale] ?? ru;
  return (key: string): string => (d[key] ?? dicts['ru'][key] ?? key).replaceAll('{C}', $sym);
});

export async function initLocale(): Promise<void> {
  try {
    const saved = await wails.getLocale();
    locale.set(saved === 'en' ? 'en' : 'ru');
  } catch {
    locale.set('ru');
  }
  document.documentElement.lang = get(locale);
  try {
    setBaseSym(await wails.getBaseCurrency());
  } catch {
    /* noop */
  }
}

export async function switchLocale(value: string): Promise<void> {
  const v = value === 'en' ? 'en' : 'ru';
  locale.set(v);
  document.documentElement.lang = v;
  try {
    await wails.setLocale(v);
  } catch {
    /* офлайн первого запуска — просто не сохраняем */
  }
}
