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

// Пресеты тем — известные палитры редакторов/IDE, разложенные на 18
// переменных приложения: фон/текст — editor-цвета, accent — фирменный цвет
// палитры, кнопки/инпуты — приподнятый тон, шапка — elevated.
function mk(
  bg: string,
  elev: string,
  fg: string,
  muted: string,
  border: string,
  accent: string,
  err: string,
  warn: string,
  sel: string,
): ThemeVars {
  return {
    bg,
    fg,
    'card-border': border,
    'nav-bg': elev,
    'header-bg': elev,
    'header-fg': fg,
    'btn-bg': elev,
    'btn-fg': fg,
    'btn-border': border,
    'input-bg': bg,
    'input-fg': fg,
    muted,
    err,
    warn,
    accent,
    'sel-bg': sel,
    'final-border': border,
    'sw-border': border,
  };
}

export interface ThemePreset {
  name: string;
  dark: boolean;
  vars: ThemeVars;
}

function dark(name: string, bg: string, elev: string, fg: string, muted: string, border: string, accent: string, err: string, warn: string, sel: string): ThemePreset {
  return { name, dark: true, vars: mk(bg, elev, fg, muted, border, accent, err, warn, sel) };
}

function light(name: string, bg: string, elev: string, fg: string, muted: string, border: string, accent: string, err: string, warn: string, sel: string): ThemePreset {
  return { name, dark: false, vars: mk(bg, elev, fg, muted, border, accent, err, warn, sel) };
}

export const THEME_PRESETS: ThemePreset[] = [
  // Тёмные (21).
  dark('Dracula', '#282a36', '#44475a', '#f8f8f2', '#6272a4', '#44475a', '#bd93f9', '#ff5555', '#ffb86c', '#44475a'),
  dark('Monokai', '#272822', '#3e3d32', '#f8f8f2', '#75715e', '#3e3d32', '#a6e22e', '#f92672', '#fd971f', '#49483e'),
  dark('One Dark', '#282c34', '#2c313a', '#abb2bf', '#5c6370', '#3e4451', '#61afef', '#e06c75', '#d19a66', '#3e4451'),
  dark('Solarized Dark', '#002b36', '#073642', '#93a1a1', '#586e75', '#073642', '#268bd2', '#dc322f', '#cb4b16', '#073642'),
  dark('Nord', '#2e3440', '#3b4252', '#d8dee9', '#616e88', '#434c5e', '#88c0d0', '#bf616a', '#d08770', '#434c5e'),
  dark('Gruvbox Dark', '#282828', '#3c3836', '#ebdbb2', '#928374', '#3c3836', '#fabd2f', '#fb4934', '#fe8019', '#3c3836'),
  dark('GitHub Dark', '#0d1117', '#161b22', '#e6edf3', '#7d8590', '#30363d', '#2f81f7', '#f85149', '#d29922', '#1c2e4a'),
  dark('Tokyo Night', '#1a1b26', '#24283b', '#c0caf5', '#565f89', '#2f354d', '#7aa2f7', '#f7768e', '#ff9e64', '#2f3d5c'),
  dark('Catppuccin Mocha', '#1e1e2e', '#313244', '#cdd6f4', '#6c7086', '#313244', '#cba6f7', '#f38ba8', '#fab387', '#45475a'),
  dark('Catppuccin Macchiato', '#24273a', '#363a4f', '#cad3f5', '#6e738d', '#363a4f', '#c6a0f6', '#ed8796', '#f5a97f', '#494d64'),
  dark('Night Owl', '#011627', '#0b2942', '#d6deeb', '#637777', '#0b2942', '#82aaff', '#ef5350', '#f78c6c', '#1d3b53'),
  dark('Palenight', '#292d3e', '#32374d', '#a6accd', '#676e95', '#32374d', '#c792ea', '#ff5370', '#f78c6c', '#343a55'),
  dark('Ayu Mirage', '#1f2430', '#272d38', '#cbccc6', '#5c6773', '#272d38', '#ffcc66', '#ff3333', '#ffa759', '#33415e'),
  dark('SynthWave 84', '#262335', '#34294f', '#dcdcf0', '#848bbd', '#34294f', '#ff7edb', '#fe4450', '#fede5d', '#34294f'),
  dark('Cobalt2', '#193549', '#0d3a58', '#ffffff', '#5b9bc4', '#0d3a58', '#ffc600', '#ff5f5f', '#ff9e64', '#1f4e79'),
  dark('Shades of Purple', '#2d2b55', '#3a375e', '#e2e0f5', '#7970a9', '#3a375e', '#b362ff', '#d93963', '#ff9e64', '#4d4d7a'),
  dark('Material Ocean', '#0f111a', '#1a1e2a', '#a6accd', '#4b5263', '#1a1e2a', '#89ddff', '#f07178', '#ffcb6b', '#232b3d'),
  dark('Rose Pine', '#191724', '#26233a', '#e0def4', '#6e6a86', '#26233a', '#c4a7e7', '#eb6f92', '#f6c177', '#403d52'),
  dark('Everforest Dark', '#2d353b', '#343f44', '#d3c6aa', '#859289', '#343f44', '#a7c080', '#e67e80', '#dbbc7f', '#3d484d'),
  dark('Kanagawa Wave', '#1f1f28', '#2a2a37', '#dcd7ba', '#727169', '#2a2a37', '#7e9cd8', '#e46876', '#ffa066', '#2d4f67'),
  dark('Horizon', '#1c1e26', '#232530', '#d5d8da', '#85878c', '#232530', '#e95678', '#ec6a88', '#fab795', '#2e303e'),
  // Светлые (11).
  light('One Light', '#fafafa', '#f0f0f0', '#383a42', '#a0a1a7', '#d5d5d6', '#4078f2', '#e45649', '#c18401', '#e5e5e6'),
  light('Solarized Light', '#fdf6e3', '#eee8d5', '#657b83', '#93a1a1', '#eee8d5', '#268bd2', '#dc322f', '#cb4b16', '#eee8d5'),
  light('Gruvbox Light', '#fbf1c7', '#ebdbb2', '#3c3836', '#928374', '#d5c4a1', '#076678', '#9d0006', '#b57614', '#ebdbb2'),
  light('GitHub Light', '#ffffff', '#f6f8fa', '#1f2328', '#59636e', '#d1d9e0', '#0969da', '#d1242f', '#9a6700', '#dbe9fd'),
  light('Catppuccin Latte', '#eff1f5', '#e6e9ef', '#4c4f69', '#8c8fa1', '#ccd0da', '#8839ef', '#d20f39', '#fe6400', '#ccd0da'),
  light('Ayu Light', '#fafafa', '#f3f4f5', '#5c6773', '#abb0bf', '#e0e2e5', '#36a3d9', '#f26d78', '#e6b450', '#e8eaed'),
  light('Everforest Light', '#fffbef', '#f4f0d9', '#5c6a72', '#939f91', '#e0dcc7', '#8da101', '#f85552', '#dfa000', '#e0dcc7'),
  light('Rose Pine Dawn', '#faf4ed', '#f2e9e1', '#575279', '#9893a5', '#dfdad9', '#907aa9', '#b4637a', '#ea9d34', '#dfdad9'),
  light('Tokyo Night Light', '#d5d6db', '#c1c2c7', '#565a6e', '#9699a3', '#c1c2c7', '#34548a', '#8c4351', '#965027', '#c4c8da'),
  light('Material Lighter', '#fafafa', '#eceff1', '#546e7a', '#90a4ae', '#d7dfe3', '#00acc1', '#e53935', '#ef6c00', '#d7dfe3'),
  light('Quiet Light', '#f5f5f5', '#e9e9e9', '#333333', '#6e6e6e', '#d4d4d4', '#0451a8', '#e51400', '#895503', '#d4e2f2'),
];

export const THEME_PRESET_DARK = THEME_PRESETS.filter((p) => p.dark);
export const THEME_PRESET_LIGHT = THEME_PRESETS.filter((p) => !p.dark);

// applyTheme ставит тему: light/dark — атрибут, custom — атрибут + переменные.
// color-scheme выставляется явно по флагу пресета, чтобы нативные контролы
// (чекбоксы, спиннеры, скроллбары) не брали схему из ОС. Чужим импортным
// темам без флага и базовым light/dark сбрасываем в дефолт CSS.
export function applyTheme(name: string, custom: Record<string, ThemeVars>): void {
  const vars = custom[name];
  if (name !== 'light' && name !== 'dark' && vars) {
    const root = document.documentElement;
    root.setAttribute('data-theme', 'custom');
    for (const k of THEME_VARS) {
      if (vars[k]) root.style.setProperty(`--${k}`, vars[k]);
      else root.style.removeProperty(`--${k}`);
    }
    const preset = THEME_PRESETS.find((p) => p.name === name);
    if (preset) root.style.colorScheme = preset.dark ? 'dark' : 'light';
    else root.style.removeProperty('color-scheme');
  } else {
    document.documentElement.setAttribute('data-theme', name === 'dark' ? 'dark' : 'light');
    const root = document.documentElement;
    for (const k of THEME_VARS) root.style.removeProperty(`--${k}`);
    root.style.removeProperty('color-scheme');
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
