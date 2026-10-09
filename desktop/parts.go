package main

import (
	"3dprint-calc/core/models"
)

// Закупки запчастей и ТО по принтерам. Деньги — минорные единицы рублей.

// PartDTO — закупка для фронта.
type PartDTO struct {
	ID        int64   `json:"id"`
	PrinterID int64   `json:"printerId"`
	BoughtAt  string  `json:"boughtAt"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	Qty       float64 `json:"qty"`
	Total     float64 `json:"total"`
	Note      string  `json:"note"`
}

func toPartDTO(p models.PartPurchase) PartDTO {
	return PartDTO{
		ID: p.ID, PrinterID: p.PrinterID, BoughtAt: p.BoughtAt, Name: p.Name,
		Price: float64(p.Price) / 100, Qty: p.Qty,
		Total: float64(p.Price) * p.Qty / 100, Note: p.Note,
	}
}

// GetParts возвращает закупки принтера.
func (a *App) GetParts(printerID int64) ([]PartDTO, error) {
	list, err := a.db.Parts().ListByPrinter(printerID)
	if err != nil {
		return nil, err
	}
	out := make([]PartDTO, 0, len(list))
	for _, p := range list {
		out = append(out, toPartDTO(p))
	}
	return out, nil
}

// ListAllParts возвращает все закупки (для отчёта).
func (a *App) ListAllParts() ([]PartDTO, error) {
	list, err := a.db.Parts().ListAll()
	if err != nil {
		return nil, err
	}
	out := make([]PartDTO, 0, len(list))
	for _, p := range list {
		out = append(out, toPartDTO(p))
	}
	return out, nil
}

// SavePart вставляет (ID=0) или обновляет закупку (manager+).
func (a *App) SavePart(p PartDTO) (int64, error) {
	if err := a.need(models.RoleManager); err != nil {
		return 0, err
	}
	return a.db.Parts().Save(models.PartPurchase{
		ID: p.ID, PrinterID: p.PrinterID, BoughtAt: p.BoughtAt, Name: p.Name,
		Price: int64(p.Price*100 + 0.5), Qty: p.Qty, Note: p.Note,
	})
}

// DeletePart удаляет закупку (manager+).
func (a *App) DeletePart(id int64) error {
	if err := a.need(models.RoleManager); err != nil {
		return err
	}
	return a.db.Parts().Delete(id)
}
