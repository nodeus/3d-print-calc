-- 0002_job_details: поля истории для round-trip CSV принтера.
-- Только ADD COLUMN (безопасно для существующих БД).
ALTER TABLE jobs ADD COLUMN end_at TEXT NOT NULL DEFAULT '';
ALTER TABLE jobs ADD COLUMN slicer TEXT NOT NULL DEFAULT '';
ALTER TABLE jobs ADD COLUMN size_bytes INTEGER NOT NULL DEFAULT 0;
ALTER TABLE jobs ADD COLUMN estimated_sec INTEGER NOT NULL DEFAULT 0;
ALTER TABLE jobs ADD COLUMN filament_total_mm REAL NOT NULL DEFAULT 0;
