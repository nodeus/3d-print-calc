package main

import (
	"path/filepath"
	"testing"

	"3dprint-calc/core/models"
)

// TestUserSettingsFile: круговая запись секции пользователя и окна,
// нет файла — пустые дефолты, гость не пишет.
func TestUserSettingsFile(t *testing.T) {
	dir := t.TempDir()
	app := &App{dbPath: filepath.Join(dir, "app.db"), currentUser: &models.User{Login: "boss"}}
	// Нет файла — пусто, не ошибка.
	got, err := app.GetMyUISettings()
	if err != nil {
		t.Fatalf("GetMyUISettings: %v", err)
	}
	if got.Theme != "" || len(got.Widths) != 0 {
		t.Fatalf("дефолт не пуст: %+v", got)
	}
	in := UserUISettings{
		Theme: "Dracula", Locale: "en", HistoryColumns: []string{"filename", "cost"},
		Widths:         map[string]map[string]float64{"history": {"cost": 120}},
		ColOrder:       map[string][]string{"materials": {"vendor", "code"}},
		OrdersPerPage:  25,
		HistoryPerPage: 100,
		CalcDefaults:   map[string]any{"tariff": 8.5},
	}
	if err := app.SaveMyUISettings(in); err != nil {
		t.Fatalf("SaveMyUISettings: %v", err)
	}
	got, err = app.GetMyUISettings()
	if err != nil {
		t.Fatalf("GetMyUISettings: %v", err)
	}
	if got.Theme != "Dracula" || got.Locale != "en" || len(got.HistoryColumns) != 2 {
		t.Fatalf("секция: %+v", got)
	}
	if got.Widths["history"]["cost"] != 120 {
		t.Fatalf("ширины: %+v", got.Widths)
	}
	if len(got.ColOrder["materials"]) != 2 || got.ColOrder["materials"][0] != "vendor" {
		t.Fatalf("порядок: %+v", got.ColOrder)
	}
	if got.CalcDefaults["tariff"] != 8.5 {
		t.Fatalf("дефолты: %+v", got.CalcDefaults)
	}
	if got.OrdersPerPage != 25 || got.HistoryPerPage != 100 {
		t.Fatalf("пагинация: %+v", got)
	}
	// Другой пользователь — своя пустая секция.
	app.currentUser = &models.User{Login: "mgr"}
	got, _ = app.GetMyUISettings()
	if got.Theme != "" {
		t.Fatalf("чуждая секция: %+v", got)
	}
	// Гость не пишет.
	app.currentUser = nil
	if err := app.SaveMyUISettings(in); err == nil {
		t.Fatal("гость обязан получить ошибку")
	}
	// Окно кругом.
	if err := app.saveWindowSettings(WindowSettings{W: 1300, H: 900, X: 10, Y: 20}); err != nil {
		t.Fatalf("saveWindow: %v", err)
	}
	w, err := app.GetWindowSettings()
	if err != nil || w.W != 1300 || w.H != 900 || w.X != 10 || w.Y != 20 {
		t.Fatalf("окно: %+v %v", w, err)
	}
}
