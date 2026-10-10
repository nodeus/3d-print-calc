package sqlite

import (
	"fmt"
	"time"
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
	out := []MonthRevenue{}
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
	out := []MaterialStat{}
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
	Name     string  `json:"name"`
	Jobs     int64   `json:"jobs"`
	TotalSec int64   `json:"totalSec"`
	LengthM  float64 `json:"lengthM"`
	WeightG  float64 `json:"weightG"`
	RemainH  float64 `json:"remainH"` // остаток ресурса/эксплуатации, ч
}

// PrinterLoad — число печати, время, длина, вес и остаток ресурса по принтерам.
func (d *DB) PrinterLoad() ([]PrinterStat, error) {
	rows, err := d.sql.Query(`SELECT p.name, COUNT(j.id), COALESCE(SUM(j.total_sec), 0),
		COALESCE(SUM(j.length_m), 0), COALESCE(SUM(j.weight_g), 0),
		p.commissioning_date, p.life_years, p.hours_per_day,
		p.calendar_mode, p.amort_method, p.resource_hours
		FROM printers p LEFT JOIN jobs j ON j.printer_id = p.id
		GROUP BY p.id ORDER BY p.name`)
	if err != nil {
		return nil, fmt.Errorf("ERR_REPORT: %w", err)
	}
	defer rows.Close()
	out := []PrinterStat{}
	for rows.Next() {
		var s PrinterStat
		var commissioned string
		var lifeYears int
		var hoursPerDay float64
		var calendarMode, amortMethod string
		var resourceHours float64
		if err := rows.Scan(&s.Name, &s.Jobs, &s.TotalSec, &s.LengthM, &s.WeightG,
			&commissioned, &lifeYears, &hoursPerDay, &calendarMode, &amortMethod,
			&resourceHours); err != nil {
			return nil, fmt.Errorf("ERR_REPORT: %w", err)
		}
		s.RemainH = remainHours(commissioned, lifeYears, hoursPerDay,
			calendarMode, amortMethod, resourceHours, float64(s.TotalSec)/3600)
		out = append(out, s)
	}
	return out, rows.Err()
}

// remainHours — остаток ресурса/эксплуатации в часах (пол — ноль).
func remainHours(commissioned string, lifeYears int, hoursPerDay float64,
	calendarMode, amortMethod string, resourceHours, usedHours float64) float64 {
	var total float64
	if amortMethod == "resource" && resourceHours > 0 {
		total = resourceHours
	} else {
		days := float64(lifeYears * 365)
		if start, err := time.Parse("2006-01-02", commissioned); err == nil && lifeYears > 0 {
			days = start.AddDate(lifeYears, 0, 0).Sub(start).Hours() / 24
		}
		if calendarMode == "workdays" || amortMethod == "workdays" {
			days = days * 5 / 7
		}
		total = days * hoursPerDay
	}
	if rem := total - usedHours; rem > 0 {
		return rem
	}
	return 0
}

// SpoolStat — остатки катушки: использование из истории, остаток вычисляемый.
type SpoolStat struct {
	Code     string  `json:"code"`
	SpoolNo  int     `json:"spoolNo"`
	ColorHex string  `json:"colorHex"`
	Vendor   string  `json:"vendor"`
	Jobs     int64   `json:"jobs"`
	UsedG    float64 `json:"usedG"`
	InitG    float64 `json:"initG"`
	LeftG    float64 `json:"leftG"`
}

// SpoolStats — остатки по катушкам: used = Σ веса работ, left = init − used.
// Ничего не перезаписывает (идемпотентно при реимпортах).
// Работы с явным spool_id идут на свою катушку; работы без катушки —
// на первую неархивную катушку того же материала (код из снапшота).
// Без кода — ничьи. Архивные катушки скрыты (архив — уровень катушки,
// флаг материала игнорируется).
func (d *DB) SpoolStats() ([]SpoolStat, error) {
	rows, err := d.sql.Query(`SELECT m.code, COALESCE(s.spool_no, 0), m.color_hex, m.vendor, s.weight_init_g,
		COUNT(j.id), COALESCE(SUM(j.weight_g), 0)
		FROM spools s JOIN materials m ON m.id = s.material_id
		LEFT JOIN jobs j ON j.spool_id = s.id OR (
			j.spool_id IS NULL
			AND json_extract(COALESCE(j.cost_snapshot, '{}'), '$.material') = m.code
			AND s.id = (SELECT MIN(s2.id) FROM spools s2
				WHERE s2.material_id = m.id AND COALESCE(s2.archived, 0) = 0)
		)
		WHERE COALESCE(s.archived, 0) = 0
		GROUP BY s.id ORDER BY m.code, s.spool_no`)
	if err != nil {
		return nil, fmt.Errorf("ERR_REPORT: %w", err)
	}
	defer rows.Close()
	out := []SpoolStat{}
	for rows.Next() {
		var s SpoolStat
		if err := rows.Scan(&s.Code, &s.SpoolNo, &s.ColorHex, &s.Vendor, &s.InitG,
			&s.Jobs, &s.UsedG); err != nil {
			return nil, fmt.Errorf("ERR_REPORT: %w", err)
		}
		s.LeftG = s.InitG - s.UsedG
		if s.LeftG < 0 {
			s.LeftG = 0
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Debt — неоплаченный остаток заказа.
type Debt struct {
	OrderID  int64  `json:"orderId"`
	OrderNo  string `json:"orderNo"`
	Customer string `json:"customer"`
	Title    string `json:"title"`
	Total    int64  `json:"total"`
	Paid     int64  `json:"paid"`
	Balance  int64  `json:"balance"`
	Status   string `json:"status"`
}

// Debts возвращает заказы с положительным остатком (кроме отменённых).
func (d *DB) Debts() ([]Debt, error) {
	rows, err := d.sql.Query(`SELECT o.id, COALESCE(o.number, ''), COALESCE(c.name, ''),
		COALESCE(o.title, ''), o.total_base,
		COALESCE((SELECT SUM(amount) FROM payments p WHERE p.order_id = o.id), 0), o.status
		FROM orders o LEFT JOIN customers c ON c.id = o.customer_id
		WHERE o.status NOT IN ('cancelled', 'paid') ORDER BY o.id DESC`)
	if err != nil {
		return nil, fmt.Errorf("ERR_REPORT: %w", err)
	}
	defer rows.Close()
	out := []Debt{}
	for rows.Next() {
		var x Debt
		if err := rows.Scan(&x.OrderID, &x.OrderNo, &x.Customer, &x.Title, &x.Total, &x.Paid, &x.Status); err != nil {
			return nil, fmt.Errorf("ERR_REPORT: %w", err)
		}
		x.Balance = x.Total - x.Paid
		if x.Balance > 0 {
			out = append(out, x)
		}
	}
	return out, rows.Err()
}
