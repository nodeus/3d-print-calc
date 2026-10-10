package sqlite

import (
	"testing"

	"3dprint-calc/core/models"
)

func TestConvertCurrency(t *testing.T) {
	db := openTestDB(t)
	// Катушка 9000₽/кг, принтер 100000₽, заказ с позицией и оплатой.
	mid, _ := db.Materials().UpsertByCode(models.Material{Code: "PETG-FX", Density: 1.27})
	db.Materials().AddSpool(models.Spool{MaterialID: mid, Price: 900000, Currency: "RUB"})
	db.Printers().Save(models.Printer{Name: "P", PurchasePrice: 10000000, Currency: "RUB",
		LifeYears: 3, HoursPerDay: 8, CalendarMode: "calendar", AmortMethod: "calendar"})
	cid, _ := db.Customers().Save(models.Customer{Name: "C"})
	oid, _ := db.Orders().Create(models.Order{CustomerID: cid, CreatedAt: "2026-01-01"})
	db.Orders().AddItem(models.OrderItem{OrderID: oid, Title: "a", Qty: 1, Price: 100000})
	db.Orders().AddPayment(models.Payment{OrderID: oid, Amount: 40000, PaidAt: "2026-01-02"})
	// Курс USD: 100₽. RUB — базовая (rate 1 неявно).
	if err := db.ConvertCurrency("USD", 100); err != nil {
		t.Fatalf("ConvertCurrency: %v", err)
	}
	spools, _ := db.Materials().ListSpools(mid)
	if spools[0].Price != 9000 || spools[0].Currency != "USD" {
		t.Fatalf("spool: %+v", spools[0])
	}
	got, _ := db.Orders().Get(oid)
	if got.TotalBase != 1000 || got.Currency != "USD" {
		t.Fatalf("order: %+v", got)
	}
	items, _ := db.Orders().Items(oid)
	if items[0].Price != 1000 {
		t.Fatalf("item: %+v", items[0])
	}
	paid, _ := db.Orders().PaidTotal(oid)
	if paid != 400 {
		t.Fatalf("paid = %d", paid)
	}
	if v, ok, _ := db.GetSetting("base_currency"); !ok || v != "USD" {
		t.Fatalf("base = %q", v)
	}
	// Повторная конвертация в ту же валюту — ошибка.
	if err := db.ConvertCurrency("USD", 100); err == nil {
		t.Fatal("конвертация в текущую базу обязана отклоняться")
	}
	// Нет курса валюты — ошибка без частичной записи.
	mid2, _ := db.Materials().UpsertByCode(models.Material{Code: "PETG-EU", Density: 1.27})
	db.Materials().AddSpool(models.Spool{MaterialID: mid2, Price: 800, Currency: "EUR"})
	if err := db.ConvertCurrency("KZT", 0.2); err == nil {
		t.Fatal("без курса EUR обязана быть ошибка")
	}
	sp2, _ := db.Materials().ListSpools(mid2)
	if sp2[0].Currency != "EUR" {
		t.Fatalf("частичная запись! %+v", sp2[0])
	}
}

// TestInspectConvert: превью считает строки для конвертации, ничего не пишет.
func TestInspectConvert(t *testing.T) {
	db := openTestDB(t)
	mid, _ := db.Materials().UpsertByCode(models.Material{Code: "PETG-I", Density: 1.27})
	db.Materials().AddSpool(models.Spool{MaterialID: mid, Price: 100, Currency: "RUB"})
	db.Printers().Save(models.Printer{Name: "P", LifeYears: 3, HoursPerDay: 8,
		CalendarMode: "calendar", AmortMethod: "calendar"})
	cid, _ := db.Customers().Save(models.Customer{Name: "C"})
	oid, _ := db.Orders().Create(models.Order{CustomerID: cid, CreatedAt: "2026-01-01"})
	db.Orders().AddItem(models.OrderItem{OrderID: oid, Title: "a", Qty: 1, Price: 100})
	db.Orders().AddItem(models.OrderItem{OrderID: oid, Title: "b", Qty: 1, Price: 200})
	db.Orders().AddPayment(models.Payment{OrderID: oid, Amount: 50, PaidAt: "2026-01-02"})
	insp, err := db.InspectConvert()
	if err != nil {
		t.Fatalf("InspectConvert: %v", err)
	}
	if insp.Spools != 1 || insp.Printers != 1 || insp.Orders != 1 || insp.Items != 2 || insp.Payments != 1 {
		t.Fatalf("превью: %+v", insp)
	}
	// Ничего не переписано.
	spools, _ := db.Materials().ListSpools(mid)
	if spools[0].Currency != "RUB" {
		t.Fatalf("превью пишет! %+v", spools[0])
	}
}

func TestFxRatesCRUD(t *testing.T) {
	db := openTestDB(t)
	if err := db.SetFxRate("USD", 100); err != nil {
		t.Fatal(err)
	}
	if err := db.SetFxRate("", 1); err == nil {
		t.Fatal("пустая валюта принята")
	}
	list, err := db.ListFxRates()
	if err != nil || len(list) != 1 || list[0].Rate != 100 {
		t.Fatalf("rates = %+v, %v", list, err)
	}
	if err := db.DeleteFxRate("USD", list[0].Day); err != nil {
		t.Fatal(err)
	}
	list, _ = db.ListFxRates()
	if len(list) != 0 {
		t.Fatalf("rates = %+v", list)
	}
}
