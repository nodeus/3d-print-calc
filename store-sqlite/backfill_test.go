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

// TestBackfillImportSnapshots: старые импортные снапшоты без price/spoolNo
// дозаполняются; правленые (edited=1) и заполненные не трогаются; повтор — no-op.
func TestBackfillImportSnapshots(t *testing.T) {
	db := openTestDB(t)
	mats := db.Materials()
	mid, err := mats.UpsertByCode(models.Material{Code: "PETG-B", Type: "PETG", Density: 1.27})
	if err != nil {
		t.Fatal(err)
	}
	sid, err := mats.AddSpool(models.Spool{MaterialID: mid, Price: 100000, Currency: "RUB",
		WeightInitG: 1000, WeightLeftG: 1000})
	if err != nil {
		t.Fatal(err)
	}
	spools, _ := mats.ListSpools(mid)
	if len(spools) != 1 || spools[0].SpoolNo != 1 {
		t.Fatalf("нумерация: %+v", spools)
	}
	oldSnap := `{"v":2,"material":"PETG-B","weight_g":10,"cost":123.45,"currency":"RUB",` +
		`"input":{"tariff":5,"powerKW":0.35,"amortPerHour":1,"opRate":0,"prepH":0,"postH":0,` +
		`"designerRate":0,"modelH":0,"modelPrice":0,"extraMat":0,"failRate":0,` +
		`"marginProfit":0,"marginDev":0,"fees":0,"maintPerHour":0,"purgeWasteG":0,` +
		`"copies":1,"vatRate":0,"roundStep":0}}`
	mk := func(fn string) int64 {
		id, err := db.Jobs().Upsert(models.Job{Filename: fn, StartAt: "2026-10-01 10:00",
			Status: "completed", TotalSec: 3600, WeightG: 10, SpoolID: sid,
			Origin: "import", CostSnapshot: oldSnap})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	plainID := mk("old.gcode")
	editedID := mk("edited.gcode")
	if _, err := db.sql.Exec(`UPDATE jobs SET edited=1 WHERE id=?`, editedID); err != nil {
		t.Fatal(err)
	}
	fullID := mk("full.gcode")
	if _, err := db.sql.Exec(`UPDATE jobs SET cost_snapshot=json_set(cost_snapshot,
		'$.price', 999, '$.input.spoolNo', 7) WHERE id=?`, fullID); err != nil {
		t.Fatal(err)
	}

	n, err := db.BackfillImportSnapshots()
	if err != nil {
		t.Fatalf("Backfill: %v", err)
	}
	if n != 1 {
		t.Fatalf("починено %d, хочу 1", n)
	}

	get := func(id int64) map[string]any {
		var raw string
		if err := db.sql.QueryRow(`SELECT cost_snapshot FROM jobs WHERE id=?`, id).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(raw), &obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	// Нетронутая: price пересчитана тем же calc по сохранённому input
	// и цене привязанной катушки (1000 ₽/кг); cost не тронут.
	obj := get(plainID)
	want := calc.Calculate(calc.Input{TotalSec: 3600, WeightG: 10, PricePerKg: 1000,
		Tariff: 5, PowerKW: 0.35, AmortPerHour: 1, Copies: 1})
	if obj["price"] != want.PriceVAT {
		t.Fatalf("price=%v, хочу %v", obj["price"], want.PriceVAT)
	}
	if obj["cost"] != 123.45 {
		t.Fatalf("cost тронут: %v", obj["cost"])
	}
	inp := obj["input"].(map[string]any)
	if inp["spoolNo"] != 1.0 {
		t.Fatalf("spoolNo=%v", inp["spoolNo"])
	}
	// Правленая без price — не тронута.
	if _, ok := get(editedID)["price"]; ok {
		t.Fatal("edited=1 изменена бэкфиллом")
	}
	// Заполненная — не тронута.
	full := get(fullID)
	if full["price"] != 999.0 || full["input"].(map[string]any)["spoolNo"] != 7.0 {
		t.Fatalf("заполненная изменена: %v", full)
	}
	// Повтор — no-op.
	n, err = db.BackfillImportSnapshots()
	if err != nil {
		t.Fatalf("Backfill twice: %v", err)
	}
	if n != 0 {
		t.Fatalf("повтор починил %d", n)
	}
}

// TestImportSnapshotHasPriceAndSpoolNo: свежий импорт пишет price и spoolNo:
// номер из имени, номер первой катушки без номера, номер стаба.
func TestImportSnapshotHasPriceAndSpoolNo(t *testing.T) {
	db := openTestDB(t)
	pid, err := db.Printers().Save(models.Printer{Name: "K1C", HoursPerDay: 8, LifeYears: 3,
		CalendarMode: "calendar", AmortMethod: "calendar"})
	if err != nil {
		t.Fatal(err)
	}
	mid, _ := db.Materials().UpsertByCode(models.Material{Code: "PETG-NIT-BLACK", Type: "PETG", Density: 1.27})
	for _, p := range []int64{100000, 200000} {
		if _, err := db.Materials().AddSpool(models.Spool{MaterialID: mid, Price: p,
			Currency: "RUB", WeightInitG: 1000, WeightLeftG: 1000}); err != nil {
			t.Fatal(err)
		}
	}
	mk := func(fn string) importers.PrintJob {
		return importers.PrintJob{Filename: fn, Status: "completed",
			StartTime:     time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC),
			EndTime:       time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
			TotalDuration: 7200, FilamentUsed: 10000}
	}
	rows := []importers.PrintJob{
		mk("part_PETG-NIT-BLACK-2_2h0m.gcode"),
		mk("part_PETG-NIT-BLACK_2h0m.gcode"),
		mk("part_PETG-NIT-BLACK-9_2h0m.gcode"),
	}
	if _, err := seed.ImportRows(db, pid, rows, "import", nil); err != nil {
		t.Fatalf("ImportRows: %v", err)
	}
	jobs, _ := db.Jobs().List(0)
	byFile := map[string]models.Job{}
	for _, j := range jobs {
		byFile[j.Filename] = j
	}
	type snap struct {
		Cost  float64            `json:"cost"`
		Price float64            `json:"price"`
		Input map[string]float64 `json:"input"`
	}
	check := func(fn string, wantSpoolNo float64) {
		var s snap
		if err := json.Unmarshal([]byte(byFile[fn].CostSnapshot), &s); err != nil {
			t.Fatalf("%s: %v", fn, err)
		}
		if s.Price <= 0 || s.Price != s.Cost {
			t.Fatalf("%s: price=%v cost=%v", fn, s.Price, s.Cost)
		}
		if s.Input["spoolNo"] != wantSpoolNo {
			t.Fatalf("%s: spoolNo=%v, хочу %v", fn, s.Input["spoolNo"], wantSpoolNo)
		}
	}
	check("part_PETG-NIT-BLACK-2_2h0m.gcode", 2)
	check("part_PETG-NIT-BLACK_2h0m.gcode", 1)
	check("part_PETG-NIT-BLACK-9_2h0m.gcode", 9)
}
