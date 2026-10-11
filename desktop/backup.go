package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"3dprint-calc/core/export"
	"3dprint-calc/core/importers"
	"3dprint-calc/core/models"
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
// заменяет файл, открывает заново. Перезапуск приложения не нужен (manager+).
func (a *App) ApplyBackup() (sqlite.Manifest, error) {
	var m sqlite.Manifest
	if err := a.need(models.RoleManager); err != nil {
		return m, err
	}
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
// limitRows — максимум последних работ (<=0 — все), запоминается для Apply.
func (a *App) PreviewHistoryCSV(limitRows int) (sqlite.ImportReport, error) {
	var rep sqlite.ImportReport
	if len(a.pendingMoonraker) > 0 || len(a.pendingMats) > 0 {
		return rep, errPendingExists
	}
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
	rep, err = a.db.ImportHistory(bytes.NewReader(data), true, limitRows, nil)
	if err != nil {
		return rep, err
	}
	a.pendingFile = data
	a.pendingPath = path
	a.pendingCSVLimit = limitRows
	return rep, nil
}

// emitProgress шлёт прогресс импорта во фронт (событие import-progress).
// Без wails-контекста (тесты) — молча пропускает: runtime.EventsEmit
// с чужим контекстом роняет процесс.
func (a *App) emitProgress(done, total int) {
	if !a.eventsOn {
		return
	}
	runtime.EventsEmit(a.ctx, "import-progress", map[string]any{"done": done, "total": total})
}

// ExportPDF строит A4-отчёт из секций (заголовки+строки уже отформатированы
// фронтом) и сохраняет через диалог. Шапка — имя и версия приложения,
// футер каждой страницы — nodeus.ru. filename — желаемое имя файла
// (санитизируется).
func (a *App) ExportPDF(title, filename string, sections []export.Section) (string, error) {
	if filename == "" {
		filename = "report.pdf"
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: filename,
		Title:           title,
		Filters:         []runtime.FileFilter{{DisplayName: "PDF (*.pdf)", Pattern: "*.pdf"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	data, err := export.BuildPDF(
		fmt.Sprintf("3D print Cost Calculator v%s (%s)", Version, BuildDate),
		title, sections)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}
	return path, nil
}

// ApplyHistoryCSV применяет запомненный PreviewHistoryCSV импорт (manager+).
// selected — индексы строк предпросмотра (nil — все); пустой список — ошибка.
func (a *App) ApplyHistoryCSV(selected []int) (sqlite.ImportReport, error) {
	var rep sqlite.ImportReport
	rep.Rows = []sqlite.ImportRow{}
	if err := a.need(models.RoleManager); err != nil {
		return rep, err
	}
	if len(a.pendingFile) == 0 {
		return rep, errNoPending()
	}
	rows, err := importers.ParseHistory(bytes.NewReader(a.pendingFile))
	if err != nil {
		return rep, err
	}
	rows = sqlite.TakeLastJobs(rows, a.pendingCSVLimit)
	picked, err := selectImportRows(rows, selected)
	if err != nil {
		return rep, err
	}
	rep.Total = len(picked)
	fresh, exist, err := a.db.ClassifyJobs(picked)
	if err != nil {
		return rep, err
	}
	rep.New, rep.Existing = fresh, exist
	if rep.New == 0 {
		a.pendingFile = nil
		a.pendingPath = ""
		a.pendingCSVLimit = 0
		return rep, nil
	}
	applied, err := a.db.ImportSelected(picked, a.emitProgress)
	if err != nil {
		return rep, err
	}
	applied.DryRun = false
	a.pendingFile = nil
	a.pendingPath = ""
	a.pendingCSVLimit = 0
	return applied, nil
}

// CancelPendingImport сбрасывает запомненный Preview импорт
// (история CSV, Moonraker, материалы). Отдельных прав не требует.
func (a *App) CancelPendingImport() {
	a.clearPendingImport()
}

// clearPendingImport очищает все pending-буферы импорта (история, Moonraker, материалы).
func (a *App) clearPendingImport() {
	a.pendingFile = nil
	a.pendingPath = ""
	a.pendingCSVLimit = 0
	a.pendingMats = nil
	a.pendingMatsLimit = 0
	a.pendingMoonraker = nil
	a.pendingPrinterID = 0
	a.pendingNew, a.pendingExisting = 0, 0
}

// selectIndices оставляет строки предпросмотра по индексам (порядок —
// как в предпросмотре). nil — все строки; пустой список — ERR_NO_SELECTION;
// индекс вне диапазона — ERR_SELECTION.
func selectIndices[T any](rows []T, selected []int) ([]T, error) {
	if selected == nil {
		return rows, nil
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("ERR_NO_SELECTION: не выбрана ни одна строка")
	}
	out := make([]T, 0, len(selected))
	for _, i := range selected {
		if i < 0 || i >= len(rows) {
			return nil, fmt.Errorf("ERR_SELECTION: плохой индекс строки %d", i)
		}
		out = append(out, rows[i])
	}
	return out, nil
}

// selectImportRows — выборка строк истории (см. selectIndices).
func selectImportRows(rows []importers.PrintJob, selected []int) ([]importers.PrintJob, error) {
	return selectIndices(rows, selected)
}

// PreviewMaterialsCSV читает CSV справочника в dry-run (отдельный буфер).
// limitRows — максимум последних строк файла (<=0 — все), запоминается для Apply.
func (a *App) PreviewMaterialsCSV(limitRows int) (sqlite.ImportReport, error) {
	var rep sqlite.ImportReport
	rep.Rows = []sqlite.ImportRow{}
	rep.MatRows = []sqlite.ImportMatRow{}
	if len(a.pendingFile) > 0 || len(a.pendingMoonraker) > 0 {
		return rep, errPendingExists
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Импорт CSV материалов",
		Filters: []runtime.FileFilter{{DisplayName: "CSV (*.csv)", Pattern: "*.csv"}},
	})
	if err != nil || path == "" {
		return rep, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return rep, err
	}
	rep, err = a.db.ImportMaterials(bytes.NewReader(data), true, limitRows, nil)
	if err != nil {
		return rep, err
	}
	a.pendingMats = data
	a.pendingMatsLimit = limitRows
	return rep, nil
}

// ApplyMaterialsCSV применяет запомненный PreviewMaterialsCSV импорт (manager+).
// selected — индексы строк предпросмотра (nil — все); пустой список — ошибка.
func (a *App) ApplyMaterialsCSV(selected []int) (sqlite.ImportReport, error) {
	var rep sqlite.ImportReport
	rep.Rows = []sqlite.ImportRow{}
	rep.MatRows = []sqlite.ImportMatRow{}
	if err := a.need(models.RoleManager); err != nil {
		return rep, err
	}
	if len(a.pendingMats) == 0 {
		return rep, errNoPending()
	}
	rows, err := importers.ParseMaterials(bytes.NewReader(a.pendingMats))
	if err != nil {
		return rep, err
	}
	rows = sqlite.TakeLastMats(rows, a.pendingMatsLimit)
	picked, err := selectIndices(rows, selected)
	if err != nil {
		return rep, err
	}
	applied, err := a.db.ImportMatsSelected(picked, a.emitProgress)
	if err != nil {
		return rep, err
	}
	a.pendingMats = nil
	a.pendingMatsLimit = 0
	return applied, nil
}

func errNoPending() error {
	return errPending
}

var errPending = errors.New("ERR_NO_PENDING: сначала выберите файл (preview)")

// errPendingExists — импорт только последовательный: предпросмотр при
// занятом чужом буфере запрещён (порядок применения влияет на результат:
// материалы меняют катушки и цены, история привязывается к катушкам).
var errPendingExists = errors.New("ERR_PENDING_EXISTS: сначала примените или отмените текущий импорт")

// reopen открывает БД после restore.
func (a *App) reopen() error {
	db, err := openDB(a.dbPath)
	if err != nil {
		return err
	}
	a.db = db
	return nil
}
