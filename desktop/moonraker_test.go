package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"3dprint-calc/core/importers"
	"3dprint-calc/core/models"
)

const moonrakerHistoryJSON = `{"result": {"count": 1, "jobs": [
  {"filename": "moon_PETG-NIT-W4_1h0m.gcode", "status": "completed",
   "start_time": 1759495020, "end_time": 1759498620,
   "print_duration": 3600, "total_duration": 3600, "filament_used": 10000,
   "metadata": {"estimated_time": 3500, "slicer": "OrcaSlicer"}}
]}}`

// TestMoonrakerPreviewApply гоняет Preview→Apply против фейкового Moonraker.
func TestMoonrakerPreviewApply(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/server/info":
			w.Write([]byte(`{"result": {"moonraker_version": "v0.9.3"}}`))
		case "/server/history/list":
			w.Write([]byte(moonrakerHistoryJSON))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "app.db")
	db, err := openDB(path)
	if err != nil {
		t.Fatalf("openDB: %v", err)
	}
	defer db.Close()
	app := &App{ctx: context.Background(), db: db, dbPath: path}
	if _, err := app.Login("admin", "admin123"); err != nil {
		t.Fatal(err)
	}
	// Чистый старт принтеров не даёт — создаём свой.
	pid, err := app.AddPrinter(models.Printer{
		Name: "K1C-test", LifeYears: 3, HoursPerDay: 8,
		CalendarMode: "calendar", AmortMethod: "calendar", PowerKW: 0.35,
	})
	if err != nil {
		t.Fatalf("AddPrinter: %v", err)
	}
	if _, err := app.MoonrakerStatus(pid); err == nil {
		t.Fatal("статус без URL обязан вернуть ошибку")
	}
	if err := app.SetMoonrakerURL(pid, srv.URL); err != nil {
		t.Fatalf("SetMoonrakerURL: %v", err)
	}
	ver, err := app.MoonrakerStatus(pid)
	if err != nil || ver != "v0.9.3" {
		t.Fatalf("Status = %q, %v", ver, err)
	}
	rep, err := app.MoonrakerPreview(pid, 50)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if !rep.DryRun || rep.Total != 1 || rep.New != 1 {
		t.Fatalf("preview = %+v", rep)
	}
	applied, err := app.MoonrakerApply(nil)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if applied.New != 1 {
		t.Fatalf("applied = %+v", applied)
	}
	// Повторный preview: всё существующее.
	rep2, err := app.MoonrakerPreview(pid, 50)
	if err != nil {
		t.Fatalf("Preview2: %v", err)
	}
	if rep2.New != 0 || rep2.Existing != 1 {
		t.Fatalf("preview2 = %+v", rep2)
	}
	jobs, err := app.GetJobs(0)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, j := range jobs {
		if j.Filename == "moon_PETG-NIT-W4_1h0m.gcode" && j.Slicer == "OrcaSlicer" {
			found = true
		}
	}
	if !found {
		t.Fatalf("импортированной работы нет: %+v", jobs)
	}
}

// TestMoonrakerPreviewAllPages: сервер с 250+ работами — preview забирает всё.
func TestMoonrakerPreviewAllPages(t *testing.T) {
	const totalJobs = 260
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/server/info" {
			w.Write([]byte(`{"result": {"moonraker_version": "v0.9.3"}}`))
			return
		}
		if r.URL.Path == "/server/history/totals" {
			w.Write([]byte(`{"result": {"job_totals": {"total_jobs": 260}}}`))
			return
		}
		q := r.URL.Query()
		limit, _ := strconv.Atoi(q.Get("limit"))
		start, _ := strconv.Atoi(q.Get("start"))
		if limit <= 0 {
			limit = 100
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, `{"result": {"count": %d, "jobs": [`, totalJobs)
		n := 0
		for i := start; i < start+limit && i < totalJobs; i++ {
			if n > 0 {
				sb.WriteString(",")
			}
			fmt.Fprintf(&sb, `{"filename": "bulk_%03d.gcode", "status": "completed",
				"start_time": %d, "end_time": %d,
				"print_duration": 100, "total_duration": 120, "filament_used": 1000,
				"metadata": {}}`, i, 1759495020+i*100, 1759495120+i*100)
			n++
		}
		sb.WriteString(`]}}`)
		w.Write([]byte(sb.String()))
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "app.db")
	db, err := openDB(path)
	if err != nil {
		t.Fatalf("openDB: %v", err)
	}
	defer db.Close()
	app := &App{ctx: context.Background(), db: db, dbPath: path}
	if _, err := app.Login("admin", "admin123"); err != nil {
		t.Fatal(err)
	}
	pid, err := app.AddPrinter(models.Printer{
		Name: "K1C-bulk", LifeYears: 3, HoursPerDay: 8,
		CalendarMode: "calendar", AmortMethod: "calendar", PowerKW: 0.35,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.SetMoonrakerURL(pid, srv.URL); err != nil {
		t.Fatal(err)
	}
	rep, err := app.MoonrakerPreview(pid, 0)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if rep.Total != totalJobs || rep.New != totalJobs || rep.LifetimeTotal != totalJobs {
		t.Fatalf("preview = %+v, хочу все %d", rep, totalJobs)
	}
	applied, err := app.MoonrakerApply(nil)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if applied.New != totalJobs {
		t.Fatalf("applied = %+v", applied)
	}
	jobs, err := app.GetJobs(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != totalJobs {
		t.Fatalf("jobs = %d, хочу %d", len(jobs), totalJobs)
	}
}

// TestMoonrakerPreviewLimit: лимит берёт N последних (новые первыми),
// остальное сервер не отдаёт в выборку.
func TestMoonrakerPreviewLimit(t *testing.T) {
	const totalJobs = 260
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/server/info" {
			w.Write([]byte(`{"result": {"moonraker_version": "v0.9.3"}}`))
			return
		}
		if r.URL.Path == "/server/history/totals" {
			w.Write([]byte(`{"result": {"job_totals": {"total_jobs": 260}}}`))
			return
		}
		q := r.URL.Query()
		limit, _ := strconv.Atoi(q.Get("limit"))
		start, _ := strconv.Atoi(q.Get("start"))
		if limit <= 0 {
			limit = 100
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, `{"result": {"count": %d, "jobs": [`, totalJobs)
		n := 0
		for i := start; i < start+limit && i < totalJobs; i++ {
			if n > 0 {
				sb.WriteString(",")
			}
			fmt.Fprintf(&sb, `{"filename": "bulk_%03d.gcode", "status": "completed",
				"start_time": %d, "end_time": %d,
				"print_duration": 100, "total_duration": 120, "filament_used": 1000,
				"metadata": {}}`, i, 1759495020+i*100, 1759495120+i*100)
			n++
		}
		sb.WriteString(`]}}`)
		w.Write([]byte(sb.String()))
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "app.db")
	db, err := openDB(path)
	if err != nil {
		t.Fatalf("openDB: %v", err)
	}
	defer db.Close()
	app := &App{ctx: context.Background(), db: db, dbPath: path}
	if _, err := app.Login("admin", "admin123"); err != nil {
		t.Fatal(err)
	}
	pid, err := app.AddPrinter(models.Printer{
		Name: "K1C-lim", LifeYears: 3, HoursPerDay: 8,
		CalendarMode: "calendar", AmortMethod: "calendar", PowerKW: 0.35,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.SetMoonrakerURL(pid, srv.URL); err != nil {
		t.Fatal(err)
	}
	rep, err := app.MoonrakerPreview(pid, 100)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if rep.Total != 100 || rep.New != 100 {
		t.Fatalf("preview = %+v, хочу 100 первых", rep)
	}
	if _, err := app.MoonrakerApply(nil); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	jobs, err := app.GetJobs(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 100 {
		t.Fatalf("jobs = %d, хочу 100", len(jobs))
	}
	for _, j := range jobs {
		if j.Filename < "bulk_000.gcode" || j.Filename > "bulk_099.gcode" {
			t.Fatalf("лишняя работа: %q", j.Filename)
		}
	}
}

// TestMoonrakerApplySelected: применяются только отмеченные строки;
// пустой выбор и плохой индекс — ошибки; отмена сбрасывает pending.
func TestMoonrakerApplySelected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/server/info":
			w.Write([]byte(`{"result": {"moonraker_version": "v0.9.3"}}`))
		case "/server/history/list":
			w.Write([]byte(moonrakerHistoryJSON))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "app.db")
	db, err := openDB(path)
	if err != nil {
		t.Fatalf("openDB: %v", err)
	}
	defer db.Close()
	app := &App{ctx: context.Background(), db: db, dbPath: path}
	if _, err := app.Login("admin", "admin123"); err != nil {
		t.Fatal(err)
	}
	pid, err := app.AddPrinter(models.Printer{
		Name: "K1C-sel", LifeYears: 3, HoursPerDay: 8,
		CalendarMode: "calendar", AmortMethod: "calendar", PowerKW: 0.35,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.SetMoonrakerURL(pid, srv.URL); err != nil {
		t.Fatal(err)
	}
	rep, err := app.MoonrakerPreview(pid, 50)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if len(rep.Rows) != 1 || rep.Rows[0].Exists {
		t.Fatalf("preview rows = %+v", rep.Rows)
	}
	// Пустой выбор — ошибка, pending жив.
	if _, err := app.MoonrakerApply([]int{}); err == nil ||
		!strings.Contains(err.Error(), "ERR_NO_SELECTION") {
		t.Fatalf("пустой выбор обязан дать ERR_NO_SELECTION: %v", err)
	}
	// Плохой индекс — ошибка.
	if _, err := app.MoonrakerApply([]int{7}); err == nil ||
		!strings.Contains(err.Error(), "ERR_SELECTION") {
		t.Fatalf("плохой индекс обязан дать ERR_SELECTION: %v", err)
	}
	// Отмена — pending сброшен, apply невозможен.
	app.CancelPendingImport()
	if _, err := app.MoonrakerApply(nil); err == nil ||
		!strings.Contains(err.Error(), "ERR_NO_PENDING") {
		t.Fatalf("после отмены обязан быть ERR_NO_PENDING: %v", err)
	}
	// Повторный preview + выбор единственной строки — импорт идёт.
	if _, err := app.MoonrakerPreview(pid, 50); err != nil {
		t.Fatalf("Preview2: %v", err)
	}
	applied, err := app.MoonrakerApply([]int{0})
	if err != nil {
		t.Fatalf("Apply selected: %v", err)
	}
	if applied.New != 1 || applied.Total != 1 {
		t.Fatalf("applied = %+v", applied)
	}
}

// TestSelectImportRows: nil — всё, подмножество по порядку, пусто и
// вне диапазона — ошибки с кодами.
func TestSelectImportRows(t *testing.T) {
	rows := []importers.PrintJob{{Filename: "a"}, {Filename: "b"}, {Filename: "c"}}
	all, err := selectImportRows(rows, nil)
	if err != nil || len(all) != 3 {
		t.Fatalf("nil = %+v, %v", all, err)
	}
	sub, err := selectImportRows(rows, []int{2, 0})
	if err != nil || len(sub) != 2 || sub[0].Filename != "c" || sub[1].Filename != "a" {
		t.Fatalf("subset = %+v, %v", sub, err)
	}
	if _, err := selectImportRows(rows, []int{}); err == nil ||
		!strings.Contains(err.Error(), "ERR_NO_SELECTION") {
		t.Fatalf("пусто: %v", err)
	}
	if _, err := selectImportRows(rows, []int{3}); err == nil ||
		!strings.Contains(err.Error(), "ERR_SELECTION") {
		t.Fatalf("индекс 3: %v", err)
	}
}
