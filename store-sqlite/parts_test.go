package sqlite

import (
	"testing"

	"3dprint-calc/core/models"
)

// TestPartPurchases: CRUD закупок и блок удаления принтера с закупками.
func TestPartPurchases(t *testing.T) {
	db := openTestDB(t)
	pp, err := db.Printers().Save(models.Printer{Name: "K1C", CalendarMode: "calendar", AmortMethod: "calendar"})
	if err != nil {
		t.Fatal(err)
	}
	id, err := db.Parts().Save(models.PartPurchase{
		PrinterID: pp, BoughtAt: "2026-10-01", Name: "Сопло 0.4", Price: 50000, Qty: 2, Note: "латунь",
	})
	if err != nil || id == 0 {
		t.Fatalf("Save: %v, %d", id, err)
	}
	list, err := db.Parts().ListByPrinter(pp)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListByPrinter: %v, %d", len(list), err)
	}
	got := list[0]
	if got.Name != "Сопло 0.4" || got.Price != 50000 || got.Qty != 2 || got.BoughtAt != "2026-10-01" {
		t.Fatalf("поля: %+v", got)
	}
	// Правка через Save с ID.
	got.Note = "латунь, 2 шт"
	if _, err := db.Parts().Save(got); err != nil {
		t.Fatalf("Update: %v", err)
	}
	all, err := db.Parts().ListAll()
	if err != nil || len(all) != 1 || all[0].Note != "латунь, 2 шт" {
		t.Fatalf("ListAll: %+v, %v", all, err)
	}
	// Принтер с закупками не удаляется.
	if err := db.Printers().Delete(pp); err == nil {
		t.Fatal("удаление принтера с закупками обязано блокироваться")
	}
	// Удаление закупки снимает блок.
	if err := db.Parts().Delete(id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := db.Printers().Delete(pp); err != nil {
		t.Fatalf("Delete принтера после чистки: %v", err)
	}
	// Qty<=0 нормализуется в 1.
	pp2, _ := db.Printers().Save(models.Printer{Name: "K2", CalendarMode: "calendar", AmortMethod: "calendar"})
	id2, err := db.Parts().Save(models.PartPurchase{PrinterID: pp2, Name: "ТО", Price: 100000})
	if err != nil {
		t.Fatal(err)
	}
	l2, _ := db.Parts().ListByPrinter(pp2)
	if len(l2) != 1 || l2[0].Qty != 1 || l2[0].ID != id2 {
		t.Fatalf("qty default: %+v", l2)
	}
}
