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
