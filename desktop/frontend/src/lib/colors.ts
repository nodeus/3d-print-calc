// Базовая палитра цветов пластика для быстрого выбора (источник —
// docs/material-colors.md; коды — ориентиры, у брендов оттенки гуляют).
// Показывается в выборе цвета всегда, плюс личные фавориты пользователя.
const PLA = [
  '#000000', '#ffffff', '#63646b', '#e9301e',
  '#033877', '#00c26b', '#ffff00', '#af654e',
];
const PETG = [
  '#000000', '#ffffff', '#d5d8dd', '#ff0000', '#ff854d',
  '#034e89', '#38af69', '#ffc946', '#7142a3', '#f79dbc',
];
const ABS = [
  '#000000', '#ffffff', '#3d9441', '#7142a3',
  '#ffff47', '#ff311f', '#f8f8ee',
];
const GENERIC = [
  '#000000', '#ffffff', '#808080', '#ff0000',
  '#ffc946', '#034e89', '#00c26b',
];

const BASE: Record<string, string[]> = { PLA, PETG, ABS };

// baseColors — базовые цвета под тип пластика (неизвестный тип — generic).
export function baseColors(matType: string): string[] {
  return BASE[(matType || '').toUpperCase()] ?? GENERIC;
}
