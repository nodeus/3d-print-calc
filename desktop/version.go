package main

import (
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Версия приложения. BuildDate прошивается при релизной сборке:
//
//	wails build -ldflags "-X main.BuildDate=2026-10-08"
//
// По умолчанию — dev.
var (
	Version   = "1.4.7"
	BuildDate = "dev"
)

// AppVersion отдаёт версию фронту (вкладка Информация).
func (a *App) AppVersion() map[string]string {
	return map[string]string{"version": Version, "buildDate": BuildDate}
}

// OpenURL открывает ссылку в браузере (автор, донат).
func (a *App) OpenURL(url string) {
	runtime.BrowserOpenURL(a.ctx, url)
}
