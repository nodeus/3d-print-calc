# tools/ — ВСЕ вспомогательные скрипты проекта

Правило (см. AGENTS.md): ничего вне репозитория — никакого `C:\Temp`.
Каждый скрипт — с шапкой: назначение, запуск, вход/выход.

- `seed/` (`go run ./tools/seed -db app.db`) — первичный сид БД из legacy-данных:
  материалы из `docs/excel-reverse-engineering.md`, принтер K1C, история из
  `csv/print_history.csv` со снапшотами себестоимости. Идемпотентно.
- `dbcheck.py` (`python tools/dbcheck.py [db] [код-материала]`) — только чтение:
  счётчики таблиц, последние работы, работы по материалу.
- `xlsx_probe.py` — (TODO) разбор `xlsx/3dprint-calc.xlsx` без pandas (только stdlib: zip+xml).

