package main

import (
	"time"

	"3dprint-calc/core/models"
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

// OrderDetail — заказ с позициями, оплатами и балансом.
type OrderDetail struct {
	Order    models.Order       `json:"order"`
	Items    []models.OrderItem `json:"items"`
	Payments []models.Payment   `json:"payments"`
	Paid     int64              `json:"paid"`
	Balance  int64              `json:"balance"`
}

// CreateOrder — новый заказ (manager+). CreatedAt проставляется сегодня.
func (a *App) CreateOrder(customerID int64, currency string) (int64, error) {
	if err := a.need(models.RoleManager); err != nil {
		return 0, err
	}
	if currency == "" {
		currency = "RUB"
	}
	return a.db.Orders().Create(models.Order{
		CustomerID: customerID, Currency: currency,
		CreatedAt: time.Now().Format("2006-01-02"),
	})
}

// ListOrders — все заказы.
func (a *App) ListOrders() ([]models.Order, error) {
	return a.db.Orders().List()
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

// AddPayment — оплата (manager+).
func (a *App) AddPayment(orderID, amount int64, note string) (int64, error) {
	if err := a.need(models.RoleManager); err != nil {
		return 0, err
	}
	return a.db.Orders().AddPayment(models.Payment{
		OrderID: orderID, Amount: amount,
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

// Debts — неоплаченные остатки.
func (a *App) Debts() ([]sqlite.Debt, error) {
	return a.db.Debts()
}
