-- 0007_order_title: человекочитаемое название заказа (дефолт при создании —
-- «Клиент — ДД.ММ.ГГГГ», правится вручную). Пусто у старых — UI показывает номер.
ALTER TABLE orders ADD COLUMN title TEXT NOT NULL DEFAULT '';
