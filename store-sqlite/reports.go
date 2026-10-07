package sqlite

import (
	"fmt"
)

// Отчёты режима «поток»: агрегации поверх заказов и истории.
// Деньги — минорные единицы базовой валюты.

// MonthRevenue — выручка и число заказов за месяц (YYYY-MM).
type MonthRevenue struct {
	Month  string `json:"month"`
	Total  int64  `json:"total"`
	Orders int64  `json:"orders"`
}

// RevenueByMonth группирует заказы (кроме черновиков и отменённых) по месяцам.
func (d *DB) RevenueByMonth() ([]MonthRevenue, error) {
	rows, err := d.sql.Query(`SELECT SUBSTR(created_at, 1, 7) AS m,
		SUM(total_base), COUNT(*) FROM orders
		WHERE status NOT IN ('draft', 'cancelled') AND created_at <> ''
		GROUP BY m ORDER BY m`)
	if err != nil {
		return nil, fmt.Errorf("ERR_REPORT: %w", err)
	}
	defer rows.Close()
	var out []MonthRevenue
	for rows.Next() {
		var r MonthRevenue
		if err := rows.Scan(&r.Month, &r.Total, &r.Orders); err != nil {
			return nil, fmt.Errorf("ERR_REPORT: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// MaterialStat — использование материала из снапшотов истории.
type MaterialStat struct {
	Code   string  `json:"code"`
	Jobs   int64   `json:"jobs"`
	Weight float64 `json:"weight"`
	Cost   float64 `json:"cost"`
}

// TopMaterials — материалы по числу печати (JSON1 поверх cost_snapshot).
func (d *DB) TopMaterials(limit int) ([]MaterialStat, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := d.sql.Query(`SELECT COALESCE(NULLIF(JSON_EXTRACT(cost_snapshot, '$.material'), ''), '(?)'),
		COUNT(*), COALESCE(SUM(weight_g), 0), COALESCE(SUM(CAST(JSON_EXTRACT(cost_snapshot, '$.cost') AS REAL)), 0)
		FROM jobs GROUP BY 1 ORDER BY 2 DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("ERR_REPORT: %w", err)
	}
	defer rows.Close()
	var out []MaterialStat
	for rows.Next() {
		var s MaterialStat
		if err := rows.Scan(&s.Code, &s.Jobs, &s.Weight, &s.Cost); err != nil {
			return nil, fmt.Errorf("ERR_REPORT: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// PrinterStat — загрузка принтера.
type PrinterStat struct {
	Name     string `json:"name"`
	Jobs     int64  `json:"jobs"`
	TotalSec int64  `json:"totalSec"`
}

// PrinterLoad — число печати и суммарное время по принтерам.
func (d *DB) PrinterLoad() ([]PrinterStat, error) {
	rows, err := d.sql.Query(`SELECT p.name, COUNT(j.id), COALESCE(SUM(j.total_sec), 0)
		FROM printers p LEFT JOIN jobs j ON j.printer_id = p.id
		GROUP BY p.id ORDER BY p.name`)
	if err != nil {
		return nil, fmt.Errorf("ERR_REPORT: %w", err)
	}
	defer rows.Close()
	var out []PrinterStat
	for rows.Next() {
		var s PrinterStat
		if err := rows.Scan(&s.Name, &s.Jobs, &s.TotalSec); err != nil {
			return nil, fmt.Errorf("ERR_REPORT: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Debt — неоплаченный остаток заказа.
type Debt struct {
	OrderID  int64  `json:"orderId"`
	Customer string `json:"customer"`
	Total    int64  `json:"total"`
	Paid     int64  `json:"paid"`
	Balance  int64  `json:"balance"`
	Status   string `json:"status"`
}

// Debts возвращает заказы с положительным остатком (кроме отменённых).
func (d *DB) Debts() ([]Debt, error) {
	rows, err := d.sql.Query(`SELECT o.id, COALESCE(c.name, ''), o.total_base,
		COALESCE((SELECT SUM(amount) FROM payments p WHERE p.order_id = o.id), 0), o.status
		FROM orders o LEFT JOIN customers c ON c.id = o.customer_id
		WHERE o.status NOT IN ('cancelled', 'paid') ORDER BY o.id DESC`)
	if err != nil {
		return nil, fmt.Errorf("ERR_REPORT: %w", err)
	}
	defer rows.Close()
	var out []Debt
	for rows.Next() {
		var x Debt
		if err := rows.Scan(&x.OrderID, &x.Customer, &x.Total, &x.Paid, &x.Status); err != nil {
			return nil, fmt.Errorf("ERR_REPORT: %w", err)
		}
		x.Balance = x.Total - x.Paid
		if x.Balance > 0 {
			out = append(out, x)
		}
	}
	return out, rows.Err()
}
