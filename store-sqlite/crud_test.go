package sqlite

import (
	"encoding/json"
	"testing"

	"3dprint-calc/core/models"
)

func TestMaterialCRUDGuards(t *testing.T) {
	db := openTestDB(t)
	repo := db.Materials()
	id, err := repo.UpsertByCode(models.Material{Code: "PETG-T1", Type: "PETG", Density: 1.27, Diameter: 1.75})
	if err != nil {
		t.Fatal(err)
	}
	// Update меняет всё кроме кода.
	if err := repo.Update(models.Material{ID: id, Code: "PETG-CHANGED", Type: "PLA", Density: 1.24, Diameter: 1.75, Vendor: "V"}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, err := repo.GetByCode("PETG-T1")
	if err != nil {
		t.Fatalf("код обязан сохраниться: %v", err)
	}
	if got.Type != "PLA" || got.Density != 1.24 || got.Vendor != "V" {
		t.Fatalf("update не применился: %+v", got)
	}
	// Катушка без работ: удаляется каскадом вместе с материалом.
	if _, err := repo.AddSpool(models.Spool{MaterialID: id, Price: 100, Currency: "RUB"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete(id); err != nil {
		t.Fatalf("Delete с пустой катушкой: %v", err)
	}
	if _, err := repo.GetByCode("PETG-T1"); err == nil {
		t.Fatal("материал остался после Delete")
	}
	// Катушка с работой: блок.
	id3, _ := repo.UpsertByCode(models.Material{Code: "PETG-T3", Density: 1.27})
	sp3, _ := repo.AddSpool(models.Spool{MaterialID: id3, Price: 100, Currency: "RUB"})
	if _, err := db.Jobs().Upsert(models.Job{Filename: "x.gcode", StartAt: "2026-01-01 10:00", SpoolID: sp3}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete(id3); err == nil {
		t.Fatal("удаление с привязанными работами обязано блокироваться")
	}
	// Чистый материал удаляется.
	id2, _ := repo.UpsertByCode(models.Material{Code: "PETG-T2", Density: 1.27})
	if err := repo.Delete(id2); err != nil {
		t.Fatalf("Delete чистого: %v", err)
	}
	// Цена партии правится (на живом материале id3).
	spools, _ := repo.ListSpools(id3)
	if err := repo.UpdateSpool(models.Spool{ID: spools[0].ID, MaterialID: id3, Price: 99900, Currency: "RUB", WeightInitG: 1000, WeightLeftG: 500}); err != nil {
		t.Fatalf("UpdateSpool: %v", err)
	}
	spools, _ = repo.ListSpools(id3)
	if spools[0].Price != 99900 || spools[0].WeightLeftG != 500 {
		t.Fatalf("spool не обновился: %+v", spools[0])
	}
}

func TestSetMaterialPrice(t *testing.T) {
	db := openTestDB(t)
	repo := db.Materials()
	// Материал без катушки (стаб): цена обязана сохраниться через создание катушки.
	id, err := repo.UpsertByCode(models.Material{Code: "PETG-STUB", Density: 1.27})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SetMaterialPrice(id, 75000, "RUB"); err != nil {
		t.Fatalf("SetMaterialPrice на стабе: %v", err)
	}
	spools, err := repo.ListSpools(id)
	if err != nil || len(spools) != 1 || spools[0].Price != 75000 {
		t.Fatalf("катушка не создалась: %+v, %v", spools, err)
	}
	// Повторный вызов обновляет, а не плодит катушки; остаток не трогает.
	if _, err := db.Materials().AddSpool(models.Spool{MaterialID: id, Price: 1, Currency: "RUB", WeightInitG: 1000, WeightLeftG: 321}); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetMaterialPrice(id, 80000, "RUB"); err != nil {
		t.Fatal(err)
	}
	spools, _ = repo.ListSpools(id)
	if len(spools) != 2 || spools[0].Price != 80000 {
		t.Fatalf("цена первой катушки: %+v", spools)
	}
	if spools[1].WeightLeftG != 321 {
		t.Fatalf("вторая катушка тронута: %+v", spools[1])
	}
	if err := repo.SetMaterialPrice(0, 100, "RUB"); err == nil {
		t.Fatal("пустой material_id принят")
	}
	if err := repo.SetMaterialPrice(id, -5, "RUB"); err == nil {
		t.Fatal("отрицательная цена принята")
	}
}

func TestPrinterCRUDGuards(t *testing.T) {
	db := openTestDB(t)
	repo := db.Printers()
	id, err := repo.Save(models.Printer{Name: "P1", LifeYears: 3, HoursPerDay: 8, CalendarMode: "calendar", AmortMethod: "calendar"})
	if err != nil {
		t.Fatal(err)
	}
	p := models.Printer{ID: id, Name: "P1-renamed", Model: "K", PurchasePrice: 100, Currency: "RUB",
		LifeYears: 5, HoursPerDay: 12, CalendarMode: "workdays", AmortMethod: "resource",
		PowerKW: 0.5, ResourceHours: 8000, MoonrakerURL: "http://x:7125"}
	if err := repo.Update(p); err != nil {
		t.Fatalf("Update: %v", err)
	}
	list, _ := repo.List()
	if len(list) != 1 || list[0].Name != "P1-renamed" || list[0].MoonrakerURL != "http://x:7125" {
		t.Fatalf("update не применился: %+v", list)
	}
	// Работа блокирует удаление.
	if _, err := db.Jobs().Upsert(models.Job{Filename: "f.gcode", StartAt: "2026-01-01 10:00", PrinterID: id}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete(id); err == nil {
		t.Fatal("удаление с работами обязано блокироваться")
	}
	// Чистый принтер удаляется.
	id2, _ := repo.Save(models.Printer{Name: "P2", LifeYears: 3, HoursPerDay: 8, CalendarMode: "calendar", AmortMethod: "calendar"})
	if err := repo.Delete(id2); err != nil {
		t.Fatalf("Delete чистого: %v", err)
	}
}

func TestCustomerDeleteGuard(t *testing.T) {
	db := openTestDB(t)
	repo := db.Customers()
	id, _ := repo.Save(models.Customer{Name: "C"})
	if _, err := db.Orders().Create(models.Order{CustomerID: id, CreatedAt: "2026-01-01"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete(id); err == nil {
		t.Fatal("удаление с заказами обязано блокироваться")
	}
	id2, _ := repo.Save(models.Customer{Name: "C2"})
	if err := repo.Delete(id2); err != nil {
		t.Fatalf("Delete чистого: %v", err)
	}
}

func TestUserDeleteGuards(t *testing.T) {
	db := openTestDB(t)
	repo := db.Users()
	a1, _ := repo.Create(models.User{Login: "a1", PasswordHash: "x", Role: models.RoleAdmin})
	if err := repo.Delete(a1); err != nil {
		// repo-уровень удаляет; guards — в App (см. desktop TestRolesDeleteGuards)
		t.Fatalf("repo Delete: %v", err)
	}
}

func TestUpdateManualReprice(t *testing.T) {
	db := openTestDB(t)
	mats := db.Materials()
	mid, err := mats.UpsertByCode(models.Material{Code: "PETG-R", Type: "PETG", Density: 1.27, Diameter: 1.75})
	if err != nil {
		t.Fatal(err)
	}
	// Катушка 1000 ₽/кг → 1 ₽/г.
	if _, err := mats.AddSpool(models.Spool{MaterialID: mid, Price: 100000, Currency: "RUB", WeightInitG: 1000, WeightLeftG: 1000}); err != nil {
		t.Fatal(err)
	}
	jobs := db.Jobs()
	id, err := jobs.Upsert(models.Job{Filename: "r.gcode", StartAt: "2026-01-01 10:00", Origin: "manual", TotalSec: 3600, WeightG: 10})
	if err != nil {
		t.Fatal(err)
	}
	// import править нельзя.
	imp, _ := jobs.Upsert(models.Job{Filename: "i.gcode", StartAt: "2026-01-01 10:00", Origin: "import"})
	if err := jobs.UpdateManual(models.Job{ID: imp}, "PETG-R"); err == nil {
		t.Fatal("правка import обязана блокироваться")
	}
	// Правка веса 10→20г: мат составит 20₽ (плюс свет/амортизация/труд по контексту).
	if err := jobs.UpdateManual(models.Job{
		ID: id, Filename: "r.gcode", StartAt: "2026-01-01 10:00", Status: "completed",
		TotalSec: 3600, WeightG: 20,
	}, "PETG-R"); err != nil {
		t.Fatalf("UpdateManual: %v", err)
	}
	list, _ := jobs.List(0)
	var got models.Job
	for _, j := range list {
		if j.ID == id {
			got = j
		}
	}
	var snap struct {
		V        int                `json:"v"`
		Material string             `json:"material"`
		Cost     float64            `json:"cost"`
		Input    map[string]float64 `json:"input"`
	}
	if err := json.Unmarshal([]byte(got.CostSnapshot), &snap); err != nil {
		t.Fatalf("снапшот битый: %v", err)
	}
	if snap.V != 2 || snap.Material != "PETG-R" {
		t.Fatalf("снапшот: %+v", snap)
	}
	// mat=20, elec=7.08*0.35*1=2.478, amort≈5.41 → cost≈27.89.
	if snap.Cost < 27.5 || snap.Cost > 28.5 {
		t.Fatalf("cost = %v, хочу ~27.89", snap.Cost)
	}
	if got.WeightG != 20 || got.Status != "completed" {
		t.Fatalf("поля не сохранились: %+v", got)
	}
}
