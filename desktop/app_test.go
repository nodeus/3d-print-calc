package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"3dprint-calc/core/calc"
	"3dprint-calc/core/importers"
	"3dprint-calc/core/models"
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

// TestBaseThemes: встроенные light/dark содержат все 18 переменных валидным
// hex — экспорт в файл работает и для базовых тем (зеркало theme.css).
func TestBaseThemes(t *testing.T) {
	want := []string{
		"bg", "fg", "card-border", "nav-bg", "header-bg", "header-fg",
		"btn-bg", "btn-fg", "btn-border", "input-bg", "input-fg",
		"muted", "err", "warn", "accent", "sel-bg", "final-border", "sw-border",
	}
	for _, name := range []string{"light", "dark"} {
		vars, ok := baseThemes[name]
		if !ok {
			t.Fatalf("нет базовой темы %q", name)
		}
		if len(vars) != len(want) {
			t.Fatalf("%s: %d vars, хочу %d", name, len(vars), len(want))
		}
		for _, k := range want {
			v, ok := vars[k]
			if !ok {
				t.Fatalf("%s: нет ключа %q", name, k)
			}
			if !hexColor.MatchString(v) {
				t.Fatalf("%s.%s = %q: не hex", name, k, v)
			}
		}
	}
}
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

// TestPayStateOf: нулевой итог без оплат — unknown, не paid.
func TestPayStateOf(t *testing.T) {
	for _, tc := range []struct {
		total, paid int64
		want        string
	}{
		{0, 0, "unknown"},
		{0, 100, "paid"},
		{10000, 0, "unpaid"},
		{10000, 4000, "partial"},
		{10000, 10000, "paid"},
		{10000, 15000, "paid"},
	} {
		if got := PayStateOf(tc.total, tc.paid); got != tc.want {
			t.Fatalf("PayStateOf(%d,%d) = %q, хочу %q", tc.total, tc.paid, got, tc.want)
		}
	}
}

// TestSaveJobSpoolNo: явная катушка линкуется, номер пишется в снапшот.
func TestSaveJobSpoolNo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.db")
	db, err := openDB(path)
	if err != nil {
		t.Fatalf("openDB: %v", err)
	}
	defer db.Close()
	app := &App{db: db, dbPath: path}
	if _, err := app.Login("admin", "admin123"); err != nil {
		t.Fatalf("Login: %v", err)
	}
	mid, err := db.Materials().UpsertByCode(models.Material{Code: "PETG-T", Type: "PETG", Density: 1.27, Diameter: 1.75})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Materials().AddSpool(models.Spool{MaterialID: mid, Price: 100000, Currency: "RUB", WeightInitG: 1000, WeightLeftG: 1000}); err != nil {
		t.Fatal(err)
	}
	s2, err := db.Materials().AddSpool(models.Spool{MaterialID: mid, Price: 200000, Currency: "RUB", WeightInitG: 1000, WeightLeftG: 1000})
	if err != nil {
		t.Fatal(err)
	}
	id, err := app.SaveJob(JobInput{Filename: "s.gcode", TotalSec: 60, WeightG: 10,
		Cost: 1, Price: 2, Currency: "RUB", Material: "PETG-T", SpoolID: s2})
	if err != nil {
		t.Fatalf("SaveJob: %v", err)
	}
	jobs, _ := app.GetJobs(0)
	var got models.Job
	for _, j := range jobs {
		if j.ID == id {
			got = j
		}
	}
	if got.SpoolID != s2 {
		t.Fatalf("spool_id = %d, хочу %d", got.SpoolID, s2)
	}
	var snap struct {
		Input map[string]float64 `json:"input"`
	}
	if err := json.Unmarshal([]byte(got.CostSnapshot), &snap); err != nil {
		t.Fatal(err)
	}
	if snap.Input["spoolNo"] != 2 {
		t.Fatalf("spoolNo: %+v", snap.Input)
	}
}

// TestSpoolLeftComputed: остаток катушки вычисляемый (партия минус расход
// по истории), а не хранимое weight_left_g. Работа с явным spool_id идёт
// на свою катушку, без линка — на первую живую того же материала,
// перерасход ограничен нулём.
func TestSpoolLeftComputed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.db")
	db, err := openDB(path)
	if err != nil {
		t.Fatalf("openDB: %v", err)
	}
	defer db.Close()
	app := &App{db: db, dbPath: path}
	mid, err := db.Materials().UpsertByCode(models.Material{Code: "PETG-L", Type: "PETG", Density: 1.27})
	if err != nil {
		t.Fatal(err)
	}
	s1, err := db.Materials().AddSpool(models.Spool{MaterialID: mid, Price: 100000, Currency: "RUB", WeightInitG: 1000, WeightLeftG: 1000})
	if err != nil {
		t.Fatal(err)
	}
	s2, err := db.Materials().AddSpool(models.Spool{MaterialID: mid, Price: 100000, Currency: "RUB", WeightInitG: 1000, WeightLeftG: 1000})
	if err != nil {
		t.Fatal(err)
	}
	addJob := func(fn string, spoolID int64, weight float64, snap string) {
		if _, err := db.Jobs().Upsert(models.Job{Filename: fn, StartAt: "2026-10-01 10:00",
			Status: "completed", TotalSec: 60, WeightG: weight, SpoolID: spoolID,
			Origin: "import", CostSnapshot: snap}); err != nil {
			t.Fatal(err)
		}
	}
	addJob("a.gcode", s1, 60, `{"v":2,"material":"PETG-L","cost":1}`)
	addJob("b.gcode", 0, 40, `{"v":2,"material":"PETG-L","cost":1}`)
	addJob("c.gcode", s2, 1200, `{"v":2,"material":"PETG-L","cost":1}`)
	mats, err := app.GetMaterials()
	if err != nil || len(mats) != 1 {
		t.Fatalf("GetMaterials: %v %d", err, len(mats))
	}
	byID := map[int64]SpoolDTO{}
	for _, s := range mats[0].Spools {
		byID[s.ID] = s
	}
	if got := byID[s1].LeftG; got != 900 {
		t.Fatalf("s1 leftG = %v, хочу 900 (60 своих + 40 без линка)", got)
	}
	if got := byID[s2].LeftG; got != 0 {
		t.Fatalf("s2 leftG = %v, хочу 0 (перерасход 1200 при партии 1000)", got)
	}
	spools, err := app.GetSpools(mid)
	if err != nil || len(spools) != 2 {
		t.Fatalf("GetSpools: %v %d", err, len(spools))
	}
	for _, s := range spools {
		if want := byID[s.ID].LeftG; s.LeftG != want {
			t.Fatalf("GetSpools leftG = %v, GetMaterials дал %v", s.LeftG, want)
		}
	}
}

// TestAddMaterialFullFields: создание со всеми полями, без цены и катушек.
// Цена живёт только на катушках: материал создаётся пустым.
func TestAddMaterialFullFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.db")
	db, err := openDB(path)
	if err != nil {
		t.Fatalf("openDB: %v", err)
	}
	defer db.Close()
	app := &App{db: db, dbPath: path}
	if _, err := app.Login("admin", "admin123"); err != nil {
		t.Fatal(err)
	}
	id, err := app.AddMaterial("PETG-NEW", "PETG", 1.27, 1000, "NIT", "#ff0000", "тестовая заметка")
	if err != nil || id == 0 {
		t.Fatalf("AddMaterial: %v %d", id, err)
	}
	m, err := db.Materials().GetByCode("PETG-NEW")
	if err != nil {
		t.Fatal(err)
	}
	if m.Vendor != "NIT" || m.ColorHex != "#ff0000" || m.Note != "тестовая заметка" {
		t.Fatalf("поля: %+v", m)
	}
	if m.Density != 1.27 || m.DefaultSpoolWeightG != 1000 || m.Type != "PETG" {
		t.Fatalf("физика: %+v", m)
	}
	spools, err := db.Materials().ListSpools(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(spools) != 0 {
		t.Fatalf("катушки созданы без цены: %+v", spools)
	}
	mats, err := app.GetMaterials()
	if err != nil {
		t.Fatal(err)
	}
	var dto MaterialDTO
	for _, x := range mats {
		if x.Code == "PETG-NEW" {
			dto = x
		}
	}
	if dto.ID == 0 || dto.Note != "тестовая заметка" || dto.PricePerKg != 0 || dto.HasLiveSpool {
		t.Fatalf("DTO: %+v", dto)
	}
	if dto.AvgPricePerKg != 0 {
		t.Fatalf("среднее без катушек: %+v", dto)
	}
}

// TestMaterialAvgPrice: витринное среднее — простое по ВСЕМ катушкам
// (включая архивную); расчётная PricePerKg не меняется.
func TestMaterialAvgPrice(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.db")
	db, err := openDB(path)
	if err != nil {
		t.Fatalf("openDB: %v", err)
	}
	defer db.Close()
	app := &App{db: db, dbPath: path}
	if _, err := app.Login("admin", "admin123"); err != nil {
		t.Fatal(err)
	}
	id, err := app.AddMaterial("PETG-AVG", "PETG", 1.27, 1000, "", "", "")
	if err != nil || id == 0 {
		t.Fatalf("AddMaterial: %v %d", id, err)
	}
	for _, s := range []models.Spool{
		{MaterialID: id, Price: 100000, Currency: "RUB", WeightInitG: 1000, WeightLeftG: 1000},
		{MaterialID: id, Price: 200000, Currency: "RUB", WeightInitG: 1000, WeightLeftG: 1000},
		{MaterialID: id, Price: 300000, Currency: "RUB", WeightInitG: 1000, WeightLeftG: 1000, Archived: true},
	} {
		if _, err := db.Materials().AddSpool(s); err != nil {
			t.Fatal(err)
		}
	}
	mats, err := app.GetMaterials()
	if err != nil {
		t.Fatal(err)
	}
	var dto MaterialDTO
	for _, x := range mats {
		if x.Code == "PETG-AVG" {
			dto = x
		}
	}
	if dto.ID == 0 {
		t.Fatal("материал не найден")
	}
	if dto.AvgPricePerKg != 2000 {
		t.Fatalf("среднее: %v, хочу 2000", dto.AvgPricePerKg)
	}
	if dto.PricePerKg != 1000 {
		t.Fatalf("расчётная цена тронута: %v", dto.PricePerKg)
	}
}

// TestMaterialsApplySelectedCancel: применение только выбранных строк
// материалов; отмена сбрасывает pendingMats (дальше — ERR_NO_PENDING).
func TestMaterialsApplySelectedCancel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.db")
	db, err := openDB(path)
	if err != nil {
		t.Fatalf("openDB: %v", err)
	}
	defer db.Close()
	app := &App{ctx: context.Background(), db: db, dbPath: path}
	if _, err := app.Login("admin", "admin123"); err != nil {
		t.Fatal(err)
	}
	raw := "code;type;vendor;price_per_kg;currency;density;diameter\n" +
		"PETG-A;PETG;;100;RUB;1,27;1,75\n" +
		"PETG-B;PETG;;200;RUB;1,27;1,75\n"
	app.pendingMats = []byte(raw)
	app.pendingMatsLimit = 0
	applied, err := app.ApplyMaterialsCSV([]int{1})
	if err != nil {
		t.Fatalf("Apply selected: %v", err)
	}
	if applied.New != 1 {
		t.Fatalf("applied = %+v", applied)
	}
	mats, err := app.GetMaterials()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, m := range mats {
		got[m.Code] = true
	}
	if got["PETG-A"] || !got["PETG-B"] {
		t.Fatalf("созданы: %v", got)
	}
	// Отмена после нового preview-буфера: apply невозможен.
	app.pendingMats = []byte(raw)
	app.CancelPendingImport()
	if len(app.pendingMats) != 0 {
		t.Fatal("pendingMats не сброшен")
	}
	if _, err := app.ApplyMaterialsCSV(nil); err == nil {
		t.Fatal("apply после отмены принят")
	}
}

// TestPendingGuardSequential: предпросмотр при занятом чужом буфере —
// ERR_PENDING_EXISTS; повторный preview своего типа не блокируется.
func TestPendingGuardSequential(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.db")
	db, err := openDB(path)
	if err != nil {
		t.Fatalf("openDB: %v", err)
	}
	defer db.Close()
	app := &App{ctx: context.Background(), db: db, dbPath: path}
	if _, err := app.Login("admin", "admin123"); err != nil {
		t.Fatal(err)
	}
	// История pending → материалы и Moonraker заблокированы.
	app.pendingFile = []byte("x")
	if _, err := app.PreviewMaterialsCSV(0); err == nil ||
		!strings.Contains(err.Error(), "ERR_PENDING_EXISTS") {
		t.Fatalf("материалы при истории: %v", err)
	}
	if _, err := app.MoonrakerPreview(1, 0); err == nil ||
		!strings.Contains(err.Error(), "ERR_PENDING_EXISTS") {
		t.Fatalf("moonraker при истории: %v", err)
	}
	// Свой тип не блокируется: Moonraker с занятым своим буфером доходит
	// до поиска принтера (в пустой БД — ERR_PRINTER_NOT_FOUND, не guard).
	// (CSV-диалоги в headless-тесте не вызываем: runtime роняет процесс.)
	app.pendingFile = nil
	app.pendingMoonraker = []importers.PrintJob{{Filename: "x"}}
	if _, err := app.MoonrakerPreview(1, 0); err == nil ||
		strings.Contains(err.Error(), "ERR_PENDING_EXISTS") {
		t.Fatalf("повтор moonraker: %v", err)
	}
	app.pendingMoonraker = nil
	// Материалы pending → история и Moonraker заблокированы.
	app.pendingMats = []byte("x")
	if _, err := app.MoonrakerPreview(1, 0); err == nil ||
		!strings.Contains(err.Error(), "ERR_PENDING_EXISTS") {
		t.Fatalf("moonraker при материалах: %v", err)
	}
	// Отмена открывает всё.
	app.CancelPendingImport()
	if _, err := app.MoonrakerPreview(1, 0); err == nil ||
		strings.Contains(err.Error(), "ERR_PENDING_EXISTS") {
		t.Fatalf("после отмены: %v", err)
	}
}
