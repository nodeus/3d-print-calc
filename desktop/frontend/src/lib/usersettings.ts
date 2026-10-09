import { writable, get } from 'svelte/store';
import { wails, type UserUISettings } from './wails';
import { locale } from './i18n';
import { applyTheme, THEME_PRESETS } from './theme';
import { applyCalcDefaults } from './calcform';

// Секция настроек текущего пользователя (user_settings.json).
// Пустые поля — фолбэк на legacy-БД, чтобы старые установки не теряли значения.
export const mySettings = writable<UserUISettings>({
  theme: '',
  locale: '',
  historyColumns: [],
  widths: {},
  colOrder: {},
  ordersPerPage: 0,
  historyPerPage: 0,
  calcDefaults: {},
});

async function legacyThemeVars(): Promise<Record<string, Record<string, string>>> {
  try {
    return (await wails.uiSettings()).custom ?? {};
  } catch {
    return {};
  }
}

// applyMySettings грузит секцию пользователя и применяет: язык, тему,
// дефолты калькулятора. Вызывать после входа (и при старте, если уже вошли).
export async function applyMySettings(): Promise<void> {
  let s: UserUISettings;
  try {
    s = await wails.getMyUISettings();
  } catch {
    return;
  }
  mySettings.set({ ...s, widths: s.widths ?? {}, colOrder: s.colOrder ?? {}, calcDefaults: s.calcDefaults ?? {} });
  // Язык: файл важнее глобальной БД.
  if (s.locale === 'ru' || s.locale === 'en') {
    locale.set(s.locale);
    document.documentElement.lang = s.locale;
  }
  // Тема: пресет известен фронту, кастомная — из БД, light/dark — CSS.
  if (s.theme) {
    const custom = await legacyThemeVars();
    const preset = THEME_PRESETS.find((p) => p.name === s.theme);
    if (preset) {
      const merged = { ...custom, [preset.name]: preset.vars as Record<string, string> };
      applyTheme(preset.name, merged);
    } else {
      applyTheme(s.theme, custom);
    }
  }
  // Дефолты калькулятора поверх заводских.
  if (s.calcDefaults && Object.keys(s.calcDefaults).length) {
    applyCalcDefaults(s.calcDefaults as Record<string, unknown>);
  }
}

// saveMySettingsPatch пишет часть секции (остальное — как было).
export async function saveMySettingsPatch(patch: Partial<UserUISettings>): Promise<void> {
  const cur = get(mySettings);
  const next = { ...cur, ...patch };
  mySettings.set(next);
  try {
    await wails.saveMyUISettings(next);
  } catch {
    /* офлайн — останется в памяти */
  }
}

// myHistoryColumns: колонки из файла или legacy-БД.
export async function myHistoryColumns(fallback: string[]): Promise<string[]> {
  const s = get(mySettings);
  if (s.historyColumns && s.historyColumns.length) return s.historyColumns;
  try {
    return (await wails.historyColumns()) ?? fallback;
  } catch {
    return fallback;
  }
}
