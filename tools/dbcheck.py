"""dbcheck.py — быстрая сверка app.db: таблицы, счётчики, последние работы.

Использование из корня репо:
    python tools/dbcheck.py [путь-к-db]   # по умолчанию app.db
Только чтение, ничего не меняет.
"""
import sqlite3
import sys

path = sys.argv[1] if len(sys.argv) > 1 else "app.db"
c = sqlite3.connect(path)

tables = [r[0] for r in c.execute(
    "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")]
print("tables:", ", ".join(tables))
for t in ("materials", "spools", "printers", "jobs", "orders",
          "customers", "users", "fx_rates"):
    if t in tables:
        print(f"{t}: {c.execute(f'SELECT COUNT(*) FROM {t}').fetchone()[0]}")

if "materials" in tables:
    for row in c.execute(
            "SELECT code, type, density FROM materials ORDER BY code LIMIT 8"):
        print("mat:", row)
if "jobs" in tables:
    for row in c.execute(
            "SELECT filename, status, ROUND(weight_g, 2), "
            "SUBSTR(cost_snapshot, 1, 100) FROM jobs "
            "ORDER BY start_at DESC LIMIT 3"):
        print("job:", row)
    if len(sys.argv) > 2:
        # tools/dbcheck.py db jobs-by-material <код>
        code = sys.argv[2]
        for row in c.execute(
                "SELECT filename FROM jobs "
                "WHERE cost_snapshot LIKE '%\"material\":\"' || ? || '\"%' LIMIT 5",
                (code,)):
            print("job-by-mat:", row[0])
