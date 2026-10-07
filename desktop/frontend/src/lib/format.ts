// Форматирование денег/дат через Intl (мультивалютность v1).
export function money(value: number, currency: string, locale: string): string {
  try {
    return new Intl.NumberFormat(locale === 'ru' ? 'ru-RU' : 'en-US', {
      style: 'currency',
      currency,
      maximumFractionDigits: 2,
    }).format(value);
  } catch {
    return `${value.toFixed(2)} ${currency}`;
  }
}

export function num(value: number, digits = 2): string {
  return value.toLocaleString('ru-RU', { maximumFractionDigits: digits });
}

export function hms(totalSec: number): string {
  const h = Math.floor(totalSec / 3600);
  const m = Math.round((totalSec % 3600) / 60);
  return `${h}h ${m}m`;
}
