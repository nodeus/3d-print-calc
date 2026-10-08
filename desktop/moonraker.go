package main

import (
	"context"
	"time"

	"3dprint-calc/core/importers"
	"3dprint-calc/core/models"
	"3dprint-calc/core/moonraker"
	sqlite "3dprint-calc/store-sqlite"
	"3dprint-calc/store-sqlite/seed"
)

// Moonraker: проверка связи и импорт истории с K1C.
// Импорт в 2 шага: Preview (dry-run) → Apply. Запись — manager+.

// SetMoonrakerURL задаёт адрес Moonraker принтера (manager+).
func (a *App) SetMoonrakerURL(printerID int64, url string) error {
	if err := a.need(models.RoleManager); err != nil {
		return err
	}
	return a.db.Printers().SetMoonrakerURL(printerID, url)
}

// MoonrakerStatus проверяет связь и возвращает версию Moonraker.
func (a *App) MoonrakerStatus(printerID int64) (string, error) {
	p, err := a.findPrinter(printerID)
	if err != nil {
		return "", err
	}
	if p.MoonrakerURL == "" {
		return "", errMoonrakerUnset
	}
	ctx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
	defer cancel()
	return moonraker.New(p.MoonrakerURL).Ping(ctx)
}

// MoonrakerPreview забирает ВСЮ историю и считает dry-run отчёт.
func (a *App) MoonrakerPreview(printerID int64, limit int) (sqlite.ImportReport, error) {
	var rep sqlite.ImportReport
	rep.DryRun = true
	rows, serverTotal, err := a.fetchMoonraker(printerID, limit)
	if err != nil {
		return rep, err
	}
	fresh, exist, err := a.db.ClassifyJobs(rows)
	if err != nil {
		return rep, err
	}
	rep.Total, rep.New, rep.Existing = len(rows), fresh, exist
	rep.ServerTotal = serverTotal
	a.pendingMoonraker = rows
	a.pendingPrinterID = printerID
	a.pendingNew, a.pendingExisting = fresh, exist
	return rep, nil
}

// MoonrakerApply применяет запомненный Preview импорт (manager+).
func (a *App) MoonrakerApply() (sqlite.ImportReport, error) {
	var rep sqlite.ImportReport
	if err := a.need(models.RoleManager); err != nil {
		return rep, err
	}
	if len(a.pendingMoonraker) == 0 {
		return rep, errNoPending()
	}
	st, err := seed.ImportRows(a.db, a.pendingPrinterID, a.pendingMoonraker, "moonraker", a.emitProgress)
	if err != nil {
		return rep, err
	}
	rep.Total, rep.New, rep.Existing, rep.Stubs =
		len(a.pendingMoonraker), a.pendingNew, a.pendingExisting, st.Stubs
	a.pendingMoonraker = nil
	return rep, nil
}

func (a *App) findPrinter(id int64) (models.Printer, error) {
	list, err := a.db.Printers().List()
	if err != nil {
		return models.Printer{}, err
	}
	for _, p := range list {
		if p.ID == id {
			return p, nil
		}
	}
	return models.Printer{}, errPrinterNotFound
}

func (a *App) fetchMoonraker(printerID int64, limit int) ([]importers.PrintJob, int, error) {
	p, err := a.findPrinter(printerID)
	if err != nil {
		return nil, 0, err
	}
	if p.MoonrakerURL == "" {
		return nil, 0, errMoonrakerUnset
	}
	page := int64(limit)
	if page <= 0 || page > 500 {
		page = 250
	}
	ctx, cancel := context.WithTimeout(a.ctx, 5*time.Minute)
	defer cancel()
	var serverTotal int
	jobs, err := moonraker.New(p.MoonrakerURL).HistoryAll(ctx, page,
		func(fetched, total int) {
			serverTotal = total
			a.emitProgress(fetched, total)
		})
	if err != nil {
		return nil, 0, err
	}
	out := make([]importers.PrintJob, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, j.ToPrintJob())
	}
	return out, serverTotal, nil
}

var (
	errMoonrakerUnset  = errMoonraker("ERR_MOONRAKER_UNSET: адрес Moonraker не задан на принтере")
	errPrinterNotFound = errMoonraker("ERR_PRINTER_NOT_FOUND: принтер не найден")
)

type errMoonraker string

func (e errMoonraker) Error() string { return string(e) }
