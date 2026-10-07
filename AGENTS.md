# AGENTS.md — Калькулятор стоимости 3D-печати

> Источник истины для AI-агентов и разработчиков. Читать перед любыми изменениями.

## 1. Продукт и решения (зафиксировано с заказчиком)

- **Стек:** Wails v2 (Go-бэкенд) + Svelte + TypeScript (фронтенд) + SQLite. Подтверждено.
- **Амортизация по умолчанию:** календарная (`calendar`), не `NETWORKDAYS`.
  `hours_total = календарных_дней(СПИ) × часов_в_день`, `amort_hour = цена / hours_total`.
  Режим `workdays` — опциональный переключатель на принтер. Третий метод — `resource` (цена/ресурс_часов).
- **i18n v1:** RU + EN. Все строки UI — через ключи (`desktop/frontend/src/lib/i18n/{ru,en}.json`),
  ошибки ядра — кодами (`ERR_*`), текст — на стороне UI.
- **Мультивалютность v1:** `settings.base_currency = RUB`, курсы — ручной ввод (`fx_rates`).
  Заказы/катушки хранят `currency + fx_rate_snapshot + total_base`. Формат — через `Intl`.
- **Mobile:** задел под Android. Wails на Android не едет, поэтому **никакой логики
  расчёта во frontend и в SQL** — только `core/calc`. Будущий Android-клиент (отдельный
  проект, Flutter) пойдёт через `server/` REST (OpenAPI) поверх того же `core`
  либо через `backup.zip`. Офлайн-first сохраняется.
- **Принтер в парке:** Creality K1C + Moonraker. CSV-импорт — MVP, Moonraker `history/list` — Фаза 6.
- **Три режима:** (1) для себя — себестоимость, (2) заказ — клиент+позиции+статусы,
  (3) поток — очередь/отчёты/долги/НДС.

## 2. Исходные данные (legacy Excel)

- `xlsx/3dprint-calc.xlsx` — 3 листа: `Калькулятор печати`, `Материалы`, `print_history`.
- `csv/print_history.csv` — 491 строка, `;`-разделитель, десятичная запятая:
  `filename;type;status;size;start_time;end_time;estimated_time;print_duration;total_duration;filament_total;filament_used;slicer`.
- `printjob = print_history!A2:N1021`: 1-filename, 3-status, 4-start, 5-end,
  9-общее время(дни), 10-длина(м), 11-вес(г), 12-примечание.
- `Материалы`: `код | цена ₽/кг | дата покупки (serial Excel) | плотность` (~15 позиций
  `PETG-*/PLA-*`, плотность 1.27, одна 1.24). Цвета/производителя нет — зашиты в коде.
- Времена `G4/G5/G6` в калькуляторе — **доли суток Excel**; часы = `G*24` (в формулах `*1440/60`).
- Разбор формул и багов — `docs/excel-reverse-engineering.md`. **Обязательно к прочтению**
  перед работой с `core/calc`.

### Критические баги Excel (исправить, не портировать)

1. `E16 = B19*G5+1440/60` фактически `B19*G5 + 24` — лишние +24₽. Правильно `B19*G5*24`.
2. `NETWORKDAYS` (только будни) занижает часы фермы 24/7. Дефолт — календарные дни.
3. Плотность `1.23` захардкожена в весе, в справочнике `1.27/1.24`. Вес — по плотности материала.
4. Пустое примечание → `0` вместо `NULL`.
5. Неясно, какой из двух методов амортизации входит в себестоимость — в приложении метод
   выбирается **на принтер** явно.
6. Нет: % неудач, ТО, purge/брак, деления на N копий, снапшотов цен, НДС, округлений.

## 3. Архитектура

```
core/                 # чистый Go, ноль UI/ОС-зависимостей
  calc/               # движок расчёта (единственное место формул)
  models/             # User, Customer, Maker, Material, Spool, Printer, Job, Order, FxRate
  store/              # Repository-ИНТЕРФЕЙСЫ (без SQL)
  importers/          # csv_history, filename_parser (*_PETG-X_*_2h4m.gcode), gcode_meta
  export/             # backup.zip, csv/json/xlsx
  moonraker/          # клиент K1C
  i18n/               # ru/en словари ядра (только коды ошибок)
store-sqlite/         # реализация Repository (modernc.org/sqlite — pure Go, без CGO)
desktop/              # Wails v2: тонкие Go-биндинги + frontend Svelte+TS
  frontend/src/lib/i18n/{ru,en}.json
server/               # Фаза 7: REST поверх core для Android/общей БД (пока каркас)
mobile/               # задел: доки контракта API, кода пока нет
tools/                # ВСЕ вспомогательные скрипты — только здесь, ничего вне репо
docs/                 # спеки, контракты
migrations/           # SQL-миграции (goose-стиль, пронумерованные)
```

### Движок расчёта (канон)

```
hours      = total_sec / 3600
mat_cost   = price_per_kg / 1000 * weight_g
purge      = price_per_kg / 1000 * purge_waste_g          # башни/брак/поддержки
elec       = tariff * power_kw * hours
amort      = amort_method(printer) * hours   # calendar | workdays | resource
maint      = maint_per_hour * hours                       # ТО/оснастка
labor      = op_rate*prep_h + op_rate*post_h + designer_rate*model_h + model_price
subtotal   = mat + purge + elec + amort + maint + labor + extra_mat
fail_adj   = subtotal * fail_rate
cost       = subtotal + fail_adj                          # итог — за стол
price      = cost*(1+margin_profit+margin_dev) + fees, затем round(step) + VAT
per_copy   = cost_f / copies, price_f / copies            # итоги делятся на N копий
weight_g   = density(material) * PI * (diameter/2)^2 * length_mm / 1000
```

Нулевые `purge/maint/fail/VAT/round`, `copies<=1` — нейтральны (legacy-поведение).
Деньги в `calc` — float основных единиц; хранение — `int64` минорных + `fx_rate`.

Настройки амортизации/мощности/ТО — **на принтер**. Снапшот всех тарифов — в `jobs`/`order_items`.

### БД SQLite (WAL, один `app.db`)

`users, customers, makers(type: designer|modeller|printer_op), materials(code UNIQUE),
spools(material_id, price, currency, bought_at, weight_left_g), printers(...,
calendar_mode, amort_method), jobs(filename+start_at UNIQUE, cost_snapshot JSON),
orders, order_items, fx_rates, settings`. Импорт идемпотентен по `filename+start_at`.

## 4. GitFlow (обязательно)

- Ветки: `main` (релизы), `develop` (интеграция). Прямые коммиты в `main`/`develop` запрещены —
  только через PR/merge из `feature/*`, `release/*`, `hotfix/*`.
- `feature/<name>` от `develop` → PR в `develop`. `release/<version>` от `develop` →
  PR в `main` + back-merge в `develop`. `hotfix/<name>` от `main` → в `main` + `develop`.
- Тег релиза: `vX.Y.Z` на `main`.
- Коммиты: Conventional Commits (`feat:`, `fix:`, `docs:`, `chore:`, `refactor:`, `test:`), RU или EN,
  императив. Один коммит — одна логическая единица.

## 5. Конвенции кода

- Go 1.26+: `gofmt` чистый, `go vet ./...` без предупреждений. Пакеты — маленькие,
  экспортируемое — с doc-комментарием. Ошибки ядра — `ERR_CODE` + `%w`.
- Frontend: TypeScript strict, Prettier, ключи i18n без хардкода строк.
- SQL: миграции только добавлением файлов `migrations/NNNN_name.sql` (up-блоки, SQLite-совместимо).
- Тесты: `core/calc` — table-driven, фикстуры из `csv/print_history.csv`;
  регресс `E16` (+24₽) и плотности — обязательные кейсы.
- Валюты — коды ISO, деньги — `int64` минорных единиц + `fx_rate` снапшотом. Float — только для
  промежуточных kWh/часов, итог — округление по правилу.

## 6. Сборка и запуск (цели)

- Desktop: из `desktop/` — `wails build [-platform windows/amd64,linux/amd64,darwin/universal]`
  (сам ставит npm-пакеты и строит frontend). Один бинарник ~15МБ, `app.db` рядом,
  работа без установки Python/Node. Релизы — только через `go run ./tools/release`
  (папки `release/<version>/<дата>/`, в git не идут, см. `docs/release.md`). Напрямую `go build ./...` требует собранного
  `desktop/frontend/dist` (`npm run build`); `frontend/dist`, `node_modules`,
  `package.json.md5` — не в git. Копия `desktop/assets/{seed.csv,migrations}` обязана
  совпадать с каноном (`csv/`, `migrations/`) — следит `desktop/assets_sync_test.go`.
- Проверки: `go build ./...`, `go vet ./...`, `go test ./...`, `gofmt -l .` (пусто).
- Бэкап/восстановление: `backup.zip (app.db + csv/json)` в 1 клик, импорт с dry-run.

## 7. Что НЕ делать

- Не портировать баги Excel (см. раздел 2). Не класть расчёты во frontend/SQL.
- Не добавлять зависимости с CGO (ломает кросс-билд) без согласования.
- Не выносить инструменты/скрипты/временные файлы за пределы репо (`C:\Temp` запрещён —
  только `tools/`, `docs/` внутри проекта).
- Не коммитить `app.db`, `*.exe`, `release/`, `node_modules/`, секреты, реальные персональные данные.
- Не менять схему БД без новой миграции. Не менять формулы `core/calc` без обновления
  `docs/excel-reverse-engineering.md` и тестов.
