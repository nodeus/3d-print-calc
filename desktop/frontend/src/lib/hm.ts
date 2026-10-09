// Часы+минуты: ввод времени парами полей, внутри — десятичные часы.
// Минуты >= 60 переносятся в часы при сборке.
export function splitHours(dec: number | null): { h: number | null; m: number | null } {
  if (dec == null) return { h: null, m: null };
  const h = Math.floor(dec);
  const m = Math.round((dec - h) * 60);
  if (m >= 60) return { h: h + 1, m: m - 60 };
  return { h, m };
}

export function joinHours(h: number | null, m: number | null): number {
  const hh = h ?? 0;
  const mm = m ?? 0;
  return hh + Math.floor(mm / 60) + ((mm % 60) / 60);
}

// normHM сворачивает минуты в часы для отображения (90м → 1ч30м).
export function normHM(h: number | null, m: number | null): { h: number | null; m: number | null } {
  if (h == null && m == null) return { h: null, m: null };
  const total = Math.round((h ?? 0) * 60 + (m ?? 0));
  return { h: Math.floor(total / 60), m: total % 60 };
}
