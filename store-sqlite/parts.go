package sqlite

import (
	"database/sql"
	"fmt"

	"3dprint-calc/core/models"
)

type partsRepo struct{ db *sql.DB }

// scanPart читает строку закупок.
func scanPart(rows *sql.Rows) (models.PartPurchase, error) {
	var p models.PartPurchase
	err := rows.Scan(&p.ID, &p.PrinterID, &p.BoughtAt, &p.Name, &p.Price, &p.Qty, &p.Note)
	return p, err
}

// Save вставляет (ID=0) или обновляет закупку.
func (r *partsRepo) Save(p models.PartPurchase) (int64, error) {
	if p.PrinterID == 0 {
		return 0, fmt.Errorf("ERR_PART_SAVE: нет принтера")
	}
	if p.Qty <= 0 {
		p.Qty = 1
	}
	if p.ID == 0 {
		res, err := r.db.Exec(`INSERT INTO part_purchases (printer_id, bought_at, name, price, qty, note)
			VALUES (?, ?, ?, ?, ?, ?)`,
			p.PrinterID, p.BoughtAt, p.Name, p.Price, p.Qty, p.Note)
		if err != nil {
			return 0, fmt.Errorf("ERR_PART_SAVE: %w", err)
		}
		id, _ := res.LastInsertId()
		return id, nil
	}
	if _, err := r.db.Exec(`UPDATE part_purchases SET printer_id=?, bought_at=?, name=?,
		price=?, qty=?, note=? WHERE id=?`,
		p.PrinterID, p.BoughtAt, p.Name, p.Price, p.Qty, p.Note, p.ID); err != nil {
		return 0, fmt.Errorf("ERR_PART_SAVE: %w", err)
	}
	return p.ID, nil
}

// ListByPrinter возвращает закупки принтера (сначала новые).
func (r *partsRepo) ListByPrinter(printerID int64) ([]models.PartPurchase, error) {
	rows, err := r.db.Query(`SELECT id, printer_id, bought_at, name, price, qty, note
		FROM part_purchases WHERE printer_id=? ORDER BY bought_at DESC, id DESC`, printerID)
	if err != nil {
		return nil, fmt.Errorf("ERR_PART_LIST: %w", err)
	}
	defer rows.Close()
	out := []models.PartPurchase{}
	for rows.Next() {
		p, err := scanPart(rows)
		if err != nil {
			return nil, fmt.Errorf("ERR_PART_LIST: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListAll возвращает все закупки (для отчёта), сначала новые.
func (r *partsRepo) ListAll() ([]models.PartPurchase, error) {
	rows, err := r.db.Query(`SELECT id, printer_id, bought_at, name, price, qty, note
		FROM part_purchases ORDER BY bought_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("ERR_PART_LIST: %w", err)
	}
	defer rows.Close()
	out := []models.PartPurchase{}
	for rows.Next() {
		p, err := scanPart(rows)
		if err != nil {
			return nil, fmt.Errorf("ERR_PART_LIST: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Delete удаляет закупку.
func (r *partsRepo) Delete(id int64) error {
	if _, err := r.db.Exec(`DELETE FROM part_purchases WHERE id=?`, id); err != nil {
		return fmt.Errorf("ERR_PART_DELETE: %w", err)
	}
	return nil
}
