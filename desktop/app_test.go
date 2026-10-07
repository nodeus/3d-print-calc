package main

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"3dprint-calc/core/calc"
	sqlite "3dprint-calc/store-sqlite"
	"3dprint-calc/store-sqlite/seed"
)

// TestAppInit повторяет initDB на временной БД: миграции из embed,
// сид из встроенного CSV, базовые биндинги.
func TestAppInit(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()
	if err := db.MigrateFS(migrationsFS, "assets/migrations"); err != nil {
		t.Fatalf("MigrateFS: %v", err)
	}
	st, err := seed.SeedAll(db, bytes.NewReader(seedCSV))
	if err != nil {
		t.Fatalf("SeedAll: %v", err)
	}
	if st.Jobs != 490 {
		t.Fatalf("jobs = %d, хочу 490", st.Jobs)
	}
	app := &App{db: db}
	mats, err := app.GetMaterials()
	if err != nil || len(mats) < 15 {
		t.Fatalf("materials: %d, %v", len(mats), err)
	}
	printers, err := app.GetPrinters()
	if err != nil || len(printers) != 1 {
		t.Fatalf("printers: %+v, %v", printers, err)
	}
	if printers[0].AmortPerHour < 5 || printers[0].AmortPerHour > 6 {
		t.Fatalf("amort K1C = %v, хочу ~5.41", printers[0].AmortPerHour)
	}
	got := app.Calc(calc.Input{TotalSec: 3600, WeightG: 100, PricePerKg: 1000})
	if got.MatCost != 100 {
		t.Fatalf("Calc mat = %v", got.MatCost)
	}
	id, err := app.SaveJob(JobInput{Filename: "test.gcode", TotalSec: 3600, WeightG: 100, Cost: 1, Price: 2, Currency: "RUB"})
	if err != nil || id == 0 {
		t.Fatalf("SaveJob: %v, %d", err, id)
	}
	if loc := app.GetLocale(); loc != "ru" {
		t.Fatalf("locale default = %q", loc)
	}
	if err := app.SetLocale("en"); err != nil || app.GetLocale() != "en" {
		t.Fatalf("locale switch: %v", err)
	}
}

// TestBackupReopen проверяет ApplyBackup: закрытие, замена файла, переоткрытие.
func TestBackupReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.db")
	db, err := openDB(path)
	if err != nil {
		t.Fatalf("openDB: %v", err)
	}
	app := &App{ctx: context.Background(), db: db, dbPath: path}

	var buf bytes.Buffer
	if err := db.CreateBackup(&buf); err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}
	if _, err := app.ApplyBackup(); err == nil {
		t.Fatal("ApplyBackup без preview обязан вернуть ERR_NO_PENDING")
	}
	app.pendingFile = buf.Bytes()
	m, err := app.ApplyBackup()
	if err != nil {
		t.Fatalf("ApplyBackup: %v", err)
	}
	if m.Counts["jobs"] != 490 {
		t.Fatalf("после restore jobs = %v", m.Counts["jobs"])
	}
	// Соединение живое после переоткрытия.
	jobs, err := app.GetJobs(1)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("GetJobs после reopen: %v", len(jobs))
	}
	app.db.Close()
}
