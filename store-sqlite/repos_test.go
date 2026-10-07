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
	// Правка шапки.
	got.CustomerID = cid
	got.VAT = 0.2
	got.Title = "Полка в гараж"
	if err := repo.Update(got); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got2, _ := repo.Get(oid)
	if got2.VAT != 0.2 || got2.Title != "Полка в гараж" {
		t.Fatalf("шапка не сохранилась: %+v", got2)
	}
	// Каскадное удаление: заказ + позиции + оплаты исчезают.
	if err := repo.Delete(oid); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if items, _ := repo.Items(oid); len(items) != 0 {
		t.Fatalf("позиции остались: %d", len(items))
	}
	if pays, _ := repo.Payments(oid); len(pays) != 0 {
		t.Fatalf("оплаты остались: %d", len(pays))
	}
	if _, err := repo.Get(oid); err == nil {
		t.Fatal("заказ остался после Delete")
	}
}

func TestNextOrderNumber(t *testing.T) {
	db := openTestDB(t)
	// Дефолты: {YYYY}-{nnnn} с 1.
	n1, err := db.NextOrderNumber()
	if err != nil {
		t.Fatalf("NextOrderNumber: %v", err)
	}
	n2, err := db.NextOrderNumber()
	if err != nil {
		t.Fatal(err)
	}
	if n1 == n2 {
		t.Fatalf("счётчик не движется: %q", n1)
	}
	// Свой шаблон + свой счётчик.
	if err := db.SetSetting("order_number_template", `{"Заказ №"}{n}`); err != nil {
		t.Fatal(err)
	}
	if err := db.SetSetting("order_number_seq", "99"); err != nil {
		t.Fatal(err)
	}
	n3, err := db.NextOrderNumber()
	if err != nil || n3 != "Заказ №99" {
		t.Fatalf("custom = %q, %v", n3, err)
	}
	// Битый шаблон — ошибка без сдвига счётчика.
	if err := db.SetSetting("order_number_template", "{yyyy}"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.NextOrderNumber(); err == nil {
		t.Fatal("битый шаблон принят")
	}
	n4, err := db.NextOrderNumber()
	if err == nil {
		t.Fatalf("после битого шаблона всё ещё ок: %q", n4)
	}
	// Починили — счётчик продолжил со 100.
	if err := db.SetSetting("order_number_template", "{n}"); err != nil {
		t.Fatal(err)
	}
	n5, err := db.NextOrderNumber()
	if err != nil || n5 != "100" {
		t.Fatalf("после починки = %q, %v", n5, err)
	}
}

func TestDeleteJobOriginGuard(t *testing.T) {
	db := openTestDB(t)
	manual, err := db.Jobs().Upsert(models.Job{Filename: "m.gcode", StartAt: "2026-01-01 10:00", Origin: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	imp, err := db.Jobs().Upsert(models.Job{Filename: "i.gcode", StartAt: "2026-01-01 10:00", Origin: "import"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Jobs().DeleteJob(imp); err == nil {
		t.Fatal("удаление import обязано блокироваться")
	}
	if err := db.Jobs().DeleteJob(manual); err != nil {
		t.Fatalf("DeleteJob manual: %v", err)
	}
	if err := db.Jobs().DeleteJob(999999); err == nil {
		t.Fatal("удаление несуществующей обязано вернуть ошибку")
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
