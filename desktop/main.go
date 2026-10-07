// Desktop — Wails v2: тонкие Go-биндинги поверх core/ + store-sqlite.
//
// Логики расчёта здесь нет — только вызовы core/calc и репозиториев.
// БД: app.db рядом с exe (или APP_DB), миграции и CSV-сид встроены.
package main

import (
	"bytes"
	"context"
	"embed"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"3dprint-calc/core/models"
	sqlite "3dprint-calc/store-sqlite"
	"3dprint-calc/store-sqlite/seed"
)

//go:embed all:frontend/dist
var assets embed.FS

// App — корень биндингов. Методы — в app.go, backup.go, users.go, orders.go.
type App struct {
	ctx         context.Context
	db          *sqlite.DB
	dbPath      string
	currentUser *models.User
	pendingFile []byte // файл после preview, до apply
	pendingPath string
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
		Bind:       []any{app},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
	})
	if err != nil {
		log.Fatal(err)
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(ctx context.Context) {
	_ = ctx
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
	a.dbPath = path
	db, err := openDB(path)
	if err != nil {
		return err
	}
	a.db = db
	return nil
}

// openDB открывает файл, накатывает миграции и сеет пустую БД.
func openDB(path string) (*sqlite.DB, error) {
	db, err := sqlite.Open(path)
	if err != nil {
		return nil, err
	}
	if err := db.MigrateFS(migrationsFS, "assets/migrations"); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := seed.SeedAll(db, bytes.NewReader(seedCSV)); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
