// motionOK — разрешены ли анимации: нет, если ОС просит reduced-motion.
// Используется там, где Svelte-переходы не подчиняются CSS-медиазапросу.
export function motionOK(): boolean {
  try {
    return !window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  } catch {
    return true;
  }
}
