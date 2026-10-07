// Command seed — dev-инструмент: полный сид app.db legacy-данными
// (тонкий CLI поверх store-sqlite/seed). Десктоп при первом запуске
// создаёт только admin (чистый старт). Идемпотентно — повтор безопасен.
//
// Запуск из корня репо:
//
//	go run ./tools/seed -db app.db
package main

import (
	"flag"
	"fmt"
	"os"

	sqlite "3dprint-calc/store-sqlite"
	"3dprint-calc/store-sqlite/seed"
)

func main() {
	dbPath := flag.String("db", "app.db", "путь к SQLite-файлу")
	csvPath := flag.String("csv", "csv/print_history.csv", "история печати принтера")
	migDir := flag.String("migrations", "migrations", "каталог миграций")
	flag.Parse()
	if err := run(*dbPath, *csvPath, *migDir); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
}

func run(dbPath, csvPath, migDir string) error {
	db, err := sqlite.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.Migrate(migDir); err != nil {
		return err
	}
	f, err := os.Open(csvPath)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := seed.SeedAll(db, f)
	if err != nil {
		return err
	}
	fmt.Printf("seed ok: db=%s materials=%d jobs=%d linked=%d stubs=%d\n",
		dbPath, st.Materials, st.Jobs, st.Linked, st.Stubs)
	return nil
}
