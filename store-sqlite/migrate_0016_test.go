package sqlite

import (
	"path/filepath"
	"testing"
)

// TestMigration0016SpoolAttrs: апгрейд старой БД (схема до 0016) обязан
// добавить колонки партии и скопировать вендор/цвет/плотность/диаметр
// с материала на его катушки. Свежие БД покрыты openTestDB.
func TestMigration0016SpoolAttrs(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "old.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	// Старая схема: materials полный + spools без новых колонок.
	setup := []string{
		`CREATE TABLE materials (
			id INTEGER PRIMARY KEY, code TEXT NOT NULL UNIQUE,
			type TEXT NOT NULL DEFAULT 'PETG', vendor TEXT NOT NULL DEFAULT '',
			color_hex TEXT NOT NULL DEFAULT '', density REAL NOT NULL DEFAULT 1.27,
			diameter REAL NOT NULL DEFAULT 1.75, note TEXT NOT NULL DEFAULT '',
			default_spool_weight_g REAL NOT NULL DEFAULT 1000)`,
		`INSERT INTO materials (code, type, vendor, color_hex, density, diameter)
			VALUES ('PETG-NIT', 'PETG', 'НИТ', '#FF0000', 1.24, 2.85)`,
		`CREATE TABLE spools (
			id INTEGER PRIMARY KEY, material_id INTEGER NOT NULL,
			price INTEGER NOT NULL DEFAULT 0, currency TEXT NOT NULL DEFAULT 'RUB',
			fx_rate REAL NOT NULL DEFAULT 1, bought_at TEXT NOT NULL DEFAULT '',
			weight_init_g REAL NOT NULL DEFAULT 1000, weight_left_g REAL NOT NULL DEFAULT 1000,
			archived INTEGER NOT NULL DEFAULT 0, spool_no INTEGER NOT NULL DEFAULT 0,
			note TEXT NOT NULL DEFAULT '')`,
		`INSERT INTO spools (material_id, price, spool_no) VALUES (1, 90000, 1)`,
		`CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT '')`,
	}
	for _, q := range setup {
		if _, err := db.sql.Exec(q); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}
	files, err := filepath.Glob("../migrations/*.sql")
	if err != nil || len(files) == 0 {
		t.Fatalf("glob: %v", err)
	}
	for _, f := range files {
		name := filepath.Base(f)
		if name == "0016_spool_attrs.sql" {
			continue
		}
		if _, err := db.sql.Exec(`INSERT INTO schema_migrations (name) VALUES (?)`, name); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}
	if err := db.Migrate("../migrations"); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	var vendor, color string
	var density, diameter float64
	if err := db.sql.QueryRow(`SELECT vendor, color_hex, density, diameter FROM spools WHERE id=1`).
		Scan(&vendor, &color, &density, &diameter); err != nil {
		t.Fatalf("spool: %v", err)
	}
	if vendor != "НИТ" || color != "#FF0000" || density != 1.24 || diameter != 2.85 {
		t.Fatalf("бэкфилл: %q %q %v %v", vendor, color, density, diameter)
	}
}
