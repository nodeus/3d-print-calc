// Desktop — Wails v2: тонкие Go-биндинги поверх core/ + store-sqlite.
//
// Логики расчёта здесь нет — только вызовы core/calc и репозиториев.
// БД: app.db рядом с exe (или APP_DB), миграции и CSV-сид встроены.
package main

import (
	"bytes"
	"embed"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	sqlite "3dprint-calc/store-sqlite"
	"3dprint-calc/store-sqlite/seed"
)

//go:embed all:frontend/dist
var assets embed.FS

// App — корень биндингов. Методы — в app.go.
type App struct {
	db *sqlite.DB
}

func main() {
	app := &App{}
	if err := app.initDB(); err != nil {
		log.Fatalf("db: %v", err)
	}
	defer app.db.Close()

	err := wails.Run(&options.App{
		Title:  "3D Print Cost Calculator",
		Width:  1180,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Bind: []any{app},
	})
	if err != nil {
		log.Fatal(err)
	}
}

// initDB открывает app.db, накатывает миграции и сеет при первом запуске.
func (a *App) initDB() error {
	path := os.Getenv("APP_DB")
	if path == "" {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		path = filepath.Join(filepath.Dir(exe), "app.db")
	}
	db, err := sqlite.Open(path)
	if err != nil {
		return err
	}
	a.db = db
	if err := db.MigrateFS(migrationsFS, "assets/migrations"); err != nil {
		return err
	}
	if _, err := seed.SeedAll(db, bytes.NewReader(seedCSV)); err != nil {
		return err
	}
	return nil
}
