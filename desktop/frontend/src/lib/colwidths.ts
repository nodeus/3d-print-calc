import { writable, get } from 'svelte/store';
import { mySettings, saveMySettingsPatch } from './usersettings';

// Ширины колонок: таблица → колонка → px. Живут в секции пользователя
// (файл настроек) → переживают вкладки и перезапуск.
export const colWidths = writable<Record<string, Record<string, number>>>({});

// Зеркалим widths из секции (секция — источник истины).
mySettings.subscribe((s) => {
  colWidths.set(s.widths ?? {});
});

const MIN_W = 40;

export function widthOf(table: string, col: string): number | undefined {
  return get(colWidths)[table]?.[col];
}

// thWidth — style для th ([style] без ширины, если не задана).
export function thWidth(table: string, col: string): string {
  const w = widthOf(table, col);
  return w ? `width:${Math.round(w)}px;` : '';
}

async function commit(table: string, cols: Record<string, number>): Promise<void> {
  const all = { ...get(colWidths), [table]: cols };
  colWidths.set(all);
  await saveMySettingsPatch({ widths: all });
}

// resetColWidth — сброс ширины двойным кликом по ручке.
export async function resetColWidth(table: string, col: string): Promise<void> {
  const all = { ...get(colWidths) };
  if (!all[table]?.[col]) return;
  const cols = { ...all[table] };
  delete cols[col];
  if (!Object.keys(cols).length) delete all[table];
  else all[table] = cols;
  colWidths.set(all);
  await saveMySettingsPatch({ widths: all });
}

// startColResize — перетаскивание правого края th. Вешаем на span.rz:
// onmousedown={(e) => startColResize(e, 'history', c)}.
// Ширина пишется напрямую в DOM th (thWidth() нереактивен: читает стор через
// get(), и шаблон без этого не перерендеривается). Стор обновляем живьём,
// в файл коммитим на mouseup.
export function startColResize(e: MouseEvent, table: string, col: string): void {
  e.preventDefault();
  e.stopPropagation();
  const th = (e.currentTarget as HTMLElement).closest('th') as HTMLElement | null;
  const startX = e.clientX;
  const startW = widthOf(table, col) ?? th?.getBoundingClientRect().width ?? 100;
  const onMove = (ev: MouseEvent) => {
    const w = Math.max(MIN_W, Math.round(startW + ev.clientX - startX));
    if (th) th.style.width = `${w}px`;
    const all = { ...get(colWidths), [table]: { ...get(colWidths)[table], [col]: w } };
    colWidths.set(all); // живьём, без записи на каждое движение
  };
  const onUp = async () => {
    window.removeEventListener('mousemove', onMove);
    window.removeEventListener('mouseup', onUp);
    await commit(table, { ...get(colWidths)[table] });
  };
  window.addEventListener('mousemove', onMove);
  window.addEventListener('mouseup', onUp);
}
