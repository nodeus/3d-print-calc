package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Пользовательские настройки на диске: user_settings.json рядом с app.db.
// Окно — общее, остальное — по логинам (разные пользователи — разные настройки).
// Нет файла — дефолты; пустые поля секции — фолбэк на legacy-настройки БД.

// WindowSettings — размер и позиция окна.
type WindowSettings struct {
	W int `json:"w"`
	H int `json:"h"`
	X int `json:"x"`
	Y int `json:"y"`
}

// UserUISettings — интерфейс пользователя по логину.
type UserUISettings struct {
	Theme          string                        `json:"theme"`
	Locale         string                        `json:"locale"`
	HistoryColumns []string                      `json:"historyColumns"`
	Widths         map[string]map[string]float64 `json:"widths"`   // таблица → колонка → px
	ColOrder       map[string][]string           `json:"colOrder"` // таблица → порядок ключей колонок
	CalcDefaults   map[string]any                `json:"calcDefaults"`
}

// SettingsFile — корень файла настроек.
type SettingsFile struct {
	Version int                        `json:"version"`
	Window  WindowSettings             `json:"window"`
	Users   map[string]*UserUISettings `json:"users"`
}

const settingsFileVersion = 1
const settingsFileName = "user_settings.json"

var uiSettingsMu sync.Mutex

// settingsPath — файл рядом с БД.
func (a *App) settingsPath() string {
	return filepath.Join(filepath.Dir(a.dbPath), settingsFileName)
}

// loadSettingsFile читает файл (нет/битый — пустой дефолт, не ошибка).
func (a *App) loadSettingsFile() *SettingsFile {
	f := &SettingsFile{Version: settingsFileVersion, Users: map[string]*UserUISettings{}}
	raw, err := os.ReadFile(a.settingsPath())
	if err != nil {
		return f
	}
	var parsed SettingsFile
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return f
	}
	if parsed.Users == nil {
		parsed.Users = map[string]*UserUISettings{}
	}
	for _, u := range parsed.Users {
		if u == nil {
			continue
		}
		if u.Widths == nil {
			u.Widths = map[string]map[string]float64{}
		}
		if u.ColOrder == nil {
			u.ColOrder = map[string][]string{}
		}
	}
	parsed.Version = settingsFileVersion
	return &parsed
}

// saveSettingsFile пишет файл атомарно (через временный).
func (a *App) saveSettingsFile(f *SettingsFile) error {
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("ERR_SETTINGS: %w", err)
	}
	tmp := a.settingsPath() + ".tmp"
	if err := os.WriteFile(tmp, raw, 0644); err != nil {
		return fmt.Errorf("ERR_SETTINGS: %w", err)
	}
	if err := os.Rename(tmp, a.settingsPath()); err != nil {
		return fmt.Errorf("ERR_SETTINGS: %w", err)
	}
	return nil
}

// myLogin — логин текущей сессии (” = гость).
func (a *App) myLogin() string {
	if a.currentUser == nil {
		return ""
	}
	return a.currentUser.Login
}

// GetMyUISettings возвращает секцию настроек текущего пользователя
// (гость — пустая, фронт добьёт дефолтами и legacy-БД).
func (a *App) GetMyUISettings() (UserUISettings, error) {
	uiSettingsMu.Lock()
	defer uiSettingsMu.Unlock()
	out := UserUISettings{Widths: map[string]map[string]float64{}}
	if login := a.myLogin(); login != "" {
		if u := a.loadSettingsFile().Users[login]; u != nil {
			out = *u
			if out.Widths == nil {
				out.Widths = map[string]map[string]float64{}
			}
			if out.ColOrder == nil {
				out.ColOrder = map[string][]string{}
			}
		}
	}
	return out, nil
}

// SaveMyUISettings сохраняет секцию текущего пользователя (любая роль:
// свои интерфейс-настройки правит каждый). Пустой логин — гость, не храним.
func (a *App) SaveMyUISettings(in UserUISettings) error {
	login := a.myLogin()
	if login == "" {
		return fmt.Errorf("ERR_SETTINGS: нет входа")
	}
	uiSettingsMu.Lock()
	defer uiSettingsMu.Unlock()
	f := a.loadSettingsFile()
	if in.Widths == nil {
		in.Widths = map[string]map[string]float64{}
	}
	if in.ColOrder == nil {
		in.ColOrder = map[string][]string{}
	}
	cp := in
	f.Users[login] = &cp
	return a.saveSettingsFile(f)
}

// GetWindowSettings возвращает сохранённое окно (нули = нет, дефолт Wails).
func (a *App) GetWindowSettings() (WindowSettings, error) {
	uiSettingsMu.Lock()
	defer uiSettingsMu.Unlock()
	return a.loadSettingsFile().Window, nil
}

// saveWindowSettings пишет окно (внутреннее, без гейтов — зовёт shutdown).
func (a *App) saveWindowSettings(w WindowSettings) error {
	uiSettingsMu.Lock()
	defer uiSettingsMu.Unlock()
	f := a.loadSettingsFile()
	f.Window = w
	return a.saveSettingsFile(f)
}
