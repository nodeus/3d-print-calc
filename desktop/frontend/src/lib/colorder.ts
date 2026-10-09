import { get } from 'svelte/store';
import { mySettings, saveMySettingsPatch } from './usersettings';

// Порядок колонок таблицы: сохранённый (без исчезнувших ключей)
// + недостающие ключи в конце. Чистая функция — страницы вызывают в $derived
// от $mySettings.colOrder, поэтому порядок реактивен.
export function orderFor(
  saved: Record<string, string[]> | undefined,
  table: string,
  defKeys: string[],
): string[] {
  const kept = (saved?.[table] ?? []).filter((k) => defKeys.includes(k));
  for (const k of defKeys) if (!kept.includes(k)) kept.push(k);
  return kept;
}

// saveTableOrder пишет порядок таблицы в секцию пользователя (файл настроек).
export async function saveTableOrder(table: string, order: string[]): Promise<void> {
  const cur = get(mySettings);
  const next = { ...(cur.colOrder ?? {}), [table]: order };
  await saveMySettingsPatch({ colOrder: next });
}
