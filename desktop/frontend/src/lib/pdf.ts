import { writable } from 'svelte/store';
import { wails, type PDFSection } from './wails';

// Реестр PDF-экспортёра текущей вкладки: страницы регистрируют функцию,
// строящую секции из своих видимых данных, шапка — общую кнопку PDF.
type Exporter = () => Promise<void>;
let current: Exporter | null = null;

// Видимость кнопки PDF в шапке: есть ли экспортёр у текущей вкладки.
export const hasPdfExporter = writable(false);

export function registerPdfExporter(fn: Exporter | null): void {
  current = fn;
  hasPdfExporter.set(!!fn);
}

export async function exportCurrentTab(): Promise<void> {
  if (current) await current();
}

// Имя файла: <название вкладки на языке интерфейса>_YYYY-MM-DD_hhmmss.pdf.
export function pdfFilename(title: string, now: Date = new Date()): string {
  const p = (v: number, w = 2) => String(v).padStart(w, '0');
  const stamp = `${now.getFullYear()}-${p(now.getMonth() + 1)}-${p(now.getDate())}_${p(now.getHours())}${p(now.getMinutes())}${p(now.getSeconds())}`;
  const safe = title.replace(/[/\\:*?"<>|]/g, '').trim() || 'report';
  return `${safe}_${stamp}.pdf`;
}

// Скачать PDF через backend (A4, футер nodeus.ru).
// Возвращает путь или null (отмена диалога / ошибка — в консоль).
export async function downloadPDF(title: string, sections: PDFSection[]): Promise<string | null> {
  try {
    return await wails.exportPDF(title, pdfFilename(title), sections);
  } catch (e) {
    console.error(e);
    return null;
  }
}
