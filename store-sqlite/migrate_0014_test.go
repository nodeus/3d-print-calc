package sqlite

import (
	"path/filepath"
	"testing"
)

// TestMigration0014DropsLegacyCols: апгрейд старой БД (схема до 0014) обязан
// удалить materials.bought_at/materials.archived, сохранив остальные данные.
// Свежие БД покрыты openTestDB (Migrate всего каталога).
func TestMigration0014DropsLegacyCols(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "old.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	// Старая схема: materials как после 0011 + пометки о применённых миграциях.
	setup := []string{
		`CREATE TABLE materials (
			id INTEGER PRIMARY KEY, code TEXT NOT NULL UNIQUE,
			type TEXT NOT NULL DEFAULT 'PETG', vendor TEXT NOT NULL DEFAULT '',
			color_hex TEXT NOT NULL DEFAULT '', density REAL NOT NULL DEFAULT 1.27,
			diameter REAL NOT NULL DEFAULT 1.75, note TEXT NOT NULL DEFAULT '',
			bought_at TEXT NOT NULL DEFAULT '', archived INTEGER NOT NULL DEFAULT 0)`,
		`INSERT INTO materials (code, type, density, bought_at, archived)
			VALUES ('PETG-OLD', 'PETG', 1.27, '2025-01-01', 1)`,
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
		if name == "0014_drop_material_legacy_cols.sql" {
			continue
		}
		if _, err := db.sql.Exec(`INSERT INTO schema_migrations (name) VALUES (?)`, name); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}
	if err := db.Migrate("../migrations"); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	rows, err := db.sql.Query(`SELECT name FROM pragma_table_info('materials')`)
	if err != nil {
		t.Fatalf("pragma: %v", err)
	}
	defer rows.Close()
	cols := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		cols[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if cols["bought_at"] || cols["archived"] {
		t.Fatalf("колонки не удалены: %v", cols)
	}
	if !cols["code"] || !cols["density"] {
		t.Fatalf("потеряны живые колонки: %v", cols)
	}
	var code string
	if err := db.sql.QueryRow(`SELECT code FROM materials`).Scan(&code); err != nil {
		t.Fatalf("данные не пережили DROP: %v", err)
	}
	if code != "PETG-OLD" {
		t.Fatalf("код: %q", code)
	}
}
