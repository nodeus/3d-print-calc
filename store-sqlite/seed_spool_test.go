package sqlite

import (
	"encoding/json"
	"testing"
	"time"

	"3dprint-calc/core/calc"
	"3dprint-calc/core/importers"
	"3dprint-calc/core/models"
	"3dprint-calc/store-sqlite/seed"
)

// TestImportSpoolDensity: вес работы считается по физике привязанной
// катушки (её плотность/диаметр), а не материала.
func TestImportSpoolDensity(t *testing.T) {
	db := openTestDB(t)
	pid, err := db.Printers().Save(models.Printer{Name: "K1C", HoursPerDay: 8, LifeYears: 3,
		CalendarMode: "calendar", AmortMethod: "calendar"})
	if err != nil {
		t.Fatal(err)
	}
	mid, _ := db.Materials().UpsertByCode(models.Material{Code: "PETG-D", Type: "PETG", Density: 1.27})
	if _, err := db.Materials().AddSpool(models.Spool{MaterialID: mid, Price: 100000,
		Currency: "RUB", WeightInitG: 1000, WeightLeftG: 1000,
		Density: 1.30, Diameter: 1.75}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Materials().AddSpool(models.Spool{MaterialID: mid, Price: 200000,
		Currency: "RUB", WeightInitG: 1000, WeightLeftG: 1000,
		Density: 1.24, Diameter: 2.85}); err != nil {
		t.Fatal(err)
	}
	mk := func(fn string) importers.PrintJob {
		return importers.PrintJob{Filename: fn, Status: "completed",
			StartTime:     time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC),
			EndTime:       time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
			TotalDuration: 7200, FilamentUsed: 10000}
	}
	rows := []importers.PrintJob{
		mk("a_PETG-D_1h.gcode"),   // без номера → первая катушка (1.30/1.75)
		mk("b_PETG-D-2_1h.gcode"), // №2 → вторая катушка (1.24/2.85)
	}
	if _, err := seed.ImportRows(db, pid, rows, "import", nil); err != nil {
		t.Fatalf("ImportRows: %v", err)
	}
	jobs, _ := db.Jobs().List(0)
	byFile := map[string]models.Job{}
	for _, j := range jobs {
		byFile[j.Filename] = j
	}
	want1 := calc.WeightFromLength(1.30, 1.75, 10000)
	if got := byFile["a_PETG-D_1h.gcode"].WeightG; got != want1 {
		t.Fatalf("вес по первой катушке: %v, хочу %v", got, want1)
	}
	want2 := calc.WeightFromLength(1.24, 2.85, 10000)
	if got := byFile["b_PETG-D-2_1h.gcode"].WeightG; got != want2 {
		t.Fatalf("вес по второй катушке: %v, хочу %v", got, want2)
	}
}

// TestImportSpoolNumber: файл PETG-NIT-BLACK-2 линкуется ко второй катушке
// материала PETG-NIT-BLACK с её ценой; несуществующий номер — откат к первой;
// неизвестная база — стаб с полным кодом.
func TestImportSpoolNumber(t *testing.T) {
	db := openTestDB(t)
	pid, err := db.Printers().Save(models.Printer{Name: "K1C", HoursPerDay: 8, LifeYears: 3,
		CalendarMode: "calendar", AmortMethod: "calendar"})
	if err != nil {
		t.Fatal(err)
	}
	mid, _ := db.Materials().UpsertByCode(models.Material{Code: "PETG-NIT-BLACK", Type: "PETG", Density: 1.27})
	if _, err := db.Materials().AddSpool(models.Spool{MaterialID: mid, Price: 100000, Currency: "RUB", WeightInitG: 1000, WeightLeftG: 1000}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Materials().AddSpool(models.Spool{MaterialID: mid, Price: 200000, Currency: "RUB", WeightInitG: 1000, WeightLeftG: 1000}); err != nil {
		t.Fatal(err)
	}
	spools, _ := db.Materials().ListSpools(mid)
	if len(spools) != 2 || spools[0].SpoolNo != 1 || spools[1].SpoolNo != 2 {
		t.Fatalf("нумерация: %+v", spools)
	}
	mk := func(fn string) importers.PrintJob {
		return importers.PrintJob{Filename: fn, Status: "completed",
			StartTime:     time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC),
			EndTime:       time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
			TotalDuration: 7200, FilamentUsed: 10000}
	}
	rows := []importers.PrintJob{
		mk("part_PETG-NIT-BLACK-2_2h0m.gcode"),
		mk("part_PETG-NIT-BLACK-9_2h0m.gcode"), // нет такой катушки → первая
		mk("part_PETG-UNKNOWN-3_1h0m.gcode"),   // нет материала → стаб
	}
	if _, err := seed.ImportRows(db, pid, rows, "import", nil); err != nil {
		t.Fatalf("ImportRows: %v", err)
	}
	jobs, _ := db.Jobs().List(0)
	byFile := map[string]models.Job{}
	for _, j := range jobs {
		byFile[j.Filename] = j
	}
	// Вторая катушка: линка и цена 2000 ₽/кг.
	j2 := byFile["part_PETG-NIT-BLACK-2_2h0m.gcode"]
	if j2.SpoolID != spools[1].ID {
		t.Fatalf("линка: %+v, хочу spool %d", j2, spools[1].ID)
	}
	var snap struct {
		Material string             `json:"material"`
		Input    map[string]float64 `json:"input"`
	}
	if err := json.Unmarshal([]byte(j2.CostSnapshot), &snap); err != nil {
		t.Fatal(err)
	}
	if snap.Material != "PETG-NIT-BLACK" {
		t.Fatalf("материал снапшота: %q", snap.Material)
	}
	// Неизвестный номер на существующем материале: стаб катушки + линк.
	j9 := byFile["part_PETG-NIT-BLACK-9_2h0m.gcode"]
	spoolsAfter, _ := db.Materials().ListSpools(mid)
	var s9 *models.Spool
	for i, s := range spoolsAfter {
		if s.SpoolNo == 9 {
			s9 = &spoolsAfter[i]
		}
	}
	if s9 == nil {
		t.Fatal("стаб катушки №9 не создан")
	}
	if s9.Price != 0 || s9.BoughtAt != "" {
		t.Fatalf("стаб не пустой: %+v", s9)
	}
	if j9.SpoolID != s9.ID {
		t.Fatalf("линк на стаб: %+v", j9)
	}
	// Неизвестная база: стаб по базе, без номера катушки.
	if _, err := db.Materials().GetByCode("PETG-UNKNOWN"); err != nil {
		t.Fatalf("стаб не создан: %v", err)
	}
	if _, err := db.Materials().GetByCode("PETG-UNKNOWN-3"); err == nil {
		t.Fatal("стаб с номером катушки в коде запрещён")
	}
}

// TestImportSpoolStubs: свежая база — стаб материала + стаб катушки с номером,
// работа слинкована; повтор идемпотентен.
func TestImportSpoolStubs(t *testing.T) {
	db := openTestDB(t)
	pid, err := db.Printers().Save(models.Printer{Name: "K1C", HoursPerDay: 8, LifeYears: 3,
		CalendarMode: "calendar", AmortMethod: "calendar"})
	if err != nil {
		t.Fatal(err)
	}
	mk := func(fn string, start time.Time) importers.PrintJob {
		return importers.PrintJob{Filename: fn, Status: "completed",
			StartTime: start, EndTime: start.Add(2 * time.Hour),
			TotalDuration: 7200, FilamentUsed: 10000}
	}
	t0 := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	rows := []importers.PrintJob{
		mk("a_PETG-NIT-BLACK-2_2h0m.gcode", t0),
		mk("b_PETG-NIT-BLACK-2_3h0m.gcode", t0.Add(time.Hour)),
	}
	st, err := seed.ImportRows(db, pid, rows, "import", nil)
	if err != nil {
		t.Fatalf("ImportRows: %v", err)
	}
	if st.Stubs != 1 || st.SpoolStubs != 1 {
		t.Fatalf("статистика: %+v", st)
	}
	m, err := db.Materials().GetByCode("PETG-NIT-BLACK")
	if err != nil {
		t.Fatalf("материал: %v", err)
	}
	spools, _ := db.Materials().ListSpools(m.ID)
	if len(spools) != 1 || spools[0].SpoolNo != 2 {
		t.Fatalf("катушки: %+v", spools)
	}
	jobs, _ := db.Jobs().List(0)
	if len(jobs) != 2 {
		t.Fatalf("работы: %d", len(jobs))
	}
	for _, j := range jobs {
		if j.SpoolID != spools[0].ID {
			t.Fatalf("линк: %+v", j)
		}
	}
	// Повтор: новых нет, стабы не дублируются.
	st2, err := seed.ImportRows(db, pid, rows, "import", nil)
	if err != nil {
		t.Fatalf("re-import: %v", err)
	}
	if st2.Jobs != 2 || st2.SpoolStubs != 0 {
		t.Fatalf("идемпотентность: %+v", st2)
	}
	spools2, _ := db.Materials().ListSpools(m.ID)
	if len(spools2) != 1 {
		t.Fatalf("дубль катушки: %+v", spools2)
	}
}
