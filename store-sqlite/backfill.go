package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"3dprint-calc/core/calc"
)

// BackfillImportSnapshots дозаполняет price и input.spoolNo у нетронутых
// (edited=0) работ, чей снапшот их не содержит. Импорт до fix писал снапшот
// без этих ключей — История показывала цену 0 и голый код материала.
// Ручные правки (edited=1) и заполненные снапшоты не трогаются.
// Цена считается тем же core/calc по сохранённому input и текущей цене
// привязанной катушки (как UpdateManual); номер катушки — по линку spool_id.
// Идемпотентен: повторный прогон ничего не меняет.
func (d *DB) BackfillImportSnapshots() (int, error) {
	rows, err := d.sql.Query(`SELECT id, spool_id, total_sec, weight_g, cost_snapshot
		FROM jobs WHERE edited=0`)
	if err != nil {
		return 0, fmt.Errorf("ERR_BACKFILL_JOBS: %w", err)
	}
	type job struct {
		id      int64
		spoolID sql.NullInt64
		total   float64
		weight  float64
		snap    string
	}
	var list []job
	for rows.Next() {
		var j job
		if err := rows.Scan(&j.id, &j.spoolID, &j.total, &j.weight, &j.snap); err != nil {
			rows.Close()
			return 0, fmt.Errorf("ERR_BACKFILL_JOBS: %w", err)
		}
		list = append(list, j)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, fmt.Errorf("ERR_BACKFILL_JOBS: %w", err)
	}
	rows.Close()

	num := func(m map[string]any, k string) float64 {
		v, _ := m[k].(float64)
		return v
	}
	fixed := 0
	for _, j := range list {
		var obj map[string]any
		if err := json.Unmarshal([]byte(j.snap), &obj); err != nil {
			continue // битый снапшот — не наша забота
		}
		_, hasPrice := obj["price"]
		inp, _ := obj["input"].(map[string]any)
		_, hasSpoolNo := inp["spoolNo"]
		if hasPrice && (hasSpoolNo || inp == nil) {
			continue
		}
		// Номер и цена привязанной катушки (удаление занятой блокируется,
		// но на случай битого линка — фолбэк на первую катушку материала).
		spoolNo := 0
		pricePerKg := 0.0
		if j.spoolID.Valid {
			var price int64
			var mno int
			if err := d.sql.QueryRow(`SELECT price, COALESCE(spool_no, 0)
				FROM spools WHERE id=?`, j.spoolID.Int64).Scan(&price, &mno); err == nil {
				pricePerKg = float64(price) / 100
				spoolNo = mno
			}
		}
		if mat, _ := obj["material"].(string); pricePerKg == 0 && mat != "" {
			if p, err := materialPriceByCode(d.sql, mat); err == nil {
				pricePerKg = p
			}
		}
		if !hasPrice {
			if inp == nil {
				inp = map[string]any{}
			}
			copies := int(num(inp, "copies"))
			if copies < 1 {
				copies = 1
			}
			res := calc.Calculate(calc.Input{
				TotalSec: j.total, WeightG: j.weight, PricePerKg: pricePerKg,
				Tariff: num(inp, "tariff"), PowerKW: num(inp, "powerKW"),
				AmortPerHour: num(inp, "amortPerHour"), MaintPerHour: num(inp, "maintPerHour"),
				PurgeWasteG: num(inp, "purgeWasteG"), OpRate: num(inp, "opRate"),
				PrepH: num(inp, "prepH"), PostH: num(inp, "postH"),
				DesignerRate: num(inp, "designerRate"), ModelH: num(inp, "modelH"),
				ModelPrice: num(inp, "modelPrice"), ExtraMat: num(inp, "extraMat"),
				FailRate: num(inp, "failRate"), MarginProfit: num(inp, "marginProfit"),
				MarginDev: num(inp, "marginDev"), Fees: num(inp, "fees"),
				Copies: copies, VATRate: num(inp, "vatRate"), RoundStep: num(inp, "roundStep"),
			})
			obj["price"] = res.PriceVAT
		}
		if !hasSpoolNo && inp != nil {
			inp["spoolNo"] = spoolNo
			obj["input"] = inp
		}
		raw, err := json.Marshal(obj)
		if err != nil {
			continue
		}
		if _, err := d.sql.Exec(`UPDATE jobs SET cost_snapshot=? WHERE id=?`, string(raw), j.id); err != nil {
			return fixed, fmt.Errorf("ERR_BACKFILL_JOBS: %w", err)
		}
		fixed++
	}
	return fixed, nil
}
