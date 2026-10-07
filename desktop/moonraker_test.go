package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

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
	applied, err := app.MoonrakerApply()
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
