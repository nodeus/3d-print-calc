package sqlite

import (
	"database/sql"
	"fmt"
	"math"
	"time"
)

// Валюты: курсы — единиц базовой за 1 единицу валюты (ручной ввод).
// Конвертация переписывает суммы в целевую валюту и переключает базу.
// Снапшоты истории (jobs.cost_snapshot) — исторические, не трогаются.

// FxRate — курс валюты к базе.
type FxRate struct {
	Currency string  `json:"currency"`
	Rate     float64 `json:"rate"`
	Day      string  `json:"day"`
}

// ListFxRates возвращает курсы (новые первыми).
func (d *DB) ListFxRates() ([]FxRate, error) {
	rows, err := d.sql.Query(`SELECT currency, rate, day FROM fx_rates ORDER BY day DESC, currency`)
	if err != nil {
		return nil, fmt.Errorf("ERR_FX: %w", err)
	}
	defer rows.Close()
	out := []FxRate{}
	for rows.Next() {
		var r FxRate
		if err := rows.Scan(&r.Currency, &r.Rate, &r.Day); err != nil {
			return nil, fmt.Errorf("ERR_FX: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// SetFxRate задаёт курс валюты на сегодня (upsert по паре валюта+день).
func (d *DB) SetFxRate(currency string, rate float64) error {
	if currency == "" || rate <= 0 {
		return fmt.Errorf("ERR_FX: пустая валюта или неположительный курс")
	}
	day := time.Now().Format("2006-01-02")
	if _, err := d.sql.Exec(`INSERT INTO fx_rates (currency, rate, day) VALUES (?, ?, ?)
		ON CONFLICT(currency, day) DO UPDATE SET rate=excluded.rate`,
		currency, rate, day); err != nil {
		return fmt.Errorf("ERR_FX: %w", err)
	}
	return nil
}

// DeleteFxRate удаляет курс.
func (d *DB) DeleteFxRate(currency, day string) error {
	if _, err := d.sql.Exec(`DELETE FROM fx_rates WHERE currency=? AND day=?`, currency, day); err != nil {
		return fmt.Errorf("ERR_FX: %w", err)
	}
	return nil
}

// rateToBase возвращает свежий курс валюты к базе (база самой базы = 1).
func (d *DB) rateToBase(tx *sql.Tx, currency, base string) (float64, error) {
	if currency == "" || currency == base {
		return 1, nil
	}
	var rate float64
	err := tx.QueryRow(`SELECT rate FROM fx_rates WHERE currency=? ORDER BY day DESC LIMIT 1`,
		currency).Scan(&rate)
	if err != nil {
		return 0, fmt.Errorf("ERR_FX: нет курса %s: %w", currency, err)
	}
	if rate <= 0 {
		return 0, fmt.Errorf("ERR_FX: битый курс %s", currency)
	}
	return rate, nil
}

// ConvertCurrency пересчитывает ВСЕ денежные суммы в target по курсу:
// amount_target = round(amount_src × rate_src / rate_target), где курсы —
// единиц текущей базы за единицу валюты. Затем base_currency=target.
// Снапшоты jobs — история, не трогаются. Всё — одной транзакцией.
func (d *DB) ConvertCurrency(target string, targetRate float64) error {
	if target == "" || targetRate <= 0 {
		return fmt.Errorf("ERR_FX: пустая валюта или курс")
	}
	var base string
	if v, ok, _ := d.GetSetting("base_currency"); ok && v != "" {
		base = v
	} else {
		base = "RUB"
	}
	if target == base {
		return fmt.Errorf("ERR_FX: это уже базовая валюта")
	}
	tx, err := d.sql.Begin()
	if err != nil {
		return fmt.Errorf("ERR_FX: %w", err)
	}
	defer tx.Rollback()

	// factor возвращает множитель src→target.
	factor := func(cur string) (float64, error) {
		rs, err := d.rateToBase(tx, cur, base)
		if err != nil {
			return 0, err
		}
		return rs / targetRate, nil
	}

	// Катушки и принтеры: по одной сумме.
	for _, t := range []struct {
		table, amount, cur, id string
	}{
		{"spools", "price", "currency", "id"},
		{"printers", "purchase_price", "currency", "id"},
	} {
		rows, err := tx.Query(fmt.Sprintf(`SELECT %s, %s, %s FROM %s`, t.id, t.amount, t.cur, t.table))
		if err != nil {
			return fmt.Errorf("ERR_FX: %w", err)
		}
		type row struct {
			id     int64
			amount int64
			cur    string
		}
		var list []row
		for rows.Next() {
			var r row
			if err := rows.Scan(&r.id, &r.amount, &r.cur); err != nil {
				rows.Close()
				return fmt.Errorf("ERR_FX: %w", err)
			}
			list = append(list, r)
		}
		rows.Close()
		for _, r := range list {
			f, err := factor(r.cur)
			if err != nil {
				return err
			}
			v := int64(math.Round(float64(r.amount) * f))
			if _, err := tx.Exec(fmt.Sprintf(`UPDATE %s SET %s=?, %s=? WHERE %s=?`,
				t.table, t.amount, t.cur, t.id), v, target, r.id); err != nil {
				return fmt.Errorf("ERR_FX: %w", err)
			}
		}
	}

	// Заказы целиком: итог, скидка-фикс, позиции, оплаты — в исходной валюте заказа.
	rows, err := tx.Query(`SELECT id, total_base, discount_fixed, currency FROM orders`)
	if err != nil {
		return fmt.Errorf("ERR_FX: %w", err)
	}
	type order struct {
		id    int64
		total int64
		fixed int64
		cur   string
	}
	var orders []order
	for rows.Next() {
		var o order
		if err := rows.Scan(&o.id, &o.total, &o.fixed, &o.cur); err != nil {
			rows.Close()
			return fmt.Errorf("ERR_FX: %w", err)
		}
		orders = append(orders, o)
	}
	rows.Close()
	for _, o := range orders {
		f, err := factor(o.cur)
		if err != nil {
			return err
		}
		total := int64(math.Round(float64(o.total) * f))
		fixed := int64(math.Round(float64(o.fixed) * f))
		if _, err := tx.Exec(`UPDATE orders SET total_base=?, discount_fixed=?, currency=? WHERE id=?`,
			total, fixed, target, o.id); err != nil {
			return fmt.Errorf("ERR_FX: %w", err)
		}
		if _, err := tx.Exec(`UPDATE order_items SET cost=CAST(ROUND(cost*?) AS INTEGER),
			price=CAST(ROUND(price*?) AS INTEGER) WHERE order_id=?`, f, f, o.id); err != nil {
			return fmt.Errorf("ERR_FX: %w", err)
		}
		if _, err := tx.Exec(`UPDATE payments SET amount=CAST(ROUND(amount*?) AS INTEGER)
			WHERE order_id=?`, f, o.id); err != nil {
			return fmt.Errorf("ERR_FX: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ERR_FX: %w", err)
	}
	return d.SetSetting("base_currency", target)
}
