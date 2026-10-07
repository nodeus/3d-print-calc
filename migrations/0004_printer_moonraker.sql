-- 0004_printer_moonraker: адрес Moonraker на принтер (пусто = не настроен).
ALTER TABLE printers ADD COLUMN moonraker_url TEXT NOT NULL DEFAULT '';
