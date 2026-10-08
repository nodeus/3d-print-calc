-- 0010_item_paid_manual: ручная отметка оплаты позиции (без движения денег).
-- Статус «Оплачен» вручную не меняет суммы/долги/выручку (считаются по деньгам).
ALTER TABLE order_items ADD COLUMN paid_manual INTEGER NOT NULL DEFAULT 0;
