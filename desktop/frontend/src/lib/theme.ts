import { writable } from 'svelte/store';
import { wails } from './wails';

// Переменные темы (ключи = CSS-переменные без --).
export const THEME_VARS = [
  'bg',
  'fg',
  'card-border',
  'nav-bg',
  'header-bg',
  'header-fg',
  'btn-bg',
  'btn-fg',
  'btn-border',
  'input-bg',
  'input-fg',
  'muted',
  'err',
  'warn',
  'accent',
  'sel-bg',
  'final-border',
  'sw-border',
] as const;

export type ThemeVars = Record<string, string>;

export interface UISettings {
  theme: string; // light | dark | имя кастомной
  custom: Record<string, ThemeVars>;
}

export const uiTheme = writable<string>('light');

// applyTheme ставит тему: light/dark — атрибут, custom — атрибут + переменные.
export function applyTheme(name: string, custom: Record<string, ThemeVars>): void {
  const vars = custom[name];
  if (name !== 'light' && name !== 'dark' && vars) {
    const root = document.documentElement;
    root.setAttribute('data-theme', 'custom');
    for (const k of THEME_VARS) {
      if (vars[k]) root.style.setProperty(`--${k}`, vars[k]);
      else root.style.removeProperty(`--${k}`);
    }
  } else {
    document.documentElement.setAttribute('data-theme', name === 'dark' ? 'dark' : 'light');
    const root = document.documentElement;
    for (const k of THEME_VARS) root.style.removeProperty(`--${k}`);
  }
  uiTheme.set(name);
}

export async function loadTheme(): Promise<UISettings> {
  try {
    const s = await wails.uiSettings();
    applyTheme(s.theme || 'light', s.custom ?? {});
    return { theme: s.theme || 'light', custom: s.custom ?? {} };
  } catch {
    applyTheme('light', {});
    return { theme: 'light', custom: {} };
  }
}

export async function persistTheme(name: string): Promise<void> {
  try {
    await wails.setUITheme(name);
  } catch {
    /* noop */
  }
}
