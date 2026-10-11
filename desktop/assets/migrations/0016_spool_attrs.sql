-- 0016_spool_attrs: свойства партии на катушке (вариант A — материал это ключ).
-- Вендор/цвет/плотность/диаметр живут на закупке, бэкфилл с материала.
ALTER TABLE spools ADD COLUMN vendor TEXT NOT NULL DEFAULT '';
ALTER TABLE spools ADD COLUMN color_hex TEXT NOT NULL DEFAULT '';
ALTER TABLE spools ADD COLUMN density REAL NOT NULL DEFAULT 1.27;
ALTER TABLE spools ADD COLUMN diameter REAL NOT NULL DEFAULT 1.75;
UPDATE spools SET
  vendor = (SELECT vendor FROM materials WHERE materials.id = spools.material_id),
  color_hex = (SELECT color_hex FROM materials WHERE materials.id = spools.material_id),
  density = (SELECT density FROM materials WHERE materials.id = spools.material_id),
  diameter = (SELECT diameter FROM materials WHERE materials.id = spools.material_id);
