package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"3dprint-calc/core/calc"
	"3dprint-calc/core/models"
	"3dprint-calc/core/store"
)

// Массовая правка истории: патч разреженный — применяются только непустые
// поля, причём запись идёт лишь при реальных отличиях (old != new).
// Превью считает без записи; применение — одной транзакцией.

// bulkQuerier — общий доступ для db и tx.
type bulkQuerier interface {
	QueryRow(query string, args ...any) *sql.Row
	Exec(query string, args ...any) (sql.Result, error)
}

// snapInputMap разбирает input снапшота v2 (толерантно к отсутствию).
func snapInputMap(raw string) map[string]any {
	var obj struct {
		Input map[string]any `json:"input"`
	}
	if err := json.Unmarshal([]byte(raw), &obj); err != nil || obj.Input == nil {
		return map[string]any{}
	}
	return obj.Input
}

func bulkNum(m map[string]any, k string) float64 {
	v, _ := m[k].(float64)
	return v
}

func bulkSnapCost(raw string) (float64, float64, string, string) {
	var obj struct {
		Material string  `json:"material"`
		Cost     float64 `json:"cost"`
		Price    float64 `json:"price"`
		Currency string  `json:"currency"`
	}
	_ = json.Unmarshal([]byte(raw), &obj)
	return obj.Cost, obj.Price, obj.Material, obj.Currency
}

type bulkComputed struct {
	id       int64
	filename string
	status   string
	startAt  string
	endAt    string
	printSec int64
	totalSec int64
	weightG  float64
	lengthM  float64
	slicer   string
	material string
	spoolID  int64
	spoolNo  int
	input    map[string]any
	cost     float64
	price    float64
	currency string
	changes  []store.BulkChange
}

func (r *jobsRepo) bulkCompute(q bulkQuerier, id int64, p store.JobBulkPatch) (*bulkComputed, error) {
	var cur models.Job
	var edited int
	var spoolNull sql.NullInt64
	err := q.QueryRow(`SELECT filename, status, start_at, end_at, print_sec, total_sec,
		estimated_sec, size_bytes, weight_g, length_m, slicer, cost_snapshot, origin, edited,
		spool_id FROM jobs WHERE id=?`, id).Scan(
		&cur.Filename, &cur.Status, &cur.StartAt, &cur.EndAt, &cur.PrintSec, &cur.TotalSec,
		&cur.EstimatedSec, &cur.SizeBytes, &cur.WeightG, &cur.LengthM, &cur.Slicer,
		&cur.CostSnapshot, &cur.Origin, &edited, &spoolNull)
	if err == sql.ErrNoRows {
		return nil, nil // удалена между выбором и применением — молча пропускаем
	}
	if err != nil {
		return nil, fmt.Errorf("ERR_JOB_SAVE: %w", err)
	}
	if spoolNull.Valid {
		cur.SpoolID = spoolNull.Int64
	}
	oldCost, oldPrice, oldMaterial, oldCurrency := bulkSnapCost(cur.CostSnapshot)
	oldInput := snapInputMap(cur.CostSnapshot)
	out := &bulkComputed{id: id, filename: cur.Filename}
	ch := func(field string, old, new any) {
		out.changes = append(out.changes, store.BulkChange{Field: field, Old: old, New: new})
	}

	// Скаляры.
	out.status, out.startAt, out.endAt = cur.Status, cur.StartAt, cur.EndAt
	out.printSec, out.totalSec = cur.PrintSec, cur.TotalSec
	out.weightG, out.lengthM, out.slicer = cur.WeightG, cur.LengthM, cur.Slicer
	if p.Status != nil && *p.Status != cur.Status {
		ch("status", cur.Status, *p.Status)
		out.status = *p.Status
	}
	if p.StartAt != nil && *p.StartAt != cur.StartAt {
		ch("startAt", cur.StartAt, *p.StartAt)
		out.startAt = *p.StartAt
	}
	if p.EndAt != nil && *p.EndAt != cur.EndAt {
		ch("endAt", cur.EndAt, *p.EndAt)
		out.endAt = *p.EndAt
	}
	if p.PrintSec != nil && *p.PrintSec != cur.PrintSec {
		ch("printSec", cur.PrintSec, *p.PrintSec)
		out.printSec = *p.PrintSec
	}
	if p.TotalSec != nil && *p.TotalSec != cur.TotalSec {
		ch("totalSec", cur.TotalSec, *p.TotalSec)
		out.totalSec = *p.TotalSec
	}
	if p.WeightG != nil && *p.WeightG != cur.WeightG {
		ch("weightG", cur.WeightG, *p.WeightG)
		out.weightG = *p.WeightG
	}
	if p.LengthM != nil && *p.LengthM != cur.LengthM {
		ch("lengthM", cur.LengthM, *p.LengthM)
		out.lengthM = *p.LengthM
	}
	if p.Slicer != nil && *p.Slicer != cur.Slicer {
		ch("slicer", cur.Slicer, *p.Slicer)
		out.slicer = *p.Slicer
	}

	// Материал и катушка.
	out.material = oldMaterial
	out.spoolID = cur.SpoolID
	pricePerKg := 0.0
	spoolNo := int(bulkNum(oldInput, "spoolNo"))
	if p.SpoolID > 0 {
		var price int64
		var mno int
		var code string
		if err := q.QueryRow(`SELECT price, COALESCE(spool_no, 0), m.code
			FROM spools s JOIN materials m ON m.id = s.material_id
			WHERE s.id=?`, p.SpoolID).Scan(&price, &mno, &code); err == nil {
			pricePerKg = float64(price) / 100
			spoolNo = mno
			out.spoolID = p.SpoolID
			if p.Material == "" && code != oldMaterial {
				ch("material", oldMaterial, code)
				out.material = code
			}
		}
	}
	if p.Material != "" && p.Material != out.material {
		ch("material", out.material, p.Material)
		out.material = p.Material
	}
	if out.material != "" && pricePerKg == 0 {
		if m, err := r.materialPrice(out.material); err == nil {
			pricePerKg = m
		}
	}
	oldSpoolNo := int(bulkNum(oldInput, "spoolNo"))
	if spoolNo != oldSpoolNo {
		ch("spoolNo", oldSpoolNo, spoolNo)
	}

	// Расчётный контекст: старый input + overlay патча.
	inp := map[string]any{}
	for k, v := range oldInput {
		inp[k] = v
	}
	setF := func(key string, v *float64) {
		if v != nil {
			inp[key] = *v
		}
	}
	setF("tariff", p.Tariff)
	setF("powerKW", p.PowerKW)
	setF("amortPerHour", p.AmortPerHour)
	setF("maintPerHour", p.MaintPerHour)
	setF("purgeWasteG", p.PurgeWasteG)
	setF("opRate", p.OpRate)
	setF("prepH", p.PrepH)
	setF("postH", p.PostH)
	setF("designerRate", p.DesignerRate)
	setF("modelH", p.ModelH)
	setF("modelPrice", p.ModelPrice)
	setF("extraMat", p.ExtraMat)
	setF("failRate", p.FailRate)
	setF("marginProfit", p.MarginProfit)
	setF("marginDev", p.MarginDev)
	setF("fees", p.Fees)
	setF("vatRate", p.VATRate)
	setF("roundStep", p.RoundStep)
	copies := int(bulkNum(inp, "copies"))
	if p.Copies != nil {
		copies = *p.Copies
	}
	if copies < 1 {
		copies = 1
	}
	inp["copies"] = float64(copies)
	for k, v := range map[string]*float64{
		"tariff": p.Tariff, "powerKW": p.PowerKW, "amortPerHour": p.AmortPerHour,
		"maintPerHour": p.MaintPerHour, "purgeWasteG": p.PurgeWasteG, "opRate": p.OpRate,
		"prepH": p.PrepH, "postH": p.PostH, "designerRate": p.DesignerRate,
		"modelH": p.ModelH, "modelPrice": p.ModelPrice, "extraMat": p.ExtraMat,
		"failRate": p.FailRate, "marginProfit": p.MarginProfit, "marginDev": p.MarginDev,
		"fees": p.Fees, "vatRate": p.VATRate, "roundStep": p.RoundStep,
	} {
		if v != nil && *v != bulkNum(oldInput, k) {
			ch(k, bulkNum(oldInput, k), *v)
		}
	}
	if p.Copies != nil && float64(copies) != bulkNum(oldInput, "copies") {
		ch("copies", bulkNum(oldInput, "copies"), float64(copies))
	}
	res := calc.Calculate(calc.Input{
		TotalSec: float64(out.totalSec), WeightG: out.weightG, PricePerKg: pricePerKg,
		Tariff: bulkNum(inp, "tariff"), PowerKW: bulkNum(inp, "powerKW"),
		AmortPerHour: bulkNum(inp, "amortPerHour"), MaintPerHour: bulkNum(inp, "maintPerHour"),
		PurgeWasteG: bulkNum(inp, "purgeWasteG"), OpRate: bulkNum(inp, "opRate"),
		PrepH: bulkNum(inp, "prepH"), PostH: bulkNum(inp, "postH"),
		DesignerRate: bulkNum(inp, "designerRate"), ModelH: bulkNum(inp, "modelH"),
		ModelPrice: bulkNum(inp, "modelPrice"), ExtraMat: bulkNum(inp, "extraMat"),
		FailRate: bulkNum(inp, "failRate"), MarginProfit: bulkNum(inp, "marginProfit"),
		MarginDev: bulkNum(inp, "marginDev"), Fees: bulkNum(inp, "fees"),
		Copies: copies, VATRate: bulkNum(inp, "vatRate"), RoundStep: bulkNum(inp, "roundStep"),
	})
	out.cost, out.price = res.Cost, res.PriceVAT
	if res.Cost != oldCost {
		ch("cost", oldCost, res.Cost)
	}
	if res.PriceVAT != oldPrice {
		ch("price", oldPrice, res.PriceVAT)
	}
	inp["spoolNo"] = float64(spoolNo)
	out.input = inp
	out.currency = oldCurrency
	if out.currency == "" {
		out.currency = "RUB"
	}
	if out.changes == nil {
		out.changes = []store.BulkChange{}
	}
	return out, nil
}

// BulkPreview считает превью массовой правки без записи.
func (r *jobsRepo) BulkPreview(ids []int64, p store.JobBulkPatch) ([]store.BulkJobPreview, error) {
	out := []store.BulkJobPreview{}
	for _, id := range ids {
		if id == 0 {
			continue
		}
		c, err := r.bulkCompute(r.db, id, p)
		if err != nil {
			return nil, err
		}
		if c == nil {
			continue
		}
		out = append(out, store.BulkJobPreview{
			JobID: c.id, Filename: c.filename, Cost: c.cost, Price: c.price, Changes: c.changes,
		})
	}
	return out, nil
}

// BulkApply применяет массовую правку одной транзакцией: пишутся только
// работы с реальными отличиями. Семантика edited/orig — как в UpdateManual.
func (r *jobsRepo) BulkApply(ids []int64, p store.JobBulkPatch) (store.BulkApplyReport, error) {
	var rep store.BulkApplyReport
	tx, err := r.db.Begin()
	if err != nil {
		return rep, fmt.Errorf("ERR_JOB_SAVE: %w", err)
	}
	defer tx.Rollback()
	for _, id := range ids {
		if id == 0 {
			continue
		}
		c, err := r.bulkCompute(tx, id, p)
		if err != nil {
			return rep, err
		}
		if c == nil || len(c.changes) == 0 {
			continue
		}
		var edited int
		var orig string
		var curOrigin string
		if err := tx.QueryRow(`SELECT edited, orig_snapshot, origin FROM jobs WHERE id=?`,
			id).Scan(&edited, &orig, &curOrigin); err != nil {
			return rep, fmt.Errorf("ERR_JOB_SAVE: %w", err)
		}
		if edited == 0 && curOrigin != "" && curOrigin != "manual" {
			var cur models.Job
			if err := tx.QueryRow(`SELECT filename, status, start_at, end_at, print_sec, total_sec,
				estimated_sec, size_bytes, weight_g, length_m, slicer, cost_snapshot
				FROM jobs WHERE id=?`, id).Scan(
				&cur.Filename, &cur.Status, &cur.StartAt, &cur.EndAt, &cur.PrintSec, &cur.TotalSec,
				&cur.EstimatedSec, &cur.SizeBytes, &cur.WeightG, &cur.LengthM, &cur.Slicer,
				&cur.CostSnapshot); err != nil {
				return rep, fmt.Errorf("ERR_JOB_SAVE: %w", err)
			}
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
		snap, _ := json.Marshal(map[string]any{
			"v": 2, "material": c.material, "cost": c.cost, "price": c.price,
			"currency": c.currency, "input": c.input,
		})
		if _, err := tx.Exec(`UPDATE jobs SET status=?, start_at=?, end_at=?,
			print_sec=?, total_sec=?, weight_g=?, length_m=?,
			slicer=?, spool_id=?, cost_snapshot=?, edited=?, orig_snapshot=? WHERE id=?`,
			c.status, c.startAt, c.endAt, c.printSec, c.totalSec, c.weightG, c.lengthM,
			c.slicer, nullInt(c.spoolID), string(snap), edited, orig, id); err != nil {
			return rep, fmt.Errorf("ERR_JOB_SAVE: %w", err)
		}
		rep.Updated++
	}
	return rep, tx.Commit()
}
