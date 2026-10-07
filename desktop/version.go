package main

// Версия приложения. BuildDate прошивается при релизной сборке:
//
//	wails build -ldflags "-X main.BuildDate=2026-10-08"
//
// По умолчанию — dev.
var (
	Version   = "0.2.0"
	BuildDate = "dev"
)

// AppVersion отдаёт версию фронту (вкладка Информация).
func (a *App) AppVersion() map[string]string {
	return map[string]string{"version": Version, "buildDate": BuildDate}
}
