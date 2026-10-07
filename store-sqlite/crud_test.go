package sqlite

import (
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
	// Катушка блокирует удаление.
	if _, err := repo.AddSpool(models.Spool{MaterialID: id, Price: 100, Currency: "RUB"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete(id); err == nil {
		t.Fatal("удаление с катушками обязано блокироваться")
	}
	// Чистый материал удаляется.
	id2, _ := repo.UpsertByCode(models.Material{Code: "PETG-T2", Density: 1.27})
	if err := repo.Delete(id2); err != nil {
		t.Fatalf("Delete чистого: %v", err)
	}
	// Цена партии правится.
	spools, _ := repo.ListSpools(id)
	if err := repo.UpdateSpool(models.Spool{ID: spools[0].ID, MaterialID: id, Price: 99900, Currency: "RUB", WeightInitG: 1000, WeightLeftG: 500}); err != nil {
		t.Fatalf("UpdateSpool: %v", err)
	}
	spools, _ = repo.ListSpools(id)
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
