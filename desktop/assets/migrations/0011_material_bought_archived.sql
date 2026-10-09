-- 0011_material_bought_archived: дата закупа (может отсутствовать) и флаг архивности материала.
ALTER TABLE materials ADD COLUMN bought_at TEXT NOT NULL DEFAULT '';
ALTER TABLE materials ADD COLUMN archived INTEGER NOT NULL DEFAULT 0;
