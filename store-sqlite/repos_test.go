package sqlite

import (
	"testing"

	"3dprint-calc/core/auth"
	"3dprint-calc/core/models"
)

func TestUsersAuth(t *testing.T) {
	db := openTestDB(t)
	repo := db.Users()
	phc, err := auth.Hash("secret")
	if err != nil {
		t.Fatal(err)
	}
	id, err := repo.Create(models.User{Login: "manager1", PasswordHash: phc, Role: models.RoleManager, Locale: "ru"})
	if err != nil || id == 0 {
		t.Fatalf("Create: %v", id)
	}
	got, err := repo.GetByLogin("manager1")
	if err != nil {
		t.Fatalf("GetByLogin: %v", err)
	}
	if got.Role != models.RoleManager {
		t.Errorf("role = %q", got.Role)
	}
	if err := auth.Verify(got.PasswordHash, "secret"); err != nil {
		t.Errorf("Verify: %v", err)
	}
	// List не отдаёт хэши.
	list, err := repo.List()
	if err != nil || len(list) != 1 || list[0].PasswordHash != "" {
		t.Fatalf("List: %+v, %v", list, err)
	}
	if err := repo.SetRole(id, models.RoleViewer); err != nil {
		t.Fatalf("SetRole: %v", err)
	}
	if err := repo.SetRole(id, "boss"); err == nil {
		t.Fatal("неизвестная роль принята")
	}
	phc2, _ := auth.Hash("new")
	if err := repo.SetPassword(id, phc2); err != nil {
		t.Fatalf("SetPassword: %v", err)
	}
	got2, _ := repo.GetByLogin("manager1")
	if err := auth.Verify(got2.PasswordHash, "new"); err != nil {
		t.Errorf("новый пароль: %v", err)
	}
	if n, _ := repo.Count(); n != 1 {
		t.Errorf("Count = %d", n)
	}
}

func TestCustomersRoundTrip(t *testing.T) {
	db := openTestDB(t)
	repo := db.Customers()
	id, err := repo.Save(models.Customer{Name: "Илья", Contact: "tg", Currency: "RUB"})
	if err != nil || id == 0 {
		t.Fatalf("Save: %v", id)
	}
	if _, err := repo.Save(models.Customer{ID: id, Name: "Илья Ж.", Contact: "tg", Currency: "RUB"}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	list, err := repo.List()
	if err != nil || len(list) != 1 || list[0].Name != "Илья Ж." {
		t.Fatalf("List: %+v, %v", list, err)
	}
}

func TestOrdersFlow(t *testing.T) {
	db := openTestDB(t)
	repo := db.Orders()
	cid, err := db.Customers().Save(models.Customer{Name: "Клиент"})
	if err != nil {
		t.Fatal(err)
	}
	oid, err := repo.Create(models.Order{CustomerID: cid, Currency: "RUB", CreatedAt: "2026-10-01"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := repo.Create(models.Order{Status: "бред"}); err == nil {
		t.Fatal("неизвестный статус принят")
	}
	if _, err := repo.AddItem(models.OrderItem{OrderID: oid, Title: "Крюк", Qty: 2, Cost: 10000, Price: 25000}); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if _, err := repo.AddItem(models.OrderItem{OrderID: oid, Title: "Полка", Qty: 1.5, Cost: 5000, Price: 10000}); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	got, err := repo.Get(oid)
	if err != nil {
		t.Fatal(err)
	}
	// 2*25000 + 1.5*10000 = 65000.
	if got.TotalBase != 65000 {
		t.Errorf("total = %d, хочу 65000", got.TotalBase)
	}
	items, err := repo.Items(oid)
	if err != nil || len(items) != 2 {
		t.Fatalf("Items: %v, %v", len(items), err)
	}
	if err := repo.SetStatus(oid, "quoted"); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}
	if _, err := repo.AddPayment(models.Payment{OrderID: oid, Amount: 20000, PaidAt: "2026-10-02"}); err != nil {
		t.Fatalf("AddPayment: %v", err)
	}
	if _, err := repo.AddPayment(models.Payment{OrderID: oid, Amount: 0}); err == nil {
		t.Fatal("нулевая оплата принята")
	}
	paid, err := repo.PaidTotal(oid)
	if err != nil || paid != 20000 {
		t.Errorf("paid = %d, %v", paid, err)
	}
	// Долг: баланс 45000.
	debts, err := db.Debts()
	if err != nil {
		t.Fatal(err)
	}
	if len(debts) != 1 || debts[0].Balance != 45000 {
		t.Fatalf("debts = %+v", debts)
	}
	// Доплатили всё — долг исчез.
	if _, err := repo.AddPayment(models.Payment{OrderID: oid, Amount: 45000, PaidAt: "2026-10-03"}); err != nil {
		t.Fatal(err)
	}
	debts, _ = db.Debts()
	if len(debts) != 0 {
		t.Fatalf("долг обязан закрыться: %+v", debts)
	}
}

func TestReports(t *testing.T) {
	db := openTestDB(t)
	repo := db.Orders()
	cid, _ := db.Customers().Save(models.Customer{Name: "К"})
	o1, _ := repo.Create(models.Order{CustomerID: cid, CreatedAt: "2026-09-05", Status: "paid"})
	o2, _ := repo.Create(models.Order{CustomerID: cid, CreatedAt: "2026-10-01", Status: "done"})
	repo.AddItem(models.OrderItem{OrderID: o1, Title: "a", Qty: 1, Price: 10000})
	repo.AddItem(models.OrderItem{OrderID: o2, Title: "b", Qty: 2, Price: 10000})
	rev, err := db.RevenueByMonth()
	if err != nil {
		t.Fatalf("RevenueByMonth: %v", err)
	}
	if len(rev) != 2 || rev[0].Month != "2026-09" || rev[0].Total != 10000 || rev[1].Total != 20000 {
		t.Fatalf("revenue = %+v", rev)
	}
	if _, err := db.Jobs().Upsert(models.Job{Filename: "x.gcode", StartAt: "2026-10-01 10:00", Status: "completed", TotalSec: 3600, WeightG: 50, CostSnapshot: `{"material":"PETG-NIT-W4","cost":100}`}); err != nil {
		t.Fatal(err)
	}
	top, err := db.TopMaterials(10)
	if err != nil {
		t.Fatalf("TopMaterials: %v", err)
	}
	if len(top) != 1 || top[0].Code != "PETG-NIT-W4" || top[0].Jobs != 1 {
		t.Fatalf("top = %+v", top)
	}
	load, err := db.PrinterLoad()
	if err != nil {
		t.Fatalf("PrinterLoad: %v", err)
	}
	_ = load
}
