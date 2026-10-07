-- 0005_order_number: человекочитаемый номер заказа по шаблону (см. core/ordnum).
-- Пусто у старых заказов — UI показывает фолбэк #id.
ALTER TABLE orders ADD COLUMN number TEXT NOT NULL DEFAULT '';
