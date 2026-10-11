package sqlite

import (
	"encoding/json"
	"testing"

	"3dprint-calc/core/models"
	"3dprint-calc/core/store"
)

func fptr(v float64) *float64 { return &v }
func sptr(v string) *string   { return &v }

// TestBulkEdit: патч применяется частично (остальное intact),
// пишутся только реально изменившиеся работы; edited/orig — как в UpdateManual.
func TestBulkEdit(t *testing.T) {
	db := openTestDB(t)
	jobs := db.Jobs()
	mk := func(fn, origin string) int64 {
		id, err := jobs.Upsert(models.Job{Filename: fn, StartAt: "2026-01-01 10:00",
			Status: "completed", TotalSec: 3600, WeightG: 10, Origin: origin,
			CostSnapshot: `{"v":2,"material":"","cost":5,"price":5,"currency":"RUB",` +
				`"input":{"tariff":5,"powerKW":0.35,"copies":1}}`})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	id1 := mk("a.gcode", "import")
	id2 := mk("b.gcode", "import")
	patch := store.JobBulkPatch{Tariff: fptr(7), Status: sptr("cancelled")}

	prev, err := jobs.BulkPreview([]int64{id1, id2, 0}, patch)
	if err != nil {
		t.Fatalf("BulkPreview: %v", err)
	}
	if len(prev) != 2 {
		t.Fatalf("превью: %+v", prev)
	}
	fields := map[string]bool{}
	for _, c := range prev[0].Changes {
		fields[c.Field] = true
	}
	for _, k := range []string{"tariff", "status", "cost", "price"} {
		if !fields[k] {
			t.Fatalf("нет изменения %q: %+v", k, prev[0].Changes)
		}
	}

	rep, err := jobs.BulkApply([]int64{id1, id2}, patch)
	if err != nil {
		t.Fatalf("BulkApply: %v", err)
	}
	if rep.Updated != 2 {
		t.Fatalf("updated = %d", rep.Updated)
	}
	list, _ := jobs.List(0)
	byID := map[int64]models.Job{}
	for _, j := range list {
		byID[j.ID] = j
	}
	var snap struct {
		Material string             `json:"material"`
		Cost     float64            `json:"cost"`
		Input    map[string]float64 `json:"input"`
	}
	if err := json.Unmarshal([]byte(byID[id1].CostSnapshot), &snap); err != nil {
		t.Fatal(err)
	}
	if snap.Input["tariff"] != 7 || snap.Input["powerKW"] != 0.35 {
		t.Fatalf("input: %+v", snap.Input)
	}
	if byID[id1].Status != "cancelled" || byID[id1].Edited != 1 {
		t.Fatalf("строка: %+v", byID[id1])
	}
	var orig string
	// List не отдаёт orig_snapshot — читаем напрямую.
	if err := db.sql.QueryRow(`SELECT orig_snapshot FROM jobs WHERE id=?`, id1).Scan(&orig); err != nil {
		t.Fatal(err)
	}
	if orig == "" {
		t.Fatal("orig не зафиксирован")
	}

	// Пустой патч ничего не пишет.
	rep, err = jobs.BulkApply([]int64{id1, id2}, store.JobBulkPatch{})
	if err != nil {
		t.Fatalf("пустой патч: %v", err)
	}
	if rep.Updated != 0 {
		t.Fatalf("пустой патч записал: %+v", rep)
	}
	// Значение как текущее — тоже не изменение.
	rep, err = jobs.BulkApply([]int64{id1}, store.JobBulkPatch{Tariff: fptr(7)})
	if err != nil {
		t.Fatalf("same-value: %v", err)
	}
	if rep.Updated != 0 {
		t.Fatalf("same-value записал: %+v", rep)
	}
}

// TestBulkEditSpool: смена катушки перелинковывает все работы и берёт её цену.
func TestBulkEditSpool(t *testing.T) {
	db := openTestDB(t)
	jobs := db.Jobs()
	mats := db.Materials()
	mid, _ := mats.UpsertByCode(models.Material{Code: "PETG-B", Type: "PETG", Density: 1.27})
	sid, _ := mats.AddSpool(models.Spool{MaterialID: mid, Price: 200000, Currency: "RUB",
		WeightInitG: 1000, WeightLeftG: 1000})
	id, err := jobs.Upsert(models.Job{Filename: "s.gcode", StartAt: "2026-01-01 10:00",
		Status: "completed", TotalSec: 3600, WeightG: 10, Origin: "import",
		CostSnapshot: `{"v":2,"material":"","cost":1,"price":1,"currency":"RUB","input":{"copies":1}}`})
	if err != nil {
		t.Fatal(err)
	}
	rep, err := jobs.BulkApply([]int64{id}, store.JobBulkPatch{SpoolID: sid})
	if err != nil {
		t.Fatalf("BulkApply: %v", err)
	}
	if rep.Updated != 1 {
		t.Fatalf("updated = %d", rep.Updated)
	}
	list, _ := jobs.List(0)
	var got models.Job
	for _, j := range list {
		if j.ID == id {
			got = j
		}
	}
	if got.SpoolID != sid {
		t.Fatalf("линк: %d", got.SpoolID)
	}
	var snap struct {
		Material string             `json:"material"`
		Input    map[string]float64 `json:"input"`
	}
	if err := json.Unmarshal([]byte(got.CostSnapshot), &snap); err != nil {
		t.Fatal(err)
	}
	if snap.Material != "PETG-B" || snap.Input["spoolNo"] == 0 {
		t.Fatalf("снапшот: %+v", snap)
	}
	// Цена — по выбранной катушке (2000 ₽/кг × 10 г = 20).
	var full struct {
		Cost float64 `json:"cost"`
	}
	_ = json.Unmarshal([]byte(got.CostSnapshot), &full)
	if full.Cost != 20 {
		t.Fatalf("cost = %v, хочу 20", full.Cost)
	}
}
