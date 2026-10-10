// Клавиатурная перестановка колонок за грип.
// Использование: tabindex={0} role="button"
//   onkeydown={(e) => colKeyMove(e, ord, key, saveColOrder)}
// Стрелки двигают колонку, Home/End — в край. Мышь — как раньше (coldrag).
export function colKeyMove(
  e: KeyboardEvent,
  order: string[],
  key: string,
  save: (next: string[]) => void,
): void {
  const from = order.indexOf(key);
  if (from < 0) return;
  let to = -1;
  if (e.key === 'ArrowLeft') to = from - 1;
  else if (e.key === 'ArrowRight') to = from + 1;
  else if (e.key === 'Home') to = 0;
  else if (e.key === 'End') to = order.length - 1;
  else return;
  e.preventDefault();
  to = Math.max(0, Math.min(order.length - 1, to));
  if (to === from) return;
  const next = order.filter((k) => k !== key);
  next.splice(to, 0, key);
  save(next);
}
