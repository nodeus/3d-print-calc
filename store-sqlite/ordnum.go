package sqlite

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"3dprint-calc/core/ordnum"
)

// Нумерация заказов: шаблон order_number_template + счётчик order_number_seq
// в settings. Дефолты — шаблон {YYYY}-{nnnn}, счётчик с 1.

// NextOrderNumber рендерит следующий номер и двигает счётчик. Атомарно.
// Невалидный шаблон — ошибка без сдвига счётчика.
func (d *DB) NextOrderNumber() (string, error) {
	tx, err := d.sql.Begin()
	if err != nil {
		return "", fmt.Errorf("ERR_ORDNUM_SEQ: %w", err)
	}
	defer tx.Rollback()
	template := ordnum.DefaultTemplate
	var v string
	if err := tx.QueryRow(`SELECT value FROM settings WHERE key='order_number_template'`).Scan(&v); err != nil && err != sql.ErrNoRows {
		return "", fmt.Errorf("ERR_ORDNUM_SEQ: %w", err)
	} else if err == nil && v != "" {
		template = v
	}
	seq := 1
	if err := tx.QueryRow(`SELECT value FROM settings WHERE key='order_number_seq'`).Scan(&v); err != nil && err != sql.ErrNoRows {
		return "", fmt.Errorf("ERR_ORDNUM_SEQ: %w", err)
	} else if err == nil && v != "" {
		n, perr := strconv.Atoi(v)
		if perr != nil || n < 1 {
			return "", fmt.Errorf("ERR_ORDNUM_SEQ: битый счётчик %q", v)
		}
		seq = n
	}
	number, err := ordnum.Render(template, time.Now(), seq)
	if err != nil {
		return "", err
	}
	if _, err := tx.Exec(`INSERT INTO settings (key, value) VALUES ('order_number_seq', ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, strconv.Itoa(seq+1)); err != nil {
		return "", fmt.Errorf("ERR_ORDNUM_SEQ: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("ERR_ORDNUM_SEQ: %w", err)
	}
	return number, nil
}

// OrderNumberSettings возвращает шаблон и счётчик (с дефолтами).
func (d *DB) OrderNumberSettings() (template string, seq int) {
	template = ordnum.DefaultTemplate
	seq = 1
	if v, ok, _ := d.GetSetting("order_number_template"); ok && v != "" {
		template = v
	}
	if v, ok, _ := d.GetSetting("order_number_seq"); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 {
			seq = n
		}
	}
	return template, seq
}
