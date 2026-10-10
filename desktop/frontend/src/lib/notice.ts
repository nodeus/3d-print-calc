import { writable } from 'svelte/store';

// Общий тост «действие принято»: страницы вызывают notify() после успешных
// сохранений/применений, слот — в шапке App.svelte (виден с любой вкладки).
// Одно активное сообщение; следующее действие перезапускает таймер.
export const noticeMsg = writable('');

let timer: number | undefined;

export function notify(text: string, ms = 3000): void {
  window.clearTimeout(timer);
  noticeMsg.set(text);
  timer = window.setTimeout(() => noticeMsg.set(''), ms);
}
