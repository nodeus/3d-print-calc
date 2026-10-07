-- 0003_payments: оплаты по заказам для отчёта по долгам.
CREATE TABLE IF NOT EXISTS payments (
  id INTEGER PRIMARY KEY,
  order_id INTEGER NOT NULL REFERENCES orders(id),
  amount INTEGER NOT NULL DEFAULT 0,
  paid_at TEXT NOT NULL DEFAULT '',
  note TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_payments_order ON payments(order_id);
-- created_at заказа обязателен для отчётов по месяцам (старые строки — пусто).
