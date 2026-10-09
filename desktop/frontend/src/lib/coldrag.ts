// Общий pointer-drag перестановки колонок за грип.
// Использование в странице:
//   let dragKey: string | null = $state(null);
//   let dropX: number | null = $state(null);
//   let wrapEl: HTMLElement | null = $state(null);
//   <div class="twrap" bind:this={wrapEl}> ... table.twrap ...
//   <th data-col={key}> {#if canWrite}<span class="grip" onpointerdown={(e) => startColDrag(e, wrapEl, key, ord, (k, x) => { dragKey = k; dropX = x; }, onDrop)} />{/if}
export function startColDrag(
  e: PointerEvent,
  wrapEl: HTMLElement | null,
  dragKey: string,
  order: string[],
  setDrag: (key: string | null, x: number | null) => void,
  onDrop: (next: string[]) => void,
): void {
  e.preventDefault();
  e.stopPropagation();
  const startX = e.clientX;
  let dragging = false;
  let idx = -1;
  const ths = () => [...(wrapEl?.querySelectorAll('thead th[data-col]') ?? [])] as HTMLElement[];
  const onMove = (ev: PointerEvent) => {
    if (!dragging) {
      if (Math.abs(ev.clientX - startX) < 4) return;
      dragging = true;
    }
    // Позиция вставки: первый th, чья середина правее курсора.
    const list = ths();
    let i = list.length;
    let lineX: number | null = null;
    const wrapR = wrapEl?.getBoundingClientRect();
    for (let k = 0; k < list.length; k++) {
      const r = list[k].getBoundingClientRect();
      if (ev.clientX < r.left + r.width / 2) {
        i = k;
        lineX = wrapR ? r.left - wrapR.left : null;
        break;
      }
      lineX = wrapR ? r.right - wrapR.left : null;
    }
    idx = i;
    setDrag(dragKey, lineX);
  };
  const onUp = () => {
    window.removeEventListener('pointermove', onMove);
    window.removeEventListener('pointerup', onUp);
    setDrag(null, null);
    if (!dragging || idx < 0) return;
    const from = order.indexOf(dragKey);
    if (from < 0) return;
    // Индекс считался со строкой-источником: после удаления сдвиг.
    let to = idx > from ? idx - 1 : idx;
    const next = order.filter((k) => k !== dragKey);
    to = Math.max(0, Math.min(next.length, to));
    next.splice(to, 0, dragKey);
    if (next.join() !== order.join()) onDrop(next);
  };
  window.addEventListener('pointermove', onMove);
  window.addEventListener('pointerup', onUp);
}
