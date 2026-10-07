-- 0006_job_origin: откуда работа — import (CSV/сид), manual (калькулятор), moonraker.
-- Удалять из UI можно только manual.
ALTER TABLE jobs ADD COLUMN origin TEXT NOT NULL DEFAULT 'import';
