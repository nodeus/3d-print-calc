package main

import (
	"fmt"
	"strconv"
	"time"

	"3dprint-calc/core/models"
	"3dprint-calc/core/ordnum"
	sqlite "3dprint-calc/store-sqlite"
)

// Заказчики, заказы, оплаты, отчёты. Чтение — вошедшим, запись — manager+.

// ListCustomers — заказчики.
func (a *App) ListCustomers() ([]models.Customer, error) {
	return a.db.Customers().List()
}

// SaveCustomer — создать/обновить заказчика (manager+).
func (a *App) SaveCustomer(c models.Customer) (int64, error) {
	if err := a.need(models.RoleManager); err != nil {
		return 0, err
	}
	if c.Currency == "" {
		c.Currency = "RUB"
	}
	return a.db.Customers().Save(c)
}

// DeleteCustomer — удалить заказчика (manager+). Блок при заказах — в store.
func (a *App) DeleteCustomer(id int64) error {
	if err := a.need(models.RoleManager); err != nil {
		return err
	}
	return a.db.Customers().Delete(id)
}

// OrderDetail — заказ с позициями, оплатами и балансом.
type OrderDetail struct {
	Order    models.Order       `json:"order"`
	Items    []models.OrderItem `json:"items"`
	Payments []models.Payment   `json:"payments"`
	Paid     int64              `json:"paid"`
	Balance  int64              `json:"balance"`
}

// CreateOrder — новый заказ (manager+). CreatedAt проставляется сегодня.
func (a *App) CreateOrder(customerID int64, currency, title string) (int64, error) {
	if err := a.need(models.RoleManager); err != nil {
		return 0, err
	}
	if currency == "" {
		currency = "RUB"
	}
	number, err := a.db.NextOrderNumber()
	if err != nil {
		return 0, err
	}
	return a.db.Orders().Create(models.Order{
		CustomerID: customerID, Number: number, Title: title, Currency: currency,
		CreatedAt: time.Now().Format("2006-01-02 15:04"),
	})
}

// OrderNumberSettings — шаблон и счётчик номеров для Настроек.
func (a *App) OrderNumberSettings() (map[string]string, error) {
	tpl, seq := a.db.OrderNumberSettings()
	return map[string]string{
		"template": tpl,
		"seq":      strconv.Itoa(seq),
		"preview":  previewNumber(tpl, seq),
	}, nil
}

// SetOrderNumberSettings — шаблон и счётчик (manager+). Шаблон валидируется.
func (a *App) SetOrderNumberSettings(template, seq string) error {
	if err := a.need(models.RoleManager); err != nil {
		return err
	}
	if _, err := ordnum.Render(template, time.Now(), 1); err != nil {
		return err
	}
	n, err := strconv.Atoi(seq)
	if err != nil || n < 1 {
		return fmt.Errorf("ERR_ORDNUM_SEQ: счётчик обязан быть ≥ 1")
	}
	if err := a.db.SetSetting("order_number_template", template); err != nil {
		return err
	}
	return a.db.SetSetting("order_number_seq", seq)
}

// PreviewOrderNumber — пример номера для шаблона без сохранения.
func (a *App) PreviewOrderNumber(template string) (string, error) {
	tpl, seq := a.db.OrderNumberSettings()
	if template != "" {
		tpl = template
	}
	return ordnum.Render(tpl, time.Now(), seq)
}

func previewNumber(tpl string, seq int) string {
	s, err := ordnum.Render(tpl, time.Now(), seq)
	if err != nil {
		return err.Error()
	}
	return s
}

// ListOrders — все заказы.
func (a *App) ListOrders() ([]models.Order, error) {
	return a.db.Orders().List()
}

// OrderRow — заказ с оплатами для списка.
type OrderRow struct {
	Order    models.Order `json:"order"`
	Paid     int64        `json:"paid"`
	Balance  int64        `json:"balance"`
	PayState string       `json:"payState"` // paid | partial | unpaid | unknown
}

// PayStateOf вычисляет состояние оплаты: paid / partial / unpaid / unknown.
// Нулевой итог без оплат (пустой или бесплатный заказ) — unknown, не paid.
func PayStateOf(total, paid int64) string {
	if total <= 0 && paid <= 0 {
		return "unknown"
	}
	if paid >= total {
		return "paid"
	}
	if paid <= 0 {
		return "unpaid"
	}
	return "partial"
}

// ListOrderRows — заказы с оплаченным/остатком/состоянием для списка.
func (a *App) ListOrderRows() ([]OrderRow, error) {
	orders, err := a.db.Orders().List()
	if err != nil {
		return nil, err
	}
	out := make([]OrderRow, 0, len(orders))
	for _, o := range orders {
		paid, err := a.db.Orders().PaidTotal(o.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, OrderRow{
			Order: o, Paid: paid,
			Balance:  o.TotalBase - paid,
			PayState: PayStateOf(o.TotalBase, paid),
		})
	}
	return out, nil
}

// GetOrderDetail — заказ целиком.
func (a *App) GetOrderDetail(id int64) (OrderDetail, error) {
	var d OrderDetail
	var err error
	if d.Order, err = a.db.Orders().Get(id); err != nil {
		return d, err
	}
	if d.Items, err = a.db.Orders().Items(id); err != nil {
		return d, err
	}
	if d.Payments, err = a.db.Orders().Payments(id); err != nil {
		return d, err
	}
	if d.Paid, err = a.db.Orders().PaidTotal(id); err != nil {
		return d, err
	}
	d.Balance = d.Order.TotalBase - d.Paid
	return d, nil
}

// SetOrderStatus — смена статуса (manager+).
func (a *App) SetOrderStatus(id int64, status string) error {
	if err := a.need(models.RoleManager); err != nil {
		return err
	}
	return a.db.Orders().SetStatus(id, status)
}

// UpdateOrder — правка шапки заказа (manager+).
func (a *App) UpdateOrder(o models.Order) error {
	if err := a.need(models.RoleManager); err != nil {
		return err
	}
	return a.db.Orders().Update(o)
}

// DeleteOrder — удаление заказа каскадом (manager+). Confirm — в UI.
func (a *App) DeleteOrder(id int64) error {
	if err := a.need(models.RoleManager); err != nil {
		return err
	}
	return a.db.Orders().Delete(id)
}

// DeleteOrderItem — удалить позицию с пересчётом (manager+).
func (a *App) DeleteOrderItem(id int64) error {
	if err := a.need(models.RoleManager); err != nil {
		return err
	}
	return a.db.Orders().DeleteOrderItem(id)
}

// ToggleItemPaid — ручная отметка оплаты позиции (manager+, сумм не меняет).
func (a *App) ToggleItemPaid(id int64, paid bool) error {
	if err := a.need(models.RoleManager); err != nil {
		return err
	}
	return a.db.Orders().SetItemPaid(id, paid)
}

// OrderItemInput — позиция из калькулятора (деньги — минорные единицы).
type OrderItemInput struct {
	OrderID int64   `json:"orderId"`
	JobID   int64   `json:"jobId"`
	Title   string  `json:"title"`
	Qty     float64 `json:"qty"`
	Cost    int64   `json:"cost"`
	Price   int64   `json:"price"`
}

// AddOrderItem — позиция в заказ (manager+). Итог пересчитывается.
func (a *App) AddOrderItem(in OrderItemInput) (int64, error) {
	if err := a.need(models.RoleManager); err != nil {
		return 0, err
	}
	if in.Qty <= 0 {
		in.Qty = 1
	}
	return a.db.Orders().AddItem(models.OrderItem{
		OrderID: in.OrderID, JobID: in.JobID, Title: in.Title,
		Qty: in.Qty, Cost: in.Cost, Price: in.Price,
	})
}

// AddPayment — оплата всего заказа (itemID=0) или позиции (manager+).
func (a *App) AddPayment(orderID, itemID, amount int64, note string) (int64, error) {
	if err := a.need(models.RoleManager); err != nil {
		return 0, err
	}
	return a.db.Orders().AddPayment(models.Payment{
		OrderID: orderID, OrderItem: itemID, Amount: amount,
		PaidAt: time.Now().Format("2006-01-02"), Note: note,
	})
}

// RevenueByMonth — выручка по месяцам.
func (a *App) RevenueByMonth() ([]sqlite.MonthRevenue, error) {
	return a.db.RevenueByMonth()
}

// TopMaterials — топ материалов из истории.
func (a *App) TopMaterials() ([]sqlite.MaterialStat, error) {
	return a.db.TopMaterials(10)
}

// PrinterLoad — загрузка принтеров.
func (a *App) PrinterLoad() ([]sqlite.PrinterStat, error) {
	return a.db.PrinterLoad()
}

// SpoolStats — остатки катушек для отчётов.
func (a *App) SpoolStats() ([]sqlite.SpoolStat, error) {
	return a.db.SpoolStats()
}

// SpoolPurchase — закупка пластика (катушка) для отчёта закупок.
type SpoolPurchase struct {
	Code     string  `json:"code"`
	Vendor   string  `json:"vendor"`
	BoughtAt string  `json:"boughtAt"`
	Price    float64 `json:"price"`
	Currency string  `json:"currency"`
	WeightG  float64 `json:"weightG"`
	Archived bool    `json:"archived"`
}

// SpoolPurchases — все катушки с датами и ценами (для отчёта закупок).
func (a *App) SpoolPurchases() ([]SpoolPurchase, error) {
	mats, err := a.db.Materials().List()
	if err != nil {
		return nil, err
	}
	out := []SpoolPurchase{}
	for _, m := range mats {
		spools, err := a.db.Materials().ListSpools(m.ID)
		if err != nil {
			return nil, err
		}
		for _, s := range spools {
			out = append(out, SpoolPurchase{
				Code: m.Code, Vendor: m.Vendor, BoughtAt: s.BoughtAt,
				Price: float64(s.Price) / 100, Currency: s.Currency,
				WeightG: s.WeightInitG, Archived: m.Archived,
			})
		}
	}
	return out, nil
}

// Debts — неоплаченные остатки.
func (a *App) Debts() ([]sqlite.Debt, error) {
	return a.db.Debts()
}
