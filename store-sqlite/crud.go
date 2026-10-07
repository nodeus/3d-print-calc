package sqlite

import (
	"fmt"

	"3dprint-calc/core/models"
)

// CRUD справочников: правки и удаления с блокировкой при живых связях.
// Снапшоты в jobs хранят код материала строкой, поэтому удаление материала
// блокируется только живыми катушками (история при этом не портится).

// Update правит материал (код неизменен — ключ и сидит в снапшотах).
func (r *materialsRepo) Update(m models.Material) error {
	if m.ID == 0 {
		return fmt.Errorf("ERR_MATERIAL_SAVE: пустой id")
	}
	if _, err := r.db.Exec(`UPDATE materials SET type=?, vendor=?, color_hex=?,
		density=?, diameter=?, note=? WHERE id=?`,
		m.Type, m.Vendor, m.ColorHex, m.Density, m.Diameter, m.Note, m.ID); err != nil {
		return fmt.Errorf("ERR_MATERIAL_SAVE: %w", err)
	}
	return nil
}

// Delete удаляет материал. Блок: есть катушки (включая привязанные работы).
func (r *materialsRepo) Delete(id int64) error {
	var n int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM spools WHERE material_id=?`, id).Scan(&n); err != nil {
		return fmt.Errorf("ERR_MATERIAL_DELETE: %w", err)
	}
	if n > 0 {
		return fmt.Errorf("ERR_MATERIAL_USED: %d катушек ссылаются на материал", n)
	}
	if _, err := r.db.Exec(`DELETE FROM materials WHERE id=?`, id); err != nil {
		return fmt.Errorf("ERR_MATERIAL_DELETE: %w", err)
	}
	return nil
}

// UpdateSpool правит цену/валюту/остаток партии.
func (r *materialsRepo) UpdateSpool(s models.Spool) error {
	if s.ID == 0 {
		return fmt.Errorf("ERR_SPOOL_SAVE: пустой id")
	}
	if _, err := r.db.Exec(`UPDATE spools SET price=?, currency=?, fx_rate=?,
		bought_at=?, weight_init_g=?, weight_left_g=?, archived=? WHERE id=?`,
		s.Price, s.Currency, s.FxRate, s.BoughtAt, s.WeightInitG, s.WeightLeftG, s.Archived, s.ID); err != nil {
		return fmt.Errorf("ERR_SPOOL_SAVE: %w", err)
	}
	return nil
}

// SetMaterialPrice ставит цену первой катушки (создаёт катушку при отсутствии).
// Остальные поля катушки не трогает — в отличие от UpdateSpool.
func (r *materialsRepo) SetMaterialPrice(materialID, priceMinor int64, currency string) error {
	if materialID == 0 {
		return fmt.Errorf("ERR_SPOOL_SAVE: пустой material_id")
	}
	if priceMinor < 0 {
		return fmt.Errorf("ERR_SPOOL_SAVE: отрицательная цена")
	}
	if currency == "" {
		currency = "RUB"
	}
	spools, err := r.ListSpools(materialID)
	if err != nil {
		return err
	}
	if len(spools) == 0 {
		_, err := r.AddSpool(models.Spool{
			MaterialID: materialID, Price: priceMinor, Currency: currency, FxRate: 1,
			WeightInitG: 1000, WeightLeftG: 1000,
		})
		return err
	}
	if _, err := r.db.Exec(`UPDATE spools SET price=?, currency=? WHERE id=?`,
		priceMinor, currency, spools[0].ID); err != nil {
		return fmt.Errorf("ERR_SPOOL_SAVE: %w", err)
	}
	return nil
}

// Update правит принтер целиком по id.
func (r *printersRepo) Update(p models.Printer) error {
	if p.ID == 0 {
		return fmt.Errorf("ERR_PRINTER_SAVE: пустой id")
	}
	if _, err := r.db.Exec(`UPDATE printers SET name=?, model=?, purchase_price=?, currency=?,
		commissioning_date=?, life_years=?, hours_per_day=?, calendar_mode=?, amort_method=?,
		power_kw=?, maint_per_hour=?, resource_hours=?, moonraker_url=? WHERE id=?`,
		p.Name, p.Model, p.PurchasePrice, p.Currency, p.CommissioningDate, p.LifeYears,
		p.HoursPerDay, p.CalendarMode, p.AmortMethod, p.PowerKW, p.MaintPerHour,
		p.ResourceHours, p.MoonrakerURL, p.ID); err != nil {
		return fmt.Errorf("ERR_PRINTER_SAVE: %w", err)
	}
	return nil
}

// Delete удаляет принтер. Блок: есть работы на нём.
func (r *printersRepo) Delete(id int64) error {
	var n int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM jobs WHERE printer_id=?`, id).Scan(&n); err != nil {
		return fmt.Errorf("ERR_PRINTER_DELETE: %w", err)
	}
	if n > 0 {
		return fmt.Errorf("ERR_PRINTER_USED: %d работ ссылаются на принтер", n)
	}
	if _, err := r.db.Exec(`DELETE FROM printers WHERE id=?`, id); err != nil {
		return fmt.Errorf("ERR_PRINTER_DELETE: %w", err)
	}
	return nil
}
