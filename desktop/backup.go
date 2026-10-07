package main

import (
	"bytes"
	"errors"
	"os"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	sqlite "3dprint-calc/store-sqlite"
)

// Импорт/экспорт и бэкап. Все файловые диалоги — нативные (runtime).
// CSV истории: сначала Preview (dry-run), потом Apply. Бэкап аналогично.

// ExportBackup сохраняет backup.zip через диалог. Возвращает путь.
func (a *App) ExportBackup() (string, error) {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: "3dprint-backup.zip",
		Title:           "Сохранить бэкап",
		Filters:         []runtime.FileFilter{{DisplayName: "Backup (*.zip)", Pattern: "*.zip"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := a.db.CreateBackup(f); err != nil {
		return "", err
	}
	return path, nil
}

// PreviewBackup открывает файл, валидирует без записи и запоминает путь.
// Возвращает манифест для показа пользователю перед применением.
func (a *App) PreviewBackup() (sqlite.Manifest, error) {
	var m sqlite.Manifest
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Выбрать бэкап",
		Filters: []runtime.FileFilter{{DisplayName: "Backup (*.zip)", Pattern: "*.zip"}},
	})
	if err != nil || path == "" {
		return m, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return m, err
	}
	m, err = sqlite.InspectBackup(data)
	if err != nil {
		return m, err
	}
	a.pendingFile = data
	a.pendingPath = path
	return m, nil
}

// ApplyBackup применяет запомненный PreviewBackup бэкап: закрывает БД,
// заменяет файл, открывает заново. Перезапуск приложения не нужен.
func (a *App) ApplyBackup() (sqlite.Manifest, error) {
	var m sqlite.Manifest
	if len(a.pendingFile) == 0 {
		return m, errNoPending()
	}
	m, err := sqlite.InspectBackup(a.pendingFile)
	if err != nil {
		return m, err
	}
	if err := a.db.Close(); err != nil {
		return m, err
	}
	if err := sqlite.RestoreAppDB(a.pendingFile, a.dbPath); err != nil {
		// Пытаемся переоткрыть старую БД, чтобы не остаться без соединения.
		a.reopen()
		return m, err
	}
	a.pendingFile = nil
	a.pendingPath = ""
	if err := a.reopen(); err != nil {
		return m, err
	}
	return m, nil
}

// ExportJobsCSV выгружает историю в формате принтера.
func (a *App) ExportJobsCSV() (string, error) {
	return a.saveCSV("print_history-export.csv", "История печати (*.csv)")
}

// ExportMaterialsCSV выгружает справочник материалов.
func (a *App) ExportMaterialsCSV() (string, error) {
	return a.saveCSV("materials-export.csv", "Материалы (*.csv)")
}

func (a *App) saveCSV(defaultName, title string) (string, error) {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: defaultName,
		Title:           title,
		Filters:         []runtime.FileFilter{{DisplayName: "CSV (*.csv)", Pattern: "*.csv"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if defaultName == "materials-export.csv" {
		err = a.db.ExportMaterialsCSV(f)
	} else {
		err = a.db.ExportJobsCSV(f)
	}
	if err != nil {
		return "", err
	}
	return path, nil
}

// PreviewHistoryCSV открывает CSV принтера и считает dry-run отчёт.
func (a *App) PreviewHistoryCSV() (sqlite.ImportReport, error) {
	var rep sqlite.ImportReport
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Выбрать CSV истории",
		Filters: []runtime.FileFilter{{DisplayName: "CSV (*.csv)", Pattern: "*.csv"}},
	})
	if err != nil || path == "" {
		return rep, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return rep, err
	}
	rep, err = a.db.ImportHistory(bytes.NewReader(data), true)
	if err != nil {
		return rep, err
	}
	a.pendingFile = data
	a.pendingPath = path
	return rep, nil
}

// ApplyHistoryCSV применяет запомненный PreviewHistoryCSV импорт.
func (a *App) ApplyHistoryCSV() (sqlite.ImportReport, error) {
	var rep sqlite.ImportReport
	if len(a.pendingFile) == 0 {
		return rep, errNoPending()
	}
	rep, err := a.db.ImportHistory(bytes.NewReader(a.pendingFile), false)
	if err != nil {
		return rep, err
	}
	a.pendingFile = nil
	a.pendingPath = ""
	return rep, nil
}

func errNoPending() error {
	return errPending
}

var errPending = errors.New("ERR_NO_PENDING: сначала выберите файл (preview)")

// reopen открывает БД после restore.
func (a *App) reopen() error {
	db, err := openDB(a.dbPath)
	if err != nil {
		return err
	}
	a.db = db
	return nil
}
