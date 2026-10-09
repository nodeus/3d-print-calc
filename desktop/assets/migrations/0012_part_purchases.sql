-- 0012_part_purchases: закупки запчастей и ТО по принтерам.
-- Сумма итого вычисляемая (price*qty), цены — минорные единицы рублей.
CREATE TABLE IF NOT EXISTS part_purchases (
	id INTEGER PRIMARY KEY,
	printer_id INTEGER NOT NULL REFERENCES printers(id),
	bought_at TEXT NOT NULL DEFAULT '',
	name TEXT NOT NULL DEFAULT '',
	price INTEGER NOT NULL DEFAULT 0,
	qty REAL NOT NULL DEFAULT 1,
	note TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_part_purchases_printer ON part_purchases(printer_id);
