package sqlite

import (
	"encoding/json"
	"testing"

	"3dprint-calc/core/calc"
	"3dprint-calc/core/models"
	"3dprint-calc/core/store"
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

// TestSpoolDeleteGuard: партию с привязанными работами удалить нельзя.
func TestSpoolDeleteGuard(t *testing.T) {
	db := openTestDB(t)
	mats := db.Materials()
	mid, _ := mats.UpsertByCode(models.Material{Code: "PETG-G", Type: "PETG", Density: 1.27})
	sid, err := mats.AddSpool(models.Spool{MaterialID: mid, Price: 100000, Currency: "RUB", WeightInitG: 1000, WeightLeftG: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Jobs().Upsert(models.Job{Filename: "g.gcode", StartAt: "2026-01-01 10:00", Origin: "manual", SpoolID: sid}); err != nil {
		t.Fatal(err)
	}
	if err := mats.DeleteSpool(sid); err == nil {
		t.Fatal("удаление катушки с работами обязано блокироваться")
	}
	// Свободная катушка удаляется.
	sid2, _ := mats.AddSpool(models.Spool{MaterialID: mid, Price: 100000, Currency: "RUB", WeightInitG: 500, WeightLeftG: 500})
	if err := mats.DeleteSpool(sid2); err != nil {
		t.Fatalf("DeleteSpool свободной: %v", err)
	}
}

// TestMaterialBoughtArchived: дата закупа и флаг архивности материала.
func TestMaterialBoughtArchived(t *testing.T) {
	db := openTestDB(t)
	mats := db.Materials()
	mid, err := mats.UpsertByCode(models.Material{Code: "PETG-A", Type: "PETG", Density: 1.27})
	if err != nil {
		t.Fatal(err)
	}
	if err := mats.Update(models.Material{ID: mid, Code: "PETG-A", Type: "PETG",
		Density: 1.27, BoughtAt: "2026-10-01", Archived: true}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, err := mats.GetByCode("PETG-A")
	if err != nil {
		t.Fatalf("GetByCode: %v", err)
	}
	if got.BoughtAt != "2026-10-01" || !got.Archived {
		t.Fatalf("поля не сохранились: %+v", got)
	}
	// Катушка архивного материала скрыта из остатков.
	if _, err := mats.AddSpool(models.Spool{MaterialID: mid, Price: 100, Currency: "RUB", WeightInitG: 1000, WeightLeftG: 1000}); err != nil {
		t.Fatal(err)
	}
	stats, err := db.SpoolStats()
	if err != nil {
		t.Fatalf("SpoolStats: %v", err)
	}
	for _, s := range stats {
		if s.Code == "PETG-A" {
			t.Fatalf("архивный материал в остатках: %+v", s)
		}
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
	// import теперь тоже правится (с флагом edited), а не блокируется.
	// Правка веса 10→20г: мат составит 20₽ (плюс свет/амортизация/труд по контексту).
	if err := jobs.UpdateManual(models.Job{
		ID: id, Filename: "r.gcode", StartAt: "2026-01-01 10:00", Status: "completed",
		TotalSec: 3600, WeightG: 20,
	}, "PETG-R", store.JobCalcInput{
		Tariff: 7.08, PowerKW: 0.35, AmortPerHour: 5.41, OpRate: 100, Copies: 1,
	}); err != nil {
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

// TestUpdateManualFullInput: правка пишет пересчёт строго по переданному
// контексту (не по старому снапшоту), вход эхом — в снапшот v2.
func TestUpdateManualFullInput(t *testing.T) {
	db := openTestDB(t)
	mats := db.Materials()
	mid, err := mats.UpsertByCode(models.Material{Code: "PETG-F", Type: "PETG", Density: 1.27, Diameter: 1.75})
	if err != nil {
		t.Fatal(err)
	}
	// Катушка 1000 ₽/кг → 1 ₽/г.
	if _, err := mats.AddSpool(models.Spool{MaterialID: mid, Price: 100000, Currency: "RUB", WeightInitG: 1000, WeightLeftG: 1000}); err != nil {
		t.Fatal(err)
	}
	jobs := db.Jobs()
	id, err := jobs.Upsert(models.Job{Filename: "f.gcode", StartAt: "2026-01-01 10:00",
		Origin: "import", TotalSec: 7200, WeightG: 10, CostSnapshot: `{"v":1}`})
	if err != nil {
		t.Fatal(err)
	}
	in := store.JobCalcInput{
		Tariff: 10, PowerKW: 0.5, AmortPerHour: 7, MaintPerHour: 1, PurgeWasteG: 5,
		OpRate: 200, PrepH: 0.5, PostH: 0.25,
		DesignerRate: 600, ModelH: 1, ModelPrice: 100,
		ExtraMat: 50, FailRate: 0.1, MarginProfit: 0.5, MarginDev: 0.2, Fees: 30,
		Copies: 2, VATRate: 0.22, RoundStep: 10,
	}
	if err := jobs.UpdateManual(models.Job{
		ID: id, Filename: "f.gcode", StartAt: "2026-01-01 10:00", Status: "completed",
		TotalSec: 7200, WeightG: 100,
	}, "PETG-F", in); err != nil {
		t.Fatalf("UpdateManual: %v", err)
	}
	list, _ := jobs.List(0)
	var got models.Job
	for _, j := range list {
		if j.ID == id {
			got = j
		}
	}
	if got.Edited != 1 {
		t.Fatalf("флаг edited: %+v", got)
	}
	var snap struct {
		V        int                `json:"v"`
		Material string             `json:"material"`
		Cost     float64            `json:"cost"`
		Price    float64            `json:"price"`
		Currency string             `json:"currency"`
		Input    map[string]float64 `json:"input"`
	}
	if err := json.Unmarshal([]byte(got.CostSnapshot), &snap); err != nil {
		t.Fatalf("снапшот битый: %v", err)
	}
	if snap.V != 2 || snap.Material != "PETG-F" || snap.Currency != "RUB" {
		t.Fatalf("шапка снапшота: %+v", snap)
	}
	for k, want := range map[string]float64{
		"tariff": 10, "powerKW": 0.5, "amortPerHour": 7, "maintPerHour": 1,
		"purgeWasteG": 5, "opRate": 200, "prepH": 0.5, "postH": 0.25,
		"designerRate": 600, "modelH": 1, "modelPrice": 100,
		"extraMat": 50, "failRate": 0.1, "marginProfit": 0.5, "marginDev": 0.2,
		"fees": 30, "copies": 2, "vatRate": 0.22, "roundStep": 10,
	} {
		if snap.Input[k] != want {
			t.Fatalf("input.%s = %v, хочу %v", k, snap.Input[k], want)
		}
	}
	want := calc.Calculate(calc.Input{
		TotalSec: 7200, WeightG: 100, PricePerKg: 1000,
		Tariff: 10, PowerKW: 0.5, AmortPerHour: 7, MaintPerHour: 1, PurgeWasteG: 5,
		OpRate: 200, PrepH: 0.5, PostH: 0.25,
		DesignerRate: 600, ModelH: 1, ModelPrice: 100,
		ExtraMat: 50, FailRate: 0.1, MarginProfit: 0.5, MarginDev: 0.2, Fees: 30,
		Copies: 2, VATRate: 0.22, RoundStep: 10,
	})
	if snap.Cost != want.Cost || snap.Price != want.PriceVAT {
		t.Fatalf("cost/price = %v/%v, хочу %v/%v", snap.Cost, snap.Price, want.Cost, want.PriceVAT)
	}
	// orig_snapshot записан (иначе откат упадёт): вес и снапшот возвращаются.
	if err := jobs.RevertJob(id); err != nil {
		t.Fatalf("RevertJob: %v", err)
	}
	list, _ = jobs.List(0)
	for _, j := range list {
		if j.ID == id {
			got = j
		}
	}
	if got.Edited != 0 || got.WeightG != 10 || got.CostSnapshot != `{"v":1}` {
		t.Fatalf("откат: %+v", got)
	}
}
