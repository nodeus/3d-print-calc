-- 0009_job_editing: правки импортированных работ.
-- edited=1 + orig_snapshot (первозданное состояние) = бейдж «изменена» и откат.
-- default_spool_weight_g: вес новых катушек материала (дефолт 1000 г).
ALTER TABLE jobs ADD COLUMN edited INTEGER NOT NULL DEFAULT 0;
ALTER TABLE jobs ADD COLUMN orig_snapshot TEXT NOT NULL DEFAULT '';
ALTER TABLE materials ADD COLUMN default_spool_weight_g REAL NOT NULL DEFAULT 1000;
