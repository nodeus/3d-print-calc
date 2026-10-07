import { wails, type PDFSection } from './wails';

// Скачать PDF через backend (A4, футер nodeus.ru).
// Возвращает путь или null (отмена диалога / ошибка — в консоль).
export async function downloadPDF(title: string, sections: PDFSection[]): Promise<string | null> {
  try {
    return await wails.exportPDF(title, sections);
  } catch (e) {
    console.error(e);
    return null;
  }
}
