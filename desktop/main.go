// Desktop — Wails v2: тонкие Go-биндинги поверх core/ + store-sqlite.
//
// Логики расчёта здесь нет — только вызовы core/calc и репозиториев.
// БД: app.db рядом с exe (или APP_DB), миграции встроены.
package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"3dprint-calc/core/importers"
	"3dprint-calc/core/models"
	sqlite "3dprint-calc/store-sqlite"
	"3dprint-calc/store-sqlite/seed"
)

//go:embed all:frontend/dist
var assets embed.FS

// App — корень биндингов. Методы — в app.go, backup.go, users.go, orders.go, moonraker.go.
type App struct {
	ctx              context.Context
	eventsOn         bool // true после startup: контекст настоящий wails
	db               *sqlite.DB
	dbPath           string
	currentUser      *models.User
	pendingFile      []byte // файл после preview, до apply
	pendingPath      string
	pendingCSVLimit  int                  // лимит последних работ CSV (<=0 — все)
	pendingMats      []byte               // CSV материалов после preview, до apply
	pendingMatsLimit int                  // лимит последних строк материалов (<=0 — все)
	pendingMoonraker []importers.PrintJob // строки Moonraker после preview
	pendingPrinterID int64
	pendingNew       int
	pendingExisting  int
}

func main() {
	app := &App{}
	if err := app.initDB(); err != nil {
		log.Fatalf("db: %v", err)
	}
	defer app.db.Close()

	// Размер окна — из файла настроек (до создания окна: SetSize после
	// старта Wails может игнорировать). Нет/битый — дефолт 1180×800.
	width, height := 1180, 800
	if w, err := app.GetWindowSettings(); err == nil && w.W > 0 && w.H > 0 {
		width, height = w.W, w.H
	}

	err := wails.Run(&options.App{
		Title:  "3D Print Cost Calculator",
		Width:  width,
		Height: height,
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
	a.eventsOn = true
	runtime.WindowSetTitle(ctx, fmt.Sprintf("3D Print Cost Calculator v%s (%s)", Version, buildDateLabel()))
	// Размер уже задан в options до Run; здесь только позиция из настроек.
	if w, err := a.GetWindowSettings(); err == nil && w.W > 0 && w.H > 0 {
		runtime.WindowSetPosition(ctx, w.X, w.Y)
	}
}

func (a *App) shutdown(ctx context.Context) {
	// Сохранить размер/позицию окна в файл настроек.
	if w, h := runtime.WindowGetSize(ctx); w > 0 && h > 0 {
		x, y := runtime.WindowGetPosition(ctx)
		_ = a.saveWindowSettings(WindowSettings{W: w, H: h, X: x, Y: y})
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
	a.dbPath = path
	db, err := openDB(path)
	if err != nil {
		return err
	}
	a.db = db
	return nil
}

// openDB открывает файл и накатывает миграции. Первый запуск чистый:
// только учётка admin/admin123 (seed.EnsureAdmin). Работы, материалы и принтеры
// пользователь заводит сам — импортом CSV, из Moonraker или вручную.
func openDB(path string) (*sqlite.DB, error) {
	db, err := sqlite.Open(path)
	if err != nil {
		return nil, err
	}
	if err := db.MigrateFS(migrationsFS, "assets/migrations"); err != nil {
		db.Close()
		return nil, err
	}
	// Разовый бэкфилл снапшотов импорта без price/spoolNo (импорт до fix).
	// Best-effort: ошибка не роняет старт, при следующем запуске повторится.
	_, _ = db.BackfillImportSnapshots()
	if err := seed.EnsureAdmin(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
