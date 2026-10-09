// Package sqlite — реализация core/store.Repository на SQLite.
//
// Драйвер: modernc.org/sqlite (pure Go, без CGO) — обязательное требование
// для кросс-билда (см. AGENTS.md). Схема — только через migrations/,
// каждая миграция применяется один раз (таблица schema_migrations).
package sqlite

import (
	"database/sql"
	"fmt"
	"io/fs"
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
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		return fmt.Errorf("ERR_DB_MIGRATE: %w", err)
	}
	names := make([]string, len(files))
	contents := make([]string, len(files))
	for i, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("ERR_DB_MIGRATE: %w", err)
		}
		names[i], contents[i] = filepath.Base(f), string(raw)
	}
	return d.applyMigrations(names, contents)
}

// MigrateFS — то же для встроенных миграций (go:embed в desktop).
func (d *DB) MigrateFS(fsys fs.FS, dir string) error {
	files, err := fs.Glob(fsys, filepath.ToSlash(filepath.Join(dir, "*.sql")))
	if err != nil {
		return fmt.Errorf("ERR_DB_MIGRATE: %w", err)
	}
	sort.Strings(files)
	names := make([]string, len(files))
	contents := make([]string, len(files))
	for i, f := range files {
		raw, err := fs.ReadFile(fsys, f)
		if err != nil {
			return fmt.Errorf("ERR_DB_MIGRATE: %w", err)
		}
		names[i], contents[i] = filepath.Base(f), string(raw)
	}
	return d.applyMigrations(names, contents)
}

// applyMigrations накатывает скрипты по порядку имён, пропуская применённые.
func (d *DB) applyMigrations(names, contents []string) error {
	if _, err := d.sql.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT '')`); err != nil {
		return fmt.Errorf("ERR_DB_MIGRATE: %w", err)
	}
	order := make([]int, len(names))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(a, b int) bool { return names[order[a]] < names[order[b]] })
	for _, i := range order {
		var n int
		if err := d.sql.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE name=?`, names[i]).Scan(&n); err != nil {
			return fmt.Errorf("ERR_DB_MIGRATE: %w", err)
		}
		if n > 0 {
			continue
		}
		if err := execScript(d.sql, contents[i]); err != nil {
			return fmt.Errorf("ERR_DB_MIGRATE %s: %w", names[i], err)
		}
		if _, err := d.sql.Exec(`INSERT INTO schema_migrations (name) VALUES (?)`, names[i]); err != nil {
			return fmt.Errorf("ERR_DB_MIGRATE: %w", err)
		}
	}
	return nil
}
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
	var archived int
	err := r.db.QueryRow(`SELECT id, code, type, vendor, color_hex, density, diameter,
		default_spool_weight_g, note, bought_at, archived FROM materials WHERE code=?`, code).
		Scan(&m.ID, &m.Code, &m.Type, &m.Vendor, &m.ColorHex, &m.Density, &m.Diameter,
			&m.DefaultSpoolWeightG, &m.Note, &m.BoughtAt, &archived)
	if err != nil {
		return m, fmt.Errorf("ERR_MATERIAL_GET: %w", err)
	}
	m.Archived = archived != 0
	return m, nil
}

// List возвращает все материалы по коду.
func (r *materialsRepo) List() ([]models.Material, error) {
	rows, err := r.db.Query(`SELECT id, code, type, vendor, color_hex, density, diameter,
		default_spool_weight_g, note, bought_at, archived FROM materials ORDER BY code`)
	if err != nil {
		return nil, fmt.Errorf("ERR_MATERIAL_LIST: %w", err)
	}
	defer rows.Close()
	out := []models.Material{}
	for rows.Next() {
		var m models.Material
		var archived int
		if err := rows.Scan(&m.ID, &m.Code, &m.Type, &m.Vendor, &m.ColorHex, &m.Density,
			&m.Diameter, &m.DefaultSpoolWeightG, &m.Note, &m.BoughtAt, &archived); err != nil {
			return nil, fmt.Errorf("ERR_MATERIAL_LIST: %w", err)
		}
		m.Archived = archived != 0
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

// ListSpools возвращает партии материала (неархивные первыми).
func (r *materialsRepo) ListSpools(materialID int64) ([]models.Spool, error) {
	rows, err := r.db.Query(`SELECT id, material_id, price, currency, fx_rate, bought_at,
		weight_init_g, weight_left_g, archived FROM spools
		WHERE material_id=? ORDER BY archived, id`, materialID)
	if err != nil {
		return nil, fmt.Errorf("ERR_SPOOL_LIST: %w", err)
	}
	defer rows.Close()
	out := []models.Spool{}
	for rows.Next() {
		var s models.Spool
		var archived int
		if err := rows.Scan(&s.ID, &s.MaterialID, &s.Price, &s.Currency, &s.FxRate,
			&s.BoughtAt, &s.WeightInitG, &s.WeightLeftG, &archived); err != nil {
			return nil, fmt.Errorf("ERR_SPOOL_LIST: %w", err)
		}
		s.Archived = archived != 0
		out = append(out, s)
	}
	return out, rows.Err()
}

// Save сохраняет принтер.
func (r *printersRepo) Save(p models.Printer) (int64, error) {
	res, err := r.db.Exec(`INSERT INTO printers (name, model, purchase_price, currency, commissioning_date,
		life_years, hours_per_day, calendar_mode, amort_method, power_kw, maint_per_hour, resource_hours,
		moonraker_url)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Name, p.Model, p.PurchasePrice, p.Currency, p.CommissioningDate, p.LifeYears,
		p.HoursPerDay, p.CalendarMode, p.AmortMethod, p.PowerKW, p.MaintPerHour, p.ResourceHours,
		p.MoonrakerURL)
	if err != nil {
		return 0, fmt.Errorf("ERR_PRINTER_SAVE: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// List возвращает все принтеры по имени.
func (r *printersRepo) List() ([]models.Printer, error) {
	rows, err := r.db.Query(`SELECT id, name, model, purchase_price, currency, commissioning_date,
		life_years, hours_per_day, calendar_mode, amort_method, power_kw, maint_per_hour, resource_hours,
		moonraker_url
		FROM printers ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("ERR_PRINTER_LIST: %w", err)
	}
	defer rows.Close()
	out := []models.Printer{}
	for rows.Next() {
		var p models.Printer
		if err := rows.Scan(&p.ID, &p.Name, &p.Model, &p.PurchasePrice, &p.Currency,
			&p.CommissioningDate, &p.LifeYears, &p.HoursPerDay, &p.CalendarMode,
			&p.AmortMethod, &p.PowerKW, &p.MaintPerHour, &p.ResourceHours,
			&p.MoonrakerURL); err != nil {
			return nil, fmt.Errorf("ERR_PRINTER_LIST: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// SetMoonrakerURL задаёт адрес Moonraker принтера.
func (r *printersRepo) SetMoonrakerURL(id int64, url string) error {
	if _, err := r.db.Exec(`UPDATE printers SET moonraker_url=? WHERE id=?`, url, id); err != nil {
		return fmt.Errorf("ERR_PRINTER_MOONRAKER: %w", err)
	}
	return nil
}

// Upsert вставляет печать или обновляет её по паре (filename, start_at).
func (r *jobsRepo) Upsert(j models.Job) (int64, error) {
	res, err := r.db.Exec(`INSERT INTO jobs (filename, start_at, end_at, printer_id, spool_id, status,
		origin, print_sec, total_sec, estimated_sec, size_bytes, filament_total_mm,
		weight_g, length_m, slicer, cost_snapshot)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(filename, start_at) DO UPDATE SET end_at=excluded.end_at,
		printer_id=excluded.printer_id, spool_id=excluded.spool_id, status=excluded.status,
		print_sec=excluded.print_sec, total_sec=excluded.total_sec,
		estimated_sec=excluded.estimated_sec, size_bytes=excluded.size_bytes,
		filament_total_mm=excluded.filament_total_mm, weight_g=excluded.weight_g,
		length_m=excluded.length_m, slicer=excluded.slicer,
		cost_snapshot=excluded.cost_snapshot`,
		j.Filename, j.StartAt, j.EndAt, nullInt(j.PrinterID), nullInt(j.SpoolID), j.Status,
		j.Origin, j.PrintSec, j.TotalSec, j.EstimatedSec, j.SizeBytes, j.FilamentTotalMm,
		j.WeightG, j.LengthM, j.Slicer, j.CostSnapshot)
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

// DeleteJob удаляет работу: ручные (manual) — полностью; изменённые
// импортированные (edited=1) — откатываются к первозданному, строка остаётся.
// Нетронутые import/moonraker удалять нельзя (данные из источника).
func (r *jobsRepo) DeleteJob(id int64) error {
	var origin string
	var edited int
	if err := r.db.QueryRow(`SELECT origin, edited FROM jobs WHERE id=?`, id).Scan(&origin, &edited); err != nil {
		return fmt.Errorf("ERR_JOB_DELETE: %w", err)
	}
	if origin == "" {
		origin = "import"
	}
	if origin != "manual" {
		if edited == 0 {
			return fmt.Errorf("ERR_JOB_DELETE: импортированные работы удалять нельзя")
		}
		return r.RevertJob(id)
	}
	if _, err := r.db.Exec(`DELETE FROM jobs WHERE id=?`, id); err != nil {
		return fmt.Errorf("ERR_JOB_DELETE: %w", err)
	}
	return nil
}

// List возвращает последние печати (limit <= 0 — все).
func (r *jobsRepo) List(limit int) ([]models.Job, error) {
	q := `SELECT id, filename, start_at, end_at, printer_id, spool_id, status, origin,
		edited, print_sec, total_sec, estimated_sec, size_bytes, filament_total_mm,
		weight_g, length_m, slicer, cost_snapshot
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
	out := []models.Job{}
	for rows.Next() {
		var j models.Job
		var printerID, spoolID sql.NullInt64
		var edited int
		if err := rows.Scan(&j.ID, &j.Filename, &j.StartAt, &j.EndAt, &printerID, &spoolID,
			&j.Status, &j.Origin, &edited, &j.PrintSec, &j.TotalSec, &j.EstimatedSec, &j.SizeBytes,
			&j.FilamentTotalMm, &j.WeightG, &j.LengthM, &j.Slicer, &j.CostSnapshot); err != nil {
			return nil, fmt.Errorf("ERR_JOB_LIST: %w", err)
		}
		j.PrinterID, j.SpoolID, j.Edited = printerID.Int64, spoolID.Int64, edited
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

// GetSetting читает настройку (value — JSON-строка). ok=false — нет ключа.
func (d *DB) GetSetting(key string) (value string, ok bool, err error) {
	err = d.sql.QueryRow(`SELECT value FROM settings WHERE key=?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("ERR_SETTING_GET: %w", err)
	}
	return value, true, nil
}

// SetSetting записывает настройку (upsert).
func (d *DB) SetSetting(key, value string) error {
	if _, err := d.sql.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value); err != nil {
		return fmt.Errorf("ERR_SETTING_SET: %w", err)
	}
	return nil
}
