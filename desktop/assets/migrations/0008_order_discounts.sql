-- 0008_order_discounts: скидки заказа вместо наценок/НДС в шапке.
-- Итог = max(0, SUM(price*qty) - discount_pct% - discount_fixed).
ALTER TABLE orders ADD COLUMN discount_pct REAL NOT NULL DEFAULT 0;
ALTER TABLE orders ADD COLUMN discount_fixed INTEGER NOT NULL DEFAULT 0;
-- Позиция оплаты: NULL = весь заказ, иначе конкретная позиция.
ALTER TABLE payments ADD COLUMN order_item_id INTEGER NULL REFERENCES order_items(id);
CREATE INDEX IF NOT EXISTS idx_payments_item ON payments(order_item_id);
