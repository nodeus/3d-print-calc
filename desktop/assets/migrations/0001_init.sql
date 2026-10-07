-- 0001_init: пользователи, контрагенты, материалы, принтеры, работы, заказы, валюты.
-- SQLite-совместимо. WAL включается на уровне соединения (PRAGMA journal_mode=WAL).
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY,
  login TEXT NOT NULL UNIQUE,
  pass_argon2 TEXT NOT NULL,
  role TEXT NOT NULL CHECK (role IN ('admin','manager','viewer')),
  locale TEXT NOT NULL DEFAULT 'ru'
);

CREATE TABLE IF NOT EXISTS customers (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  contact TEXT NOT NULL DEFAULT '',
  currency TEXT NOT NULL DEFAULT 'RUB',
  note TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS makers (
  id INTEGER PRIMARY KEY,
  type TEXT NOT NULL CHECK (type IN ('designer','modeller','printer_op')),
  name TEXT NOT NULL,
  rate_per_h INTEGER NOT NULL DEFAULT 0,
  currency TEXT NOT NULL DEFAULT 'RUB'
);

CREATE TABLE IF NOT EXISTS materials (
  id INTEGER PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  type TEXT NOT NULL DEFAULT 'PETG',
  vendor TEXT NOT NULL DEFAULT '',
  color_hex TEXT NOT NULL DEFAULT '',
  density REAL NOT NULL DEFAULT 1.27,
  diameter REAL NOT NULL DEFAULT 1.75,
  note TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS spools (
  id INTEGER PRIMARY KEY,
  material_id INTEGER NOT NULL REFERENCES materials(id),
  price INTEGER NOT NULL DEFAULT 0,
  currency TEXT NOT NULL DEFAULT 'RUB',
  fx_rate REAL NOT NULL DEFAULT 1,
  bought_at TEXT NOT NULL DEFAULT '',
  weight_init_g REAL NOT NULL DEFAULT 0,
  weight_left_g REAL NOT NULL DEFAULT 0,
  archived INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS printers (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  model TEXT NOT NULL DEFAULT '',
  purchase_price INTEGER NOT NULL DEFAULT 0,
  currency TEXT NOT NULL DEFAULT 'RUB',
  commissioning_date TEXT NOT NULL DEFAULT '',
  life_years INTEGER NOT NULL DEFAULT 3,
  hours_per_day REAL NOT NULL DEFAULT 8,
  calendar_mode TEXT NOT NULL DEFAULT 'calendar' CHECK (calendar_mode IN ('calendar','workdays')),
  amort_method TEXT NOT NULL DEFAULT 'calendar' CHECK (amort_method IN ('calendar','workdays','resource')),
  power_kw REAL NOT NULL DEFAULT 0.35,
  maint_per_hour INTEGER NOT NULL DEFAULT 0,
  resource_hours REAL NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS jobs (
  id INTEGER PRIMARY KEY,
  filename TEXT NOT NULL,
  start_at TEXT NOT NULL,
  printer_id INTEGER REFERENCES printers(id),
  spool_id INTEGER REFERENCES spools(id),
  status TEXT NOT NULL DEFAULT 'completed',
  print_sec INTEGER NOT NULL DEFAULT 0,
  total_sec INTEGER NOT NULL DEFAULT 0,
  weight_g REAL NOT NULL DEFAULT 0,
  length_m REAL NOT NULL DEFAULT 0,
  cost_snapshot TEXT NOT NULL DEFAULT '{}',
  UNIQUE (filename, start_at)
);

CREATE TABLE IF NOT EXISTS orders (
  id INTEGER PRIMARY KEY,
  customer_id INTEGER REFERENCES customers(id),
  currency TEXT NOT NULL DEFAULT 'RUB',
  fx_rate REAL NOT NULL DEFAULT 1,
  margin_profit REAL NOT NULL DEFAULT 0,
  margin_dev REAL NOT NULL DEFAULT 0,
  vat REAL NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'draft',
  total_base INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS order_items (
  id INTEGER PRIMARY KEY,
  order_id INTEGER NOT NULL REFERENCES orders(id),
  job_id INTEGER REFERENCES jobs(id),
  title TEXT NOT NULL DEFAULT '',
  qty REAL NOT NULL DEFAULT 1,
  cost INTEGER NOT NULL DEFAULT 0,
  price INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS fx_rates (
  currency TEXT NOT NULL,
  rate REAL NOT NULL,
  day TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (currency, day)
);

CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL DEFAULT '{}'
);
