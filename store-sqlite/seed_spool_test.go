package sqlite

import (
	"encoding/json"
	"testing"
	"time"

	"3dprint-calc/core/importers"
	"3dprint-calc/core/models"
	"3dprint-calc/store-sqlite/seed"
)

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
	// Третья катушка отсутствует: первая + её цена.
	j9 := byFile["part_PETG-NIT-BLACK-9_2h0m.gcode"]
	if j9.SpoolID != spools[0].ID {
		t.Fatalf("фолбэк: %+v", j9)
	}
	// Неизвестная база: стаб по базе, без номера катушки.
	if _, err := db.Materials().GetByCode("PETG-UNKNOWN"); err != nil {
		t.Fatalf("стаб не создан: %v", err)
	}
	if _, err := db.Materials().GetByCode("PETG-UNKNOWN-3"); err == nil {
		t.Fatal("стаб с номером катушки в коде запрещён")
	}
}
