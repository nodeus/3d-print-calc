# API-контракт (задел под Фазу 7 / Android)

> Задел: `server/main.go` — пока каркас. Набор ниже неполон и зафиксирует
> реальные методы `App` (bulk, импорт с preview, катушки, валюты, Moonraker)
> в Фазе 7.

Базовый принцип: сервер — тонкая обёртка над `core/` (те же формулы, те же коды ошибок).

```
GET  /api/v1/health
GET  /api/v1/materials
POST /api/v1/calculate        # -> core/calc.Calculate, снапшот тарифов в ответе
GET  /api/v1/jobs?limit=&q=
POST /api/v1/jobs/import      # CSV истории, dry-run флаг
GET  /api/v1/printers
GET  /api/v1/orders
POST /api/v1/backup/export    # backup.zip
POST /api/v1/backup/import
```

Деньги: минорные единицы + `currency` + `fx_rate`. Ошибки: `{ "code": "ERR_*", "messageKey": "<i18n-key>" }`.
Полная OpenAPI-спека — в Фазе 7.
