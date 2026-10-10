-- 0014_drop_material_legacy_cols: дата закупа и архивность материала мертвы
-- (значимы только дата/архив катушки). Чистое удаление без легаси.
-- Исключение из конвенции ADD/CREATE (согласовано с заказчиком).
-- SQLite 3.35+ умеет DROP COLUMN, modernc.org/sqlite свежий — ок.
ALTER TABLE materials DROP COLUMN bought_at;
ALTER TABLE materials DROP COLUMN archived;
