package sqlite

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"3dprint-calc/core/models"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate("../migrations"); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	// Повторная миграция обязана быть no-op (идемпотентность).
	if err := db.Migrate("../migrations"); err != nil {
		t.Fatalf("Migrate twice: %v", err)
	}
	return db
}

func TestEmptyListsSerializeAsArray(t *testing.T) {
	// Пустые выборки обязаны быть [] (не null): иначе фронт падает на .length.
	db := openTestDB(t)
	mats, _ := db.Materials().List()
	printers, _ := db.Printers().List()
	jobs, _ := db.Jobs().List(0)
	users, _ := db.Users().List()
	for name, v := range map[string]any{
		"materials": mats, "printers": printers, "jobs": jobs, "users": users,
	} {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if string(raw) != "[]" {
			t.Errorf("%s marshals as %s, хочу []", name, raw)
		}
	}
}

func TestMaterialsRoundTrip(t *testing.T) {
	db := openTestDB(t)
	repo := db.Materials()
	id, err := repo.UpsertByCode(models.Material{Code: "PETG-NIT-W4", Type: "PETG", Density: 1.27, Diameter: 1.75})
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	// Повторный upsert того же кода — обновление, дубля нет.
	id2, err := repo.UpsertByCode(models.Material{Code: "PETG-NIT-W4", Type: "PETG", Vendor: "NIT", Density: 1.27, Diameter: 1.75})
	if err != nil {
		t.Fatalf("Upsert again: %v", err)
	}
	if id != id2 {
		t.Fatalf("upsert обязан вернуть тот же id: %d != %d", id, id2)
	}
	got, err := repo.GetByCode("PETG-NIT-W4")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Vendor != "NIT" {
		t.Errorf("vendor = %q, хочу NIT", got.Vendor)
	}
	list, err := repo.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("List = %v, %v", len(list), err)
	}
	spoolID, err := repo.AddSpool(models.Spool{MaterialID: id, Price: 86900, Currency: "RUB", FxRate: 1, BoughtAt: "2025-11-25", WeightInitG: 1000, WeightLeftG: 1000})
	if err != nil || spoolID == 0 {
		t.Fatalf("AddSpool: %v, %d", err, spoolID)
	}
}

func TestPrintersRoundTrip(t *testing.T) {
	db := openTestDB(t)
	repo := db.Printers()
	id, err := repo.Save(models.Printer{
		Name: "K1C", Model: "Creality K1C", PurchasePrice: 4745400, Currency: "RUB",
		CommissioningDate: "2024-11-01", LifeYears: 3, HoursPerDay: 8,
		CalendarMode: "calendar", AmortMethod: "calendar", PowerKW: 0.35,
	})
	if err != nil || id == 0 {
		t.Fatalf("Save: %v, %d", err, id)
	}
	list, err := repo.List()
	if err != nil || len(list) != 1 || list[0].CalendarMode != "calendar" {
		t.Fatalf("List: %+v, %v", list, err)
	}
}

func TestJobsIdempotent(t *testing.T) {
	db := openTestDB(t)
	repo := db.Jobs()
	job := models.Job{
		Filename: "my_rack_PETG-PPL01_2h1m.gcode", StartAt: "2026-10-03 09:57",
		Status: "completed", PrintSec: 7024, TotalSec: 7442,
		WeightG: 59.6, LengthM: 20.14, CostSnapshot: `{"cost":100}`,
	}
	id1, err := repo.Upsert(job)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	job.Status = "cancelled"
	id2, err := repo.Upsert(job)
	if err != nil {
		t.Fatalf("Upsert again: %v", err)
	}
	if id1 != id2 {
		t.Fatalf("идемпотентность по filename+start_at: %d != %d", id1, id2)
	}
	list, err := repo.List(10)
	if err != nil || len(list) != 1 {
		t.Fatalf("List: %v, %v", len(list), err)
	}
	if list[0].Status != "cancelled" {
		t.Errorf("статус обязан обновиться: %q", list[0].Status)
	}
}
