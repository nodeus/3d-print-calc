# AGENTS.md — Калькулятор стоимости 3D-печати

> Источник истины для AI-агентов и разработчиков. Читать перед любыми изменениями.
> Текущий релиз: **v1.4.18** на `main`. Фиксы — инкрементом `1.4.x`, мажорный релиз — только по команде заказчика.

## 1. Продукт и решения (зафиксировано с заказчиком)

- **Стек:** Wails v2 (Go-бэкенд) + Svelte 5 + TypeScript (фронтенд) + SQLite. Подтверждено.
- **Амортизация по умолчанию:** календарная (`calendar`), не `NETWORKDAYS`.
  `hours_total = календарных_дней(СПИ) × часов_в_день`, `amort_hour = цена / hours_total`.
  Режим `workdays` — опциональный переключатель на принтер. Третий метод — `resource` (цена/ресурс_часов).
- **i18n:** RU + EN. Все строки UI — через ключи (`desktop/frontend/src/lib/i18n/{ru,en}.json`),
  ошибки ядра — кодами (`ERR_*`), текст — на стороне UI.
- **Мультивалютность v1:** `settings.base_currency = RUB`, курсы — ручной ввод (`fx_rates`).
  Заказы/катушки хранят `currency + fx_rate_snapshot + total_base`. Формат — через `Intl`.
- **Mobile:** задел под Android. Wails на Android не едет, поэтому **никакой логики
  расчёта во frontend и в SQL** — только `core/calc`. Будущий Android-клиент (отдельный
  проект, Flutter) пойдёт через `server/` REST (OpenAPI) поверх того же `core`
  либо через `backup.zip`. Офлайн-first сохраняется.
- **Принтер в парке:** Creality K1C + Moonraker. CSV-импорт и Moonraker `history/list`
  работают (dry-run + прогресс). Адрес Moonraker вводится **со схемой** (`http://IP:7125`).
  Импорт забирает **всю** историю постранично (`core/moonraker.HistoryAll`)
  либо N последних (`HistoryRecent`, лимит из UI; пусто — всё). CSV-лимит —
  топ-N по времени старта (`takeLastJobs` в `ImportHistory`), превью и
  применение режут одинаково (лимит запоминается в `pendingCSVLimit`).
  Превью — табличное (`ImportReport.Rows` + флаг `Exists`, модалка
  `lib/ImportPreview.svelte`, пагинация по `historyPerPage`); импорт —
  только после предпросмотра (инлайн-кнопка по отмеченным, `pending*` +
  `ERR_NO_PENDING`). В модалке — только просмотр и чекбоксы
  (одна кнопка «Закрыть»); выбор общий со страницей (`bind:checked`).
  В модалке — чекбоксы строк (импорт только отмеченных: `Apply*(selected)`),
  кнопки «Предпросмотр импорта» (открыть заново) и «Отменить импорт»
  (`CancelPendingImport` сбрасывает `pending*`, включая `pendingMats`).
  Материалы — тот же флоу: табличное превью (`MatRows` + `SpoolNo`/`Exists`,
  модалка `lib/ImportMatsPreview.svelte`), свой лимит хвоста файла
  (`TakeLastMats`, `pendingMatsLimit`), применение подмножества
  (`ImportMatsSelected`).
  Импорт строго последовательный: предпросмотр при занятом чужом буфере
  запрещён (`ERR_PENDING_EXISTS` в начале каждого `Preview*`; свой тип
  перезапускать можно). Поле «Последние N» — одно на историю и материалы.
- **Три режима:** (1) для себя — себестоимость, (2) заказ — клиент+позиции+статусы,
  (3) поток — очередь/отчёты/долги/НДС.
- **Номера заказов** — по шаблону (`core/ordnum`): `{YYYY} {YY} {MM} {DD} {HH} {mm} {ss}`,
  серия `{n…n}` (длина = мин. ширина), литерал `{"текст"}`. Дефолт `{YYYY}-{nnnn}`.

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
core/                       # чистый Go, ноль UI/ОС-зависимостей
  calc/                     # движок расчёта (единственное место формул)
  models/                   # User, Customer, Maker, Material, Spool, Printer, Job, Order(+Item), Payment, FxRate
  store/                    # Repository-ИНТЕРФЕЙСЫ (без SQL)
  importers/                # csv_history (толерантен к пустым/"null"), filename_parser, gcode_meta
  export/                   # backup.zip, csv, pdf.go (gofpdf + DejaVu) + fonts/
  moonraker/                # клиент K1C (NormalizeBase добавляет http://).
                            # ВАЖНО: count в history/list = размер СТРАНИЦЫ, не базы!
                            # Стоп только: пустая страница / нет новых job_id (+ maxPages).
                            # totals.total_jobs — lifetime-счётчик для сверки.
  ordnum/                   # шаблон номеров заказов
  auth/                     # argon2id (PHC без v=, 5 частей)
  i18n/                     # ru/en словари ядра (только коды ошибок)
store-sqlite/               # реализация Repository (modernc.org/sqlite — pure Go, без CGO)
  seed/                     # EnsureAdmin (чистый старт) + SeedAll (только CLI tools/seed)
desktop/                    # Wails v2: тонкие Go-биндинги + frontend Svelte+TS
  frontend/src/{pages,lib/{i18n,confirm,pdf,nav,prefill},table.css}
  frontend/src/assets/donate-qr.png
  assets/migrations/        # копии канона для go:embed (go:embed запрещает ..)
server/                     # Фаза 7: REST поверх core для Android/общей БД (пока каркас)
mobile/                     # задел: доки контракта API, кода пока нет
tools/                      # ВСЕ вспомогательные скрипты — только здесь: seed, release, qrcode, dbcheck
docs/                       # HELP (руководство), release, updating, api-contract, excel-reverse-engineering
migrations/                 # SQL-миграции (только ADD COLUMN / CREATE, пронумерованные;
                          # исключение — 0014 DROP мёртвых колонок materials, согласован с заказчиком)
utils/                      # upx.exe для релизов (в git НЕ идёт, локальный инструмент)
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
price      = cost*(1+margin_profit+margin_dev) + fees, затем round(step) + VAT + round(step)
           # второй проход round убирает копейки, возвращённые НДС (280×1.22=341.6→340)
per_copy   = cost_f / copies, price_f / copies            # итоги делятся на N копий
weight_g   = density(катушка, фолбэк материал) * PI * (diameter/2)^2 * length_mm / 1000
```

Нулевые `purge/maint/fail/VAT/round`, `copies<=1` — нейтральны (legacy-поведение).
Деньги в `calc` — float основных единиц; хранение — `int64` минорных + `fx_rate`.
Снапшот расчёта в `jobs.cost_snapshot` — **v2**: `{v, material, input:{tariff,powerKW,
amortPerHour,opRate,prepH,postH,...,spoolNo}, cost, price, currency}` (v1 читается с фолбэком).
Импорт пишет `price` и `spoolNo` сразу; старые строки без них чинит разовый
`BackfillImportSnapshots` при старте (только `edited=0`, ручные не трогает).
Правка работы (`UpdateManual`) пересчитывает строго по полям формы правки
(полный `input`: тарифы, ставки, наценки, НДС, копии...), цена материала — текущая
(первая неархивная катушка по id — `materialPrice`, UI её метит «расчётная»).
Номера катушек (`spool_no`, миграция `0013`): имя файла `..._КОД-N_...`
→ материал КОД + катушка N (`ParseMaterialSpool`; точный код приоритетен).
Импорт (CSV/Moonraker через `ImportRows`) линкует N-ю катушку и её цену;
нет номера — стаб-катушка N с линком; база без совпадения — стаб базы.
Калькулятор и правка выбирают катушку (`spoolId` + `input.spoolNo`
в снапшоте); цена пересчёта — выбранной. История — колонка `spool`
(`КОД-N` из снапшота).
Катушки правятся только своим UI (Materials ▸); логика цены не менялась.
Перенос «В калькулятор»: физика (вес/время/материал) — всегда;
ставки — только из настоящих данных (`manual` или `edited=1`), чистый импорт
пользовательские ставки не затирает. Форма калькулятора живёт в `calcForm`
+ `localStorage` (`calcform.v1`) — переживает вкладки и перезапуск.
Дефолты калькулятора (`factoryValues` + `userDefaults` из файла настроек):
кнопка в калькуляторе + редактор в Настройках.
Числовые поля форм — `number | null` (`null` = пусто, виден плейсхолдер);
на границе расчёта/сохранения — `?? 0` (`copies/qty ?? 1`).
Время работ (подготовка/пост/модель) — ввод парами ч+мин (`lib/hm.ts`),
внутри/снапшотах — десятичные часы. Копий нет: всегда `Copies: 1`
(движок деление хранит для старых данных).
Колонки истории — 33 ключа (`HistoryColumns` в `desktop/app.go`: 13 базовых +
20 расчётных из снапшота); настройка — карточка с аккордеонами в Настройках
(таб «Основное»), ярлык «Колонки: N/33» вверху Истории. Порядок колонок —
drag за грип `⋮⋮` (общий `startColDrag` в `lib/coldrag.ts`, порядок —
`orderFor`/`colOrder` в секции пользователя; у Истории порядок живёт
в массиве видимых колонок).
Ширины колонок — drag за край `th` (`span.rz`, `table.tw`), хранятся
`widths[table][col]` в секции пользователя. Сортировка `button.th` — во всех таблицах.
Межвкладочная навигация — `lib/nav.ts` (`goTab` + payload-сторы по образцу
`openOrderId`: `openMaterial{code,spoolNo}` → Materials раскрывает и подсвечивает).
Счётчик катушек у материала — всегда из DTO (`m.spools`: живые + архив).
Настройки интерфейса — `user_settings.json` рядом с `app.db`
(`desktop/uisettings.go`): окно общее, остальное по логинам; нет файла — дефолты.
Размер окна задаётся в options до `wails.Run` (SetSize после старта ненадёжен),
позиция — в `startup`; размер живьём пишет фронт на resize (`SaveWindowSize`).

Настройки амортизации/мощности/ТО — **на принтер**.

### БД SQLite (WAL, один `app.db`)

`users, customers, makers(type: designer|modeller|printer_op), materials(code UNIQUE),
+part_purchases(printer_id, bought_at, price, qty — сумма вычисляется),
spools(material_id, price, currency, bought_at, weight_left_g, spool_no,
vendor, color_hex, density, diameter — миграция `0016` с бэкфиллом с материала;
материал — лишь ключ-группа для парсинга, физика партии — на катушке;
вес работ — по плотности/диаметру привязанной катушки),
materials.code правится с переписью снапшота (`json_replace`);
дата закупа и архивность — только spools.bought_at/spools.archived
(остатки, цена, селект); колонки materials.bought_at/materials.archived
удалены миграцией `0014` (были мертвы с `0011`).
Остаток катушки — вычисляемый (`SpoolUsedG`: партия минус Σ веса работ,
атрибуция как в `SpoolStats`); хранимое `spools.weight_left_g` в показе
не участвует (автосписания нет), printers(...,
calendar_mode, amort_method, moonraker_url), jobs(filename+start_at UNIQUE,
calendar_mode, amort_method, moonraker_url), jobs(filename+start_at UNIQUE,
cost_snapshot JSON v2, origin: import|manual|moonraker, + end_at/slicer/size/...),
orders(+number, +title), order_items, payments, fx_rates, settings`. Импорт идемпотентен
по `filename+start_at`. Удаление: ручные работы — свободно; материалы/принтеры/
заказчики — блок при живых связях (пустые катушки — каскадом); заказы — каскадом
с подтверждением. Массовое удаление истории (чекбоксы, `DeleteJobs`): всё
выбранное — физически (включая нетронутые импорты, файлы добираются
реимпортом); связанные с позициями заказов пропускаются с именами в отчёте.
Массовая правка (`BulkPreview`/`BulkApply`): разреженный патч, пишутся
только реально изменившиеся поля; edited/orig — как в одиночной правке.

## 4. GitFlow (обязательно)

- Ветки: `main` (релизы), `develop` (интеграция). Прямые коммиты в `main`/`develop` запрещены —
  только через PR/merge из `feature/*`, `release/*`, `hotfix/*`.
- `feature/<name>` от `develop` → PR в `develop`. `release/<version>` от `develop` →
  PR в `main` + back-merge в `develop`. `hotfix/<name>` от `main` → в `main` + `develop`.
- Тег релиза: `vX.Y.Z` на `main`. Версионирование: фиксы — `1.0.x` от текущей,
  **мажорный релиз — только по команде заказчика**.
- Коммиты: Conventional Commits (`feat:`, `fix:`, `docs:`, `chore:`, `refactor:`, `test:`), RU или EN,
  императив. Один коммит — одна логическая единица.
- Большие плановые задачи — строго по одной фиче за раз: реализовал фичу →
  проверки (`go build ./...`, `go vet ./...`, `go test ./...`, `gofmt -l .`,
  `npm run check`) → dev-билд → проверка заказчиком → коммит → только потом
  следующая фича. Несколько фич в одном коммите запрещены — чтобы не было
  путаницы и чтобы любую фичу можно было откатить, не затрагивая другие.
- После мёржа topic-ветки в `develop` ветку удалять локально
  (`git branch -d <name>`); remote — по мере появления.

## 5. Конвенции кода

- Go 1.26+: `gofmt` чистый, `go vet ./...` без предупреждений. Пакеты — маленькие,
  экспортируемое — с doc-комментарием. Ошибки ядра — `ERR_CODE` + `%w`.
- Списки из Go обязаны быть `[]`, не `null` (`out := []T{}`): пустой JSON-массив,
  иначе фронт падает на `.length`. Тест `TestEmptyListsSerializeAsArray` — обязателен.
- Frontend: TypeScript strict, Prettier, ключи i18n без хардкода строк.
  **Запрещён нативный `confirm()`** (показывает `wails.localhost`) — только `lib/confirm.ts` +
  `<Confirm/>` в `App.svelte`. Фигурные скобки в атрибутах — только как выражения
  (`placeholder={'{YYYY}'}`), иначе Svelte парсит их как JS.
- Вкладки — в `<svelte:boundary>` + `{#key tab}` (ошибка не залипает между вкладками).
  `onMount` — не `async` с возвратом подписки (svelte-check это ловит).
- SQL: миграции только добавлением файлов `migrations/NNNN_name.sql` (up-блоки, SQLite-совместимо).
- Тесты: `core/calc` — table-driven, фикстуры из `csv/print_history.csv`;
  регресс `E16` (+24₽) и плотности — обязательные кейсы.
- Валюты — коды ISO, деньги — `int64` минорных единиц + `fx_rate` снапшотом. Float — только для
  промежуточных kWh/часов, итог — округление по правилу.
- PowerShell пишет редиректы в UTF-16 и не знает `&&`/`||`: инлайн-скрипты — только через
  файлы (`Write` → `C:\Temp\*.py` → запуск → удалить), heredoc-синтаксиса нет.
- Moonraker `count` не использовать как тотал БД. Постраничные обходы — только
  с дедупом по серверным id и предохранителем от вечного цикла; тест обязан
  эмулировать кэп страницы и `count=len(page)` (регресс v1.0.2 → v1.0.3).

## 6. Сборка и запуск (цели)

- Desktop: из `desktop/` — `wails build [-platform windows/amd64,linux/amd64,darwin/universal]`
  (сам ставит npm-пакеты и строит frontend). Один бинарник, `app.db` рядом,
  работа без установки Python/Node. Релизы — только через `go run ./tools/release`
  (папки `release/<version>/<дата>/`, в git не идут, см. `docs/release.md`;
  Windows-exe пакуются `utils/upx.exe`, флаг `-upx=false` отключает). Напрямую `go build ./...`
  требует собранного `desktop/frontend/dist` (`npm run build`); `frontend/dist`,
  `node_modules`, `package.json.md5` — не в git. Копия `desktop/assets/migrations` обязана
  совпадать с каноном (`migrations/`) — следит `desktop/assets_sync_test.go`.
- Первый запуск чистый: только `admin/admin123` (пароль сменить сразу).
  Работы/материалы/принтеры пользователь заводит сам: CSV-импорт, Moonraker, вручную.
  Полный сид legacy-данных — только dev-инструмент `go run ./tools/seed -db ...`.
- Проверки: `go build ./...`, `go vet ./...`, `go test ./...`, `gofmt -l .` (пусто),
  `npm run check` в `desktop/frontend` (svelte-check: ловит ошибки шаблонов,
  которые `vite build` пропускает — напр. `{YYYY}` в атрибутах).
- Порядок сдачи работы: после правок кода — dev-билд (`wails build` из `desktop/`),
  затем просьба заказчику проверить визуально/вживую и только потом коммит.
  Без подтверждения проверки не коммитить. Коммит — на каждую фичу отдельно,
  не пачкой в конце задачи (см. раздел 4).
- Бэкап/восстановление: `backup.zip (app.db + csv/json)` в 1 клик, импорт с dry-run.

## 7. Что НЕ делать

- Не портировать баги Excel (см. раздел 2). Не класть расчёты во frontend/SQL.
- Не добавлять зависимости с CGO (ломает кросс-билд) без согласования.
- Не выносить инструменты/скрипты/временные файлы за пределы репо (`C:\Temp` запрещён —
  только `tools/`, `docs/` внутри проекта).
- Не коммитить `app.db`, `*.exe`, `release/`, `node_modules/`, секреты, реальные персональные данные.
- Не менять схему БД без новой миграции. Не менять формулы `core/calc` без обновления
  `docs/excel-reverse-engineering.md` и тестов.
