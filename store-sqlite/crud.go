package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"3dprint-calc/core/calc"
	"3dprint-calc/core/models"
	"3dprint-calc/core/store"
)

// CRUD справочников: правки и удаления с блокировкой при живых связях.
// Снапшоты в jobs хранят код материала строкой, поэтому удаление материала
// блокируется только живыми катушками (история при этом не портится).

// boolInt — bool в 0/1 для SQLite.
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Update правит материал. Код правится тоже: уникальность проверяется,
// material в снапшотах работ переписывается (история сохраняет связь).
func (r *materialsRepo) Update(m models.Material) error {
	if m.ID == 0 {
		return fmt.Errorf("ERR_MATERIAL_SAVE: пустой id")
	}
	if m.Code == "" {
		return fmt.Errorf("ERR_MATERIAL_CODE: пустой код")
	}
	var old string
	if err := r.db.QueryRow(`SELECT code FROM materials WHERE id=?`, m.ID).Scan(&old); err != nil {
		return fmt.Errorf("ERR_MATERIAL_SAVE: %w", err)
	}
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("ERR_MATERIAL_SAVE: %w", err)
	}
	defer tx.Rollback()
	if m.Code != old {
		var clash int64
		if err := tx.QueryRow(`SELECT id FROM materials WHERE code=? AND id<>?`, m.Code, m.ID).Scan(&clash); err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("ERR_MATERIAL_SAVE: %w", err)
		}
		if clash != 0 {
			return fmt.Errorf("ERR_MATERIAL_CODE_USED: код %q занят", m.Code)
		}
		if _, err := tx.Exec(`UPDATE materials SET code=? WHERE id=?`, m.Code, m.ID); err != nil {
			return fmt.Errorf("ERR_MATERIAL_SAVE: %w", err)
		}
		// Перепривязка истории: битые/пустые снапшоты пропускаются сами.
		if _, err := tx.Exec(`UPDATE jobs SET cost_snapshot = json_replace(cost_snapshot,
			'$.material', ?) WHERE json_extract(COALESCE(cost_snapshot, '{}'), '$.material') = ?`,
			m.Code, old); err != nil {
			return fmt.Errorf("ERR_MATERIAL_SAVE: %w", err)
		}
	}
	if _, err := tx.Exec(`UPDATE materials SET type=?, vendor=?, color_hex=?,
		density=?, diameter=?, default_spool_weight_g=?, note=?, bought_at=?, archived=? WHERE id=?`,
		m.Type, m.Vendor, m.ColorHex, m.Density, m.Diameter, m.DefaultSpoolWeightG, m.Note,
		m.BoughtAt, boolInt(m.Archived), m.ID); err != nil {
		return fmt.Errorf("ERR_MATERIAL_SAVE: %w", err)
	}
	return tx.Commit()
}

// Delete удаляет материал. Пустые катушки (без привязанных работ) удаляются
// каскадом в той же транзакции. Блок: есть работы, ссылающиеся на катушки.
func (r *materialsRepo) Delete(id int64) error {
	var n int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM jobs j
		JOIN spools s ON s.id = j.spool_id WHERE s.material_id=?`, id).Scan(&n); err != nil {
		return fmt.Errorf("ERR_MATERIAL_DELETE: %w", err)
	}
	if n > 0 {
		return fmt.Errorf("ERR_MATERIAL_USED: %d работ ссылаются на катушки материала", n)
	}
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("ERR_MATERIAL_DELETE: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM spools WHERE material_id=?`, id); err != nil {
		return fmt.Errorf("ERR_MATERIAL_DELETE: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM materials WHERE id=?`, id); err != nil {
		return fmt.Errorf("ERR_MATERIAL_DELETE: %w", err)
	}
	if err := tx.Commit(); err != nil {
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

// DeleteSpool удаляет партию. Блок: есть работы, привязанные к катушке.
func (r *materialsRepo) DeleteSpool(id int64) error {
	var n int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM jobs WHERE spool_id=?`, id).Scan(&n); err != nil {
		return fmt.Errorf("ERR_SPOOL_DELETE: %w", err)
	}
	if n > 0 {
		return fmt.Errorf("ERR_SPOOL_USED: %d работ ссылаются на катушку", n)
	}
	if _, err := r.db.Exec(`DELETE FROM spools WHERE id=?`, id); err != nil {
		return fmt.Errorf("ERR_SPOOL_DELETE: %w", err)
	}
	return nil
}

// SetMaterialPrice ставит цену первой катушки (создаёт катушку при отсутствии).
// Остальные поля катушки не трогает — в отличие от UpdateSpool.
// Новая катушка — весом default_spool_weight_g материала.
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
		w := 1000.0
		var def float64
		if err := r.db.QueryRow(`SELECT default_spool_weight_g FROM materials WHERE id=?`,
			materialID).Scan(&def); err == nil && def > 0 {
			w = def
		}
		_, err := r.AddSpool(models.Spool{
			MaterialID: materialID, Price: priceMinor, Currency: currency, FxRate: 1,
			WeightInitG: w, WeightLeftG: w,
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

// Delete удаляет принтер. Блок: есть работы или закупки на нём.
func (r *printersRepo) Delete(id int64) error {
	var n int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM jobs WHERE printer_id=?`, id).Scan(&n); err != nil {
		return fmt.Errorf("ERR_PRINTER_DELETE: %w", err)
	}
	if n > 0 {
		return fmt.Errorf("ERR_PRINTER_USED: %d работ ссылаются на принтер", n)
	}
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM part_purchases WHERE printer_id=?`, id).Scan(&n); err != nil {
		return fmt.Errorf("ERR_PRINTER_DELETE: %w", err)
	}
	if n > 0 {
		return fmt.Errorf("ERR_PRINTER_USED: %d закупок ссылаются на принтер", n)
	}
	if _, err := r.db.Exec(`DELETE FROM printers WHERE id=?`, id); err != nil {
		return fmt.Errorf("ERR_PRINTER_DELETE: %w", err)
	}
	return nil
}

// snapshotCurrency вытаскивает валюту из снапшота (v2), иначе RUB.
func snapshotCurrency(raw string) string {
	var snap struct {
		Currency string `json:"currency"`
	}
	if err := json.Unmarshal([]byte(raw), &snap); err != nil || snap.Currency == "" {
		return "RUB"
	}
	return snap.Currency
}

// UpdateManual правит работу истории (manual и import) и пересчитывает
// себестоимость И цену строго по переданному расчётному контексту in.
// Цена материала — текущая (первая катушка). Снапшот — v2, валюта — из
// старого снапшота (RUB по умолчанию). Первая правка импортированной
// фиксирует первозданное в orig_snapshot для отката (edited=1).
func (r *jobsRepo) UpdateManual(j models.Job, material string, in store.JobCalcInput) error {
	if j.ID == 0 {
		return fmt.Errorf("ERR_JOB_SAVE: пустой id")
	}
	var cur models.Job
	var edited int
	var orig string
	err := r.db.QueryRow(`SELECT filename, status, start_at, end_at, print_sec, total_sec,
		estimated_sec, size_bytes, weight_g, length_m, slicer, cost_snapshot, origin, edited, orig_snapshot
		FROM jobs WHERE id=?`, j.ID).Scan(
		&cur.Filename, &cur.Status, &cur.StartAt, &cur.EndAt, &cur.PrintSec, &cur.TotalSec,
		&cur.EstimatedSec, &cur.SizeBytes, &cur.WeightG, &cur.LengthM, &cur.Slicer,
		&cur.CostSnapshot, &cur.Origin, &edited, &orig)
	if err != nil {
		return fmt.Errorf("ERR_JOB_SAVE: %w", err)
	}
	allowManual := cur.Origin == "" || cur.Origin == "manual" || cur.Origin == "import" || cur.Origin == "moonraker"
	if !allowManual {
		return fmt.Errorf("ERR_JOB_SAVE: править можно только работы истории")
	}
	// Первая правка импортированной: фиксируем первозданное для отката.
	if edited == 0 && cur.Origin != "" && cur.Origin != "manual" {
		raw, _ := json.Marshal(map[string]any{
			"filename": cur.Filename, "status": cur.Status,
			"start_at": cur.StartAt, "end_at": cur.EndAt,
			"print_sec": cur.PrintSec, "total_sec": cur.TotalSec,
			"estimated_sec": cur.EstimatedSec, "size_bytes": cur.SizeBytes,
			"weight_g": cur.WeightG, "length_m": cur.LengthM,
			"slicer": cur.Slicer, "snapshot": cur.CostSnapshot,
		})
		orig = string(raw)
		edited = 1
	}
	pricePerKg := 0.0
	if material != "" {
		if m, err := r.materialPrice(material); err == nil {
			pricePerKg = m
		}
	}
	copies := in.Copies
	if copies < 1 {
		copies = 1
	}
	res := calc.Calculate(calc.Input{
		TotalSec: float64(j.TotalSec), WeightG: j.WeightG, PricePerKg: pricePerKg,
		Tariff: in.Tariff, PowerKW: in.PowerKW, AmortPerHour: in.AmortPerHour,
		MaintPerHour: in.MaintPerHour, PurgeWasteG: in.PurgeWasteG,
		OpRate: in.OpRate, PrepH: in.PrepH, PostH: in.PostH,
		DesignerRate: in.DesignerRate, ModelH: in.ModelH, ModelPrice: in.ModelPrice,
		ExtraMat: in.ExtraMat, FailRate: in.FailRate,
		MarginProfit: in.MarginProfit, MarginDev: in.MarginDev, Fees: in.Fees,
		Copies: copies, VATRate: in.VATRate, RoundStep: in.RoundStep,
	})
	ctx := map[string]any{
		"tariff": in.Tariff, "powerKW": in.PowerKW,
		"amortPerHour": in.AmortPerHour, "opRate": in.OpRate,
		"prepH": in.PrepH, "postH": in.PostH,
		"designerRate": in.DesignerRate, "modelH": in.ModelH,
		"modelPrice": in.ModelPrice, "extraMat": in.ExtraMat,
		"failRate": in.FailRate, "marginProfit": in.MarginProfit,
		"marginDev": in.MarginDev, "fees": in.Fees,
		"maintPerHour": in.MaintPerHour, "purgeWasteG": in.PurgeWasteG,
		"copies": copies, "vatRate": in.VATRate, "roundStep": in.RoundStep,
	}
	currency := snapshotCurrency(cur.CostSnapshot)
	snap, _ := json.Marshal(map[string]any{
		"v": 2, "material": material, "cost": res.Cost, "price": res.PriceVAT, "currency": currency,
		"input": ctx,
	})
	if _, err := r.db.Exec(`UPDATE jobs SET filename=?, status=?, start_at=?, end_at=?,
		print_sec=?, total_sec=?, estimated_sec=?, size_bytes=?, weight_g=?, length_m=?,
		slicer=?, cost_snapshot=?, edited=?, orig_snapshot=? WHERE id=?`,
		j.Filename, j.Status, j.StartAt, j.EndAt, j.PrintSec, j.TotalSec,
		j.EstimatedSec, j.SizeBytes, j.WeightG, j.LengthM, j.Slicer, string(snap),
		edited, orig, j.ID); err != nil {
		return fmt.Errorf("ERR_JOB_SAVE: %w", err)
	}
	return nil
}

// RevertJob откатывает изменённую импортированную работу к первозданному
// состоянию (поля + снапшот), снимает флаг edited. Только edited=1.
func (r *jobsRepo) RevertJob(id int64) error {
	var edited int
	var orig string
	if err := r.db.QueryRow(`SELECT edited, orig_snapshot FROM jobs WHERE id=?`, id).
		Scan(&edited, &orig); err != nil {
		return fmt.Errorf("ERR_JOB_REVERT: %w", err)
	}
	if edited == 0 || orig == "" {
		return fmt.Errorf("ERR_JOB_REVERT: нечего откатывать")
	}
	var o struct {
		Filename     string  `json:"filename"`
		Status       string  `json:"status"`
		StartAt      string  `json:"start_at"`
		EndAt        string  `json:"end_at"`
		PrintSec     int64   `json:"print_sec"`
		TotalSec     int64   `json:"total_sec"`
		EstimatedSec int64   `json:"estimated_sec"`
		SizeBytes    int64   `json:"size_bytes"`
		WeightG      float64 `json:"weight_g"`
		LengthM      float64 `json:"length_m"`
		Slicer       string  `json:"slicer"`
		Snapshot     string  `json:"snapshot"`
	}
	if err := json.Unmarshal([]byte(orig), &o); err != nil {
		return fmt.Errorf("ERR_JOB_REVERT: битый orig_snapshot: %w", err)
	}
	if _, err := r.db.Exec(`UPDATE jobs SET filename=?, status=?, start_at=?, end_at=?,
		print_sec=?, total_sec=?, estimated_sec=?, size_bytes=?, weight_g=?, length_m=?,
		slicer=?, cost_snapshot=?, edited=0, orig_snapshot='' WHERE id=?`,
		o.Filename, o.Status, o.StartAt, o.EndAt, o.PrintSec, o.TotalSec,
		o.EstimatedSec, o.SizeBytes, o.WeightG, o.LengthM, o.Slicer, o.Snapshot, id); err != nil {
		return fmt.Errorf("ERR_JOB_REVERT: %w", err)
	}
	return nil
}

// materialPrice возвращает текущую цену кг (первая катушка) или 0.
func (r *jobsRepo) materialPrice(code string) (float64, error) {
	var id int64
	if err := r.db.QueryRow(`SELECT id FROM materials WHERE code=?`, code).Scan(&id); err != nil {
		return 0, err
	}
	var price int64
	if err := r.db.QueryRow(`SELECT price FROM spools WHERE material_id=? ORDER BY archived, id LIMIT 1`, id).Scan(&price); err != nil {
		return 0, err
	}
	return float64(price) / 100, nil
}
