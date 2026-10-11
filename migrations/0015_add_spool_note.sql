-- 0015_add_spool_note: заметка катушки (произвольный текст закупки).
ALTER TABLE spools ADD COLUMN note TEXT NOT NULL DEFAULT '';
