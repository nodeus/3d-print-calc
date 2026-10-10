package sqlite

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"3dprint-calc/core/models"
	"3dprint-calc/store-sqlite/seed"
)

// progressCounter возвращает ProgressFunc, считающий вызовы: done растёт,
// финал равен total. Используется в импортных тестах.
func progressCounter(t *testing.T, wantTotal int) (seed.ProgressFunc, *int) {
	t.Helper()
	calls := 0
	last := -1
	return func(done, total int) {
		calls++
		if total != wantTotal {
			t.Errorf("progress total = %d, хочу %d", total, wantTotal)
		}
		if done <= last {
			t.Errorf("progress не монотонен: %d после %d", done, last)
		}
		last = done
	}, &calls
}

func seedTestDB(t *testing.T, dir string) *DB {
	t.Helper()
	db, err := Open(filepath.Join(dir, "src.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate("../migrations"); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return db
}

func TestBackupRoundTrip(t *testing.T) {
	dir := t.TempDir()
	db := seedTestDB(t, dir)
	if _, err := seed.SeedAll(db, nil); err != nil {
		t.Fatalf("SeedAll: %v", err)
	}
	if _, err := db.Jobs().Upsert(models.Job{Filename: "a.gcode", StartAt: "2026-01-01 10:00", Status: "completed"}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	var buf bytes.Buffer
	if err := db.CreateBackup(&buf); err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("пустой бэкап")
	}
	m, err := InspectBackup(buf.Bytes())
	if err != nil {
		t.Fatalf("InspectBackup: %v", err)
	}
	if m.Version != BackupVersion {
		t.Errorf("version = %d", m.Version)
	}
	if m.Counts["jobs"] != 1 || m.Counts["materials"] != 15 || m.Counts["printers"] != 1 {
		t.Errorf("counts = %v", m.Counts)
	}
	wantFiles := []string{"app.db", "meta.json", "jobs.csv", "materials.csv"}
	for _, f := range wantFiles {
		found := false
		for _, got := range m.Files {
			if got == f {
				found = true
			}
		}
		if !found {
			t.Errorf("нет файла %s в %v", f, m.Files)
		}
	}

	// Битый архив обязаны отклонить.
	if _, err := InspectBackup([]byte("not a zip")); err == nil {
		t.Error("битый архив прошёл инспекцию")
	}

	// Restore в новый путь + сверка счётчиков.
	restored := filepath.Join(dir, "restored.db")
	db.Close()
	if err := RestoreAppDB(buf.Bytes(), restored); err != nil {
		t.Fatalf("RestoreAppDB: %v", err)
	}
	db2, err := Open(restored)
	if err != nil {
		t.Fatalf("Open restored: %v", err)
	}
	defer db2.Close()
	got, err := db2.TableCounts()
	if err != nil {
		t.Fatalf("TableCounts: %v", err)
	}
	if got["jobs"] != 1 || got["materials"] != 15 {
		t.Errorf("restored counts = %v", got)
	}
}

func TestJobsCSVRoundTrip(t *testing.T) {
	dir := t.TempDir()
	db := seedTestDB(t, dir)
	f, err := os.Open("../csv/print_history.csv")
	if err != nil {
		t.Skipf("нет фикстуры: %v", err)
	}
	defer f.Close()
	if _, err := seed.SeedAll(db, f); err != nil {
		t.Fatalf("SeedAll: %v", err)
	}

	var buf bytes.Buffer
	if err := db.ExportJobsCSV(&buf); err != nil {
		t.Fatalf("ExportJobsCSV: %v", err)
	}
	head := buf.String()
	if !strings.HasPrefix(head, "filename;type;status;size;start_time;end_time;") {
		t.Errorf("заголовок: %.60q", head)
	}

	// Dry-run в пустую БД: всё новое, записи нет.
	db2, err := Open(filepath.Join(dir, "empty.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	if err := db2.Migrate("../migrations"); err != nil {
		t.Fatal(err)
	}
	rep, err := db2.ImportHistory(bytes.NewReader(buf.Bytes()), true, nil)
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if !rep.DryRun || rep.New != 490 || rep.Existing != 0 {
		t.Fatalf("dry-run report = %+v, хочу {490 490 0}", rep)
	}
	if n, _ := db2.TableCounts(); n["jobs"] != 0 {
		t.Fatalf("dry-run обязан ничего не писать: %v", n)
	}
	// Apply без принтера — ошибка с понятным кодом.
	if _, err := db2.ImportHistory(bytes.NewReader(buf.Bytes()), false, nil); err == nil {
		t.Fatal("apply без принтера обязан вернуть ошибку")
	}
	// Apply с принтером.
	if _, err := db2.Printers().Save(models.Printer{Name: "K1C", HoursPerDay: 8, LifeYears: 3, CalendarMode: "calendar", AmortMethod: "calendar"}); err != nil {
		t.Fatal(err)
	}
	prog, calls := progressCounter(t, 490)
	rep2, err := db2.ImportHistory(bytes.NewReader(buf.Bytes()), false, prog)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if rep2.New != 490 {
		t.Fatalf("apply report = %+v", rep2)
	}
	if *calls == 0 {
		t.Fatal("progress ни разу не вызван")
	}
	// Повторный импорт тех же данных: всё существующее, дублей нет.
	rep3, err := db2.ImportHistory(bytes.NewReader(buf.Bytes()), true, nil)
	if err != nil {
		t.Fatalf("reimport: %v", err)
	}
	if rep3.New != 0 || rep3.Existing != 490 {
		t.Fatalf("reimport report = %+v", rep3)
	}
}

func TestMaterialsCSVExport(t *testing.T) {
	dir := t.TempDir()
	db := seedTestDB(t, dir)
	if _, err := seed.SeedAll(db, nil); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := db.ExportMaterialsCSV(&buf); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(buf.String(), "code;type;vendor;price_per_kg;currency;density;diameter") {
		t.Errorf("заголовок: %.60q", buf.String())
	}
	if n := strings.Count(buf.String(), "\n"); n != 16 { // header + 15 материалов
		t.Errorf("строк = %d, хочу 16", n)
	}
}

// TestImportMaterials: dry-run считает, apply создаёт только новые
// (существующие не трогает, цены их не меняет), повтор идемпотентен.
func TestImportMaterials(t *testing.T) {
	db := openTestDB(t)
	mats := db.Materials()
	mid, _ := mats.UpsertByCode(models.Material{Code: "PETG-OLD", Type: "PLA", Density: 1.24})
	if _, err := mats.AddSpool(models.Spool{MaterialID: mid, Price: 50000, Currency: "RUB", WeightInitG: 1000, WeightLeftG: 1000}); err != nil {
		t.Fatal(err)
	}
	raw := "code;type;vendor;price_per_kg;currency;density;diameter\n" +
		"PETG-OLD;PETG;NIT;900;RUB;1,27;1,75\n" +
		"PETG-NEW;PETG;;0;RUB;1,27;1,75\n" +
		"PETG-PRICE;PETG;V;1000;RUB;1,27;1,75\n"
	rep, err := db.ImportMaterials(strings.NewReader(raw), true)
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if rep.Total != 3 || rep.New != 2 || rep.Existing != 1 {
		t.Fatalf("dry-run: %+v", rep)
	}
	rep, err = db.ImportMaterials(strings.NewReader(raw), false)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if rep.New != 2 || rep.Existing != 1 {
		t.Fatalf("apply: %+v", rep)
	}
	// Существующий не тронут: тип PLA, цена 500.
	got, _ := mats.GetByCode("PETG-OLD")
	if got.Type != "PLA" {
		t.Fatalf("существующий перезаписан: %+v", got)
	}
	sp, _ := mats.ListSpools(mid)
	if len(sp) != 1 || sp[0].Price != 50000 {
		t.Fatalf("цена существующего: %+v", sp)
	}
	// Новый с ценой: первая катушка 1000.
	nm, err := mats.GetByCode("PETG-PRICE")
	if err != nil {
		t.Fatalf("новый: %v", err)
	}
	nsp, _ := mats.ListSpools(nm.ID)
	if len(nsp) != 1 || nsp[0].Price != 100000 || nsp[0].BoughtAt != "" {
		t.Fatalf("катушка нового: %+v", nsp)
	}
	// Новый без цены — без катушек.
	nn, _ := mats.GetByCode("PETG-NEW")
	nsp2, _ := mats.ListSpools(nn.ID)
	if len(nsp2) != 0 {
		t.Fatalf("лишние катушки: %+v", nsp2)
	}
	// Повтор: всё существующее.
	rep, err = db.ImportMaterials(strings.NewReader(raw), false)
	if err != nil {
		t.Fatalf("re-apply: %v", err)
	}
	if rep.New != 0 || rep.Existing != 3 {
		t.Fatalf("идемпотентность: %+v", rep)
	}
}

// TestImportMaterialsSpools: строки КОД-N группируются — один материал,
// катушки с явными номерами (хоть нулевыми ценами).
func TestImportMaterialsSpools(t *testing.T) {
	db := openTestDB(t)
	raw := "code;type;vendor;price_per_kg;currency;density;diameter\n" +
		"PETG-NIT-BLACK-1;PETG;;0;RUB;1,27;1,75\n" +
		"PETG-NIT-BLACK-4;PETG;НИТ;900;RUB;1,27;1,75\n" +
		"PETG-NIT-BLACK-5;PETG;;1200;RUB;1,27;1,75\n"
	rep, err := db.ImportMaterials(strings.NewReader(raw), false)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if rep.Total != 3 || rep.New != 3 || rep.Existing != 0 {
		t.Fatalf("report: %+v", rep)
	}
	m, err := db.Materials().GetByCode("PETG-NIT-BLACK")
	if err != nil {
		t.Fatalf("материал: %v", err)
	}
	if m.Vendor != "" && m.Vendor != "НИТ" {
		t.Fatalf("вендор: %+v", m)
	}
	spools, _ := db.Materials().ListSpools(m.ID)
	if len(spools) != 3 {
		t.Fatalf("катушек: %d", len(spools))
	}
	got := map[int]int64{}
	for _, s := range spools {
		got[s.SpoolNo] = s.Price
	}
	if got[1] != 0 || got[4] != 90000 || got[5] != 120000 {
		t.Fatalf("номера/цены: %+v", got)
	}
	// Мусорных материалов с номерами нет.
	if _, err := db.Materials().GetByCode("PETG-NIT-BLACK-1"); err == nil {
		t.Fatal("материал с номером в коде создан")
	}
	// Дубль номера — ошибка со строкой.
	dup := "code;type;vendor;price_per_kg;currency;density;diameter\n" +
		"PETG-X-1;PETG;;0;RUB;1,27;1,75\n" +
		"PETG-X-1;PETG;;0;RUB;1,27;1,75\n"
	if _, err := db.ImportMaterials(strings.NewReader(dup), true); err == nil {
		t.Fatal("дубль номера принят")
	}
	// Повтор: всё существующее (и номера заняты).
	rep, err = db.ImportMaterials(strings.NewReader(raw), false)
	if err != nil {
		t.Fatalf("re-apply: %v", err)
	}
	if rep.New != 0 || rep.Existing != 3 {
		t.Fatalf("идемпотентность: %+v", rep)
	}
}
