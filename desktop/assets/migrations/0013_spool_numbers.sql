-- 0013_spool_numbers: порядковый номер катушки внутри материала (1, 2, 3...).
-- Backfill существующих по порядку создания. Новые получают max+1 (см. AddSpool).
ALTER TABLE spools ADD COLUMN spool_no INTEGER NOT NULL DEFAULT 0;
UPDATE spools SET spool_no = (
	SELECT COUNT(*) FROM spools s2
	WHERE s2.material_id = spools.material_id AND s2.id <= spools.id
) WHERE spool_no = 0;
