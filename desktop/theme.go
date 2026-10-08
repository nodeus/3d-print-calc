package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"3dprint-calc/core/models"
)

// Темы оформления: light/dark встроены (CSS), кастомные — JSON в settings.
// Печать всегда светлая (см. theme.css @media print).

// UISettingsDTO — тема и кастомные наборы для фронта.
type UISettingsDTO struct {
	Theme  string                       `json:"theme"`
	Custom map[string]map[string]string `json:"custom"`
}

var hexColor = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

// UISettings возвращает тему и кастомные наборы.
func (a *App) UISettings() (UISettingsDTO, error) {
	out := UISettingsDTO{Theme: "light", Custom: map[string]map[string]string{}}
	if v, ok, _ := a.db.GetSetting("ui_theme"); ok && v != "" {
		out.Theme = v
	}
	if v, ok, _ := a.db.GetSetting("ui_custom_themes"); ok && v != "" {
		var custom map[string]map[string]string
		if err := json.Unmarshal([]byte(v), &custom); err == nil {
			out.Custom = custom
		}
	}
	if out.Custom == nil {
		out.Custom = map[string]map[string]string{}
	}
	return out, nil
}

// SetUITheme переключает тему (light/dark/имя кастомной).
func (a *App) SetUITheme(name string) error {
	if name == "" {
		name = "light"
	}
	return a.db.SetSetting("ui_theme", name)
}

// SaveCustomTheme сохраняет/обновляет кастомный набор (manager+).
func (a *App) SaveCustomTheme(name string, vars map[string]string) error {
	if err := a.need(models.RoleManager); err != nil {
		return err
	}
	if name == "" || name == "light" || name == "dark" {
		return fmt.Errorf("ERR_THEME: плохое имя %q", name)
	}
	for k, v := range vars {
		if !hexColor.MatchString(v) {
			return fmt.Errorf("ERR_THEME: %s: не hex-цвет %q", k, v)
		}
	}
	s, err := a.UISettings()
	if err != nil {
		return err
	}
	s.Custom[name] = vars
	raw, _ := json.Marshal(s.Custom)
	return a.db.SetSetting("ui_custom_themes", string(raw))
}

// DeleteCustomTheme удаляет кастомный набор (manager+).
func (a *App) DeleteCustomTheme(name string) error {
	if err := a.need(models.RoleManager); err != nil {
		return err
	}
	s, err := a.UISettings()
	if err != nil {
		return err
	}
	delete(s.Custom, name)
	raw, _ := json.Marshal(s.Custom)
	if err := a.db.SetSetting("ui_custom_themes", string(raw)); err != nil {
		return err
	}
	if cur, ok, _ := a.db.GetSetting("ui_theme"); ok && cur == name {
		return a.db.SetSetting("ui_theme", "light")
	}
	return nil
}

// baseThemes — встроенные темы (зеркало theme.css): их нет в Custom,
// но экспорт в файл обязан работать и для них.
var baseThemes = map[string]map[string]string{
	"light": {
		"bg": "#fafafa", "fg": "#222222", "card-border": "#dddddd",
		"nav-bg": "#e5e7eb", "header-bg": "#1f2937", "header-fg": "#ffffff",
		"btn-bg": "#ffffff", "btn-fg": "#222222", "btn-border": "#cccccc",
		"input-bg": "#ffffff", "input-fg": "#222222",
		"muted": "#666666", "err": "#bb0000", "warn": "#bb6600",
		"accent": "#0645ad", "sel-bg": "#eeeeff",
		"final-border": "#333333", "sw-border": "#999999",
	},
	"dark": {
		"bg": "#1a1d21", "fg": "#e6e6e6", "card-border": "#3a3f45",
		"nav-bg": "#24282e", "header-bg": "#101216", "header-fg": "#ffffff",
		"btn-bg": "#2a2e34", "btn-fg": "#e6e6e6", "btn-border": "#4a5058",
		"input-bg": "#24282e", "input-fg": "#e6e6e6",
		"muted": "#9aa0a6", "err": "#ff6b6b", "warn": "#ffb020",
		"accent": "#7aa2ff", "sel-bg": "#2c3a55",
		"final-border": "#8a8f96", "sw-border": "#6a6f76",
	},
}

// ExportThemeFile сохраняет набор в JSON-файл.
func (a *App) ExportThemeFile(name string) (string, error) {
	vars, ok := baseThemes[name]
	if !ok {
		s, err := a.UISettings()
		if err != nil {
			return "", err
		}
		vars, ok = s.Custom[name]
		if !ok {
			return "", fmt.Errorf("ERR_THEME: нет темы %q", name)
		}
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: name + ".theme.json",
		Title:           name,
		Filters:         []runtime.FileFilter{{DisplayName: "Theme (*.json)", Pattern: "*.json"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	raw, _ := json.MarshalIndent(map[string]any{"name": name, "vars": vars}, "", "  ")
	if err := os.WriteFile(path, raw, 0644); err != nil {
		return "", err
	}
	return path, nil
}

// ImportThemeFile читает тему из JSON-файла и сохраняет (manager+).
func (a *App) ImportThemeFile() (string, error) {
	if err := a.need(models.RoleManager); err != nil {
		return "", err
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Импорт темы",
		Filters: []runtime.FileFilter{{DisplayName: "Theme (*.json)", Pattern: "*.json"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var doc struct {
		Name string            `json:"name"`
		Vars map[string]string `json:"vars"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return "", fmt.Errorf("ERR_THEME: битый JSON: %w", err)
	}
	if err := a.SaveCustomTheme(doc.Name, doc.Vars); err != nil {
		return "", err
	}
	return doc.Name, nil
}
