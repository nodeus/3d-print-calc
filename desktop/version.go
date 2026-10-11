package main

import (
	"os"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Версия приложения. BuildDate прошивается при релизной сборке:
//
//	wails build -ldflags "-X main.BuildDate=2026-10-08"
//
// По умолчанию — dev.
var (
	Version   = "2.0.0"
	BuildDate = "dev"
)

// AppVersion отдаёт версию фронту (вкладка Информация).
func (a *App) AppVersion() map[string]string {
	return map[string]string{"version": Version, "buildDate": buildDateLabel()}
}

// buildDateLabel — подпись даты сборки для заголовка окна и вкладки Информация.
// Релиз: дата, прошитая ldflags (tools/release). Dev (BuildDate == "dev"):
// дата и время записи exe-файла, т.е. момент сборки; пометка dev сохраняется.
func buildDateLabel() string {
	if BuildDate != "dev" {
		return BuildDate
	}
	if exe, err := os.Executable(); err == nil {
		if st, err := os.Stat(exe); err == nil {
			return "dev " + st.ModTime().Format("2006-01-02 15:04")
		}
	}
	return "dev"
}

// OpenURL открывает ссылку в браузере (автор, донат).
func (a *App) OpenURL(url string) {
	runtime.BrowserOpenURL(a.ctx, url)
}
