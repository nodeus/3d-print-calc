package main

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"3dprint-calc/core/calc"
)

// TestAppCleanStart: первый запуск чистый — openDB даёт только admin,
// справочники и работы пусты, базовые биндинги работают.
func TestAppCleanStart(t *testing.T) {
	db, err := openDB(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatalf("openDB: %v", err)
	}
	defer db.Close()
	app := &App{db: db, dbPath: filepath.Join(t.TempDir(), "unused.db")}
	if _, err := app.Login("admin", "admin123"); err != nil {
		t.Fatalf("Login admin: %v", err)
	}
	mats, err := app.GetMaterials()
	if err != nil || len(mats) != 0 {
		t.Fatalf("materials обязаны быть пусты: %d, %v", len(mats), err)
	}
	printers, err := app.GetPrinters()
	if err != nil || len(printers) != 0 {
		t.Fatalf("printers обязаны быть пусты: %d, %v", len(printers), err)
	}
	jobs, err := app.GetJobs(0)
	if err != nil || len(jobs) != 0 {
		t.Fatalf("jobs обязаны быть пусты: %d, %v", len(jobs), err)
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
	if _, err := app.Login("admin", "admin123"); err != nil {
		t.Fatalf("Login: %v", err)
	}
	// Одна ручная работа — проверять restore есть на чём.
	if _, err := app.SaveJob(JobInput{Filename: "probe.gcode", TotalSec: 60, Cost: 1, Price: 2, Currency: "RUB"}); err != nil {
		t.Fatalf("SaveJob: %v", err)
	}

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
	if m.Counts["jobs"] != 1 {
		t.Fatalf("после restore jobs = %v", m.Counts["jobs"])
	}
	// Соединение живое после переоткрытия.
	jobs, err := app.GetJobs(1)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("GetJobs после reopen: %v", len(jobs))
	}
	app.db.Close()
}

// TestRoles проверяет gate ролей: гость/вьювер не пишут, менеджер пишет,
// неверный пароль не пускает.
func TestRoles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.db")
	db, err := openDB(path)
	if err != nil {
		t.Fatalf("openDB: %v", err)
	}
	defer db.Close()
	app := &App{db: db, dbPath: path}

	if _, err := app.Login("admin", "bad"); err == nil {
		t.Fatal("неверный пароль пустил")
	}
	if _, err := app.SaveJob(JobInput{Filename: "x.gcode"}); err == nil {
		t.Fatal("гость пишет без логина")
	}
	if _, err := app.Login("admin", "admin123"); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if _, err := app.CreateUser("viewer1", "pass1234", "viewer"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	app.Logout()
	if _, err := app.Login("viewer1", "pass1234"); err != nil {
		t.Fatalf("Login viewer: %v", err)
	}
	if _, err := app.SaveJob(JobInput{Filename: "x.gcode"}); err == nil {
		t.Fatal("viewer пишет")
	}
	if _, err := app.CreateUser("z", "pass1234", "viewer"); err == nil {
		t.Fatal("viewer создаёт пользователей")
	}
	// Чтение вьюверу разрешено.
	if _, err := app.GetJobs(1); err != nil {
		t.Fatalf("viewer чтение: %v", err)
	}
	if err := app.ChangeMyPassword("pass1234", "newpass1"); err != nil {
		t.Fatalf("ChangeMyPassword: %v", err)
	}
	app.Logout()
	if _, err := app.Login("viewer1", "newpass1"); err != nil {
		t.Fatalf("Login новым паролем: %v", err)
	}
}

// TestRolesDeleteGuards: нельзя удалить себя и последнего админа.
func TestRolesDeleteGuards(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.db")
	db, err := openDB(path)
	if err != nil {
		t.Fatalf("openDB: %v", err)
	}
	defer db.Close()
	app := &App{db: db, dbPath: path}
	me, err := app.Login("admin", "admin123")
	if err != nil {
		t.Fatal(err)
	}
	if err := app.DeleteUser(me.ID); err == nil {
		t.Fatal("самоудаление обязано блокироваться")
	}
	// Второй админ — удаление первого разрешено.
	other, err := app.CreateUser("admin2", "pass1234", "admin")
	if err != nil {
		t.Fatal(err)
	}
	app.Logout()
	if _, err := app.Login("admin2", "pass1234"); err != nil {
		t.Fatal(err)
	}
	if err := app.DeleteUser(me.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	// Теперь admin2 — последний админ: самоудаление всё равно запрещено (себя).
	if err := app.DeleteUser(other); err == nil {
		t.Fatal("самоудаление последнего админа обязано блокироваться")
	}
}

// TestMaterialColorFavs: фавориты цветов round-trip, мусор отфильтровывается.
func TestMaterialColorFavs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.db")
	db, err := openDB(path)
	if err != nil {
		t.Fatalf("openDB: %v", err)
	}
	defer db.Close()
	app := &App{db: db, dbPath: path}
	if got := app.GetMaterialColorFavs(); len(got) != 0 {
		t.Fatalf("пусто: %v", got)
	}
	if err := app.SetMaterialColorFavs([]string{"#FF0000"}); err == nil {
		t.Fatal("гость пишет фавориты")
	}
	if _, err := app.Login("admin", "admin123"); err != nil {
		t.Fatal(err)
	}
	if err := app.SetMaterialColorFavs([]string{"#FF0000", "мусор", "#00c000", "#FF0000"}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got := app.GetMaterialColorFavs()
	if len(got) != 2 || got[0] != "#ff0000" || got[1] != "#00c000" {
		t.Fatalf("favs = %v", got)
	}
}
