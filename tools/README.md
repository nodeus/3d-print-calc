# tools/ — ВСЕ вспомогательные скрипты проекта

Правило (см. AGENTS.md): ничего вне репозитория — никакого `C:\Temp`.
Каждый скрипт — с шапкой: назначение, запуск, вход/выход.

- `import_csv.py` — (TODO Фаза 2) импорт `csv/print_history.csv` в `app.db`, dry-run по умолчанию.
- `xlsx_probe.py` — (TODO) разбор `xlsx/3dprint-calc.xlsx` без pandas (только stdlib: zip+xml).
