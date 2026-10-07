// Package sqlite — реализация core/store.Repository на SQLite.
//
// Драйвер: modernc.org/sqlite (pure Go, без CGO) — обязательное требование
// для кросс-билда (см. AGENTS.md). Схема — только через migrations/,
// каждая миграция применяется один раз (таблица schema_migrations).
package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"3dprint-calc/core/models"
	"3dprint-calc/core/store"

	_ "modernc.org/sqlite"
)

// DB — соединение с app.db. Репозитории — через методы Materials/Printers/Jobs.
type DB struct {
	sql *sql.DB
}

var (
	_ store.Materials = (*materialsRepo)(nil)
	_ store.Printers  = (*printersRepo)(nil)
	_ store.Jobs      = (*jobsRepo)(nil)
)

// Open открывает (или создаёт) SQLite-файл: WAL, foreign keys, busy timeout.
func Open(path string) (*DB, error) {
	sqldb, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("ERR_DB_OPEN: %w", err)
	}
	for _, pragma := range []string{
		`PRAGMA journal_mode=WAL`,
		`PRAGMA foreign_keys=ON`,
		`PRAGMA busy_timeout=5000`,
	} {
		if _, err := sqldb.Exec(pragma); err != nil {
			sqldb.Close()
			return nil, fmt.Errorf("ERR_DB_PRAGMA: %w", err)
		}
	}
	return &DB{sql: sqldb}, nil
}

// Close закрывает соединение.
func (d *DB) Close() error { return d.sql.Close() }

// Materials возвращает репозиторий материалов и партий.
func (d *DB) Materials() store.Materials { return &materialsRepo{d.sql} }

// Printers возвращает репозиторий принтеров.
func (d *DB) Printers() store.Printers { return &printersRepo{d.sql} }

// Jobs возвращает репозиторий истории печати.
func (d *DB) Jobs() store.Jobs { return &jobsRepo{d.sql} }

// Migrate применяет все *.sql из dir по алфавиту, пропуская уже применённые.
func (d *DB) Migrate(dir string) error {
	if _, err := d.sql.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT '')`); err != nil {
		return fmt.Errorf("ERR_DB_MIGRATE: %w", err)
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		return fmt.Errorf("ERR_DB_MIGRATE: %w", err)
	}
	sort.Strings(files)
	for _, f := range files {
		name := filepath.Base(f)
		var n int
		if err := d.sql.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE name=?`, name).Scan(&n); err != nil {
			return fmt.Errorf("ERR_DB_MIGRATE: %w", err)
		}
		if n > 0 {
			continue
		}
		raw, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("ERR_DB_MIGRATE: %w", err)
		}
		if err := execScript(d.sql, string(raw)); err != nil {
			return fmt.Errorf("ERR_DB_MIGRATE %s: %w", name, err)
		}
		if _, err := d.sql.Exec(`INSERT INTO schema_migrations (name) VALUES (?)`, name); err != nil {
			return fmt.Errorf("ERR_DB_MIGRATE: %w", err)
		}
	}
	return nil
}

// execScript выполняет SQL-скрипт по операторам (в миграциях нет
// триггеров с внутренними ';').
func execScript(sqldb *sql.DB, script string) error {
	for _, stmt := range strings.Split(script, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := sqldb.Exec(stmt); err != nil {
			return fmt.Errorf("%w (%.80q)", err, stmt)
		}
	}
	return nil
}

type materialsRepo struct{ db *sql.DB }
type printersRepo struct{ db *sql.DB }
type jobsRepo struct{ db *sql.DB }

// UpsertByCode вставляет материал или обновляет его по коду.
func (r *materialsRepo) UpsertByCode(m models.Material) (int64, error) {
	res, err := r.db.Exec(`INSERT INTO materials (code, type, vendor, color_hex, density, diameter, note)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(code) DO UPDATE SET type=excluded.type, vendor=excluded.vendor,
		color_hex=excluded.color_hex, density=excluded.density, diameter=excluded.diameter, note=excluded.note`,
		m.Code, m.Type, m.Vendor, m.ColorHex, m.Density, m.Diameter, m.Note)
	if err != nil {
		return 0, fmt.Errorf("ERR_MATERIAL_SAVE: %w", err)
	}
	if id, _ := res.LastInsertId(); id != 0 {
		return id, nil
	}
	got, err := r.GetByCode(m.Code)
	if err != nil {
		return 0, err
	}
	return got.ID, nil
}

// GetByCode возвращает материал по коду.
func (r *materialsRepo) GetByCode(code string) (models.Material, error) {
	var m models.Material
	err := r.db.QueryRow(`SELECT id, code, type, vendor, color_hex, density, diameter, note
		FROM materials WHERE code=?`, code).
		Scan(&m.ID, &m.Code, &m.Type, &m.Vendor, &m.ColorHex, &m.Density, &m.Diameter, &m.Note)
	if err != nil {
		return m, fmt.Errorf("ERR_MATERIAL_GET: %w", err)
	}
	return m, nil
}

// List возвращает все материалы по коду.
func (r *materialsRepo) List() ([]models.Material, error) {
	rows, err := r.db.Query(`SELECT id, code, type, vendor, color_hex, density, diameter, note
		FROM materials ORDER BY code`)
	if err != nil {
		return nil, fmt.Errorf("ERR_MATERIAL_LIST: %w", err)
	}
	defer rows.Close()
	var out []models.Material
	for rows.Next() {
		var m models.Material
		if err := rows.Scan(&m.ID, &m.Code, &m.Type, &m.Vendor, &m.ColorHex, &m.Density, &m.Diameter, &m.Note); err != nil {
			return nil, fmt.Errorf("ERR_MATERIAL_LIST: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// AddSpool добавляет закупочную партию (катушку).
func (r *materialsRepo) AddSpool(s models.Spool) (int64, error) {
	res, err := r.db.Exec(`INSERT INTO spools (material_id, price, currency, fx_rate, bought_at,
		weight_init_g, weight_left_g, archived) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		s.MaterialID, s.Price, s.Currency, s.FxRate, s.BoughtAt, s.WeightInitG, s.WeightLeftG, s.Archived)
	if err != nil {
		return 0, fmt.Errorf("ERR_SPOOL_ADD: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// Save сохраняет принтер.
func (r *printersRepo) Save(p models.Printer) (int64, error) {
	res, err := r.db.Exec(`INSERT INTO printers (name, model, purchase_price, currency, commissioning_date,
		life_years, hours_per_day, calendar_mode, amort_method, power_kw, maint_per_hour, resource_hours)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Name, p.Model, p.PurchasePrice, p.Currency, p.CommissioningDate, p.LifeYears,
		p.HoursPerDay, p.CalendarMode, p.AmortMethod, p.PowerKW, p.MaintPerHour, p.ResourceHours)
	if err != nil {
		return 0, fmt.Errorf("ERR_PRINTER_SAVE: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// List возвращает все принтеры по имени.
func (r *printersRepo) List() ([]models.Printer, error) {
	rows, err := r.db.Query(`SELECT id, name, model, purchase_price, currency, commissioning_date,
		life_years, hours_per_day, calendar_mode, amort_method, power_kw, maint_per_hour, resource_hours
		FROM printers ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("ERR_PRINTER_LIST: %w", err)
	}
	defer rows.Close()
	var out []models.Printer
	for rows.Next() {
		var p models.Printer
		if err := rows.Scan(&p.ID, &p.Name, &p.Model, &p.PurchasePrice, &p.Currency,
			&p.CommissioningDate, &p.LifeYears, &p.HoursPerDay, &p.CalendarMode,
			&p.AmortMethod, &p.PowerKW, &p.MaintPerHour, &p.ResourceHours); err != nil {
			return nil, fmt.Errorf("ERR_PRINTER_LIST: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Upsert вставляет печать или обновляет её по паре (filename, start_at).
func (r *jobsRepo) Upsert(j models.Job) (int64, error) {
	res, err := r.db.Exec(`INSERT INTO jobs (filename, start_at, printer_id, spool_id, status,
		print_sec, total_sec, weight_g, length_m, cost_snapshot)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(filename, start_at) DO UPDATE SET printer_id=excluded.printer_id,
		spool_id=excluded.spool_id, status=excluded.status, print_sec=excluded.print_sec,
		total_sec=excluded.total_sec, weight_g=excluded.weight_g, length_m=excluded.length_m,
		cost_snapshot=excluded.cost_snapshot`,
		j.Filename, j.StartAt, nullInt(j.PrinterID), nullInt(j.SpoolID), j.Status,
		j.PrintSec, j.TotalSec, j.WeightG, j.LengthM, j.CostSnapshot)
	if err != nil {
		return 0, fmt.Errorf("ERR_JOB_SAVE: %w", err)
	}
	if id, _ := res.LastInsertId(); id != 0 {
		return id, nil
	}
	var id int64
	if err := r.db.QueryRow(`SELECT id FROM jobs WHERE filename=? AND start_at=?`,
		j.Filename, j.StartAt).Scan(&id); err != nil {
		return 0, fmt.Errorf("ERR_JOB_SAVE: %w", err)
	}
	return id, nil
}

// List возвращает последние печати (limit <= 0 — все).
func (r *jobsRepo) List(limit int) ([]models.Job, error) {
	q := `SELECT id, filename, start_at, printer_id, spool_id, status,
		print_sec, total_sec, weight_g, length_m, cost_snapshot
		FROM jobs ORDER BY start_at DESC`
	var rows *sql.Rows
	var err error
	if limit > 0 {
		rows, err = r.db.Query(q+` LIMIT ?`, limit)
	} else {
		rows, err = r.db.Query(q)
	}
	if err != nil {
		return nil, fmt.Errorf("ERR_JOB_LIST: %w", err)
	}
	defer rows.Close()
	var out []models.Job
	for rows.Next() {
		var j models.Job
		var printerID, spoolID sql.NullInt64
		if err := rows.Scan(&j.ID, &j.Filename, &j.StartAt, &printerID, &spoolID,
			&j.Status, &j.PrintSec, &j.TotalSec, &j.WeightG, &j.LengthM, &j.CostSnapshot); err != nil {
			return nil, fmt.Errorf("ERR_JOB_LIST: %w", err)
		}
		j.PrinterID, j.SpoolID = printerID.Int64, spoolID.Int64
		out = append(out, j)
	}
	return out, rows.Err()
}

// nullInt превращает 0 в NULL для необязательных внешних ключей.
func nullInt(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}
