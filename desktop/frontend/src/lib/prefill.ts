import { writable } from 'svelte/store';
import type { JobDTO } from './wails';

// Работа, выбранная в истории для расчёта в калькуляторе.
// Calculator применяет один раз при монтировании/изменении и очищает.
export const prefillJob = writable<JobDTO | null>(null);
