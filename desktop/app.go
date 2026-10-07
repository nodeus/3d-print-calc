package main

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"3dprint-calc/core/calc"
	"3dprint-calc/core/models"
)

//go:embed all:assets/migrations
var migrationsFS embed.FS

// MaterialDTO — материал с ценой первой катушки для калькулятора.
type MaterialDTO struct {
	ID         int64   `json:"id"`
	Code       string  `json:"code"`
	Type       string  `json:"type"`
	Vendor     string  `json:"vendor"`
	ColorHex   string  `json:"colorHex"`
	Note       string  `json:"note"`
	Density    float64 `json:"density"`
	Diameter   float64 `json:"diameter"`
	PricePerKg float64 `json:"pricePerKg"`
	Currency   string  `json:"currency"`
	SpoolID    int64   `json:"spoolId"`
	SpoolLeftG float64 `json:"spoolLeftG"`
}

// GetMaterials возвращает справочник с ценами.
func (a *App) GetMaterials() ([]MaterialDTO, error) {
	mats, err := a.db.Materials().List()
	if err != nil {
		return nil, err
	}
	out := make([]MaterialDTO, 0, len(mats))
	for _, m := range mats {
		dto := MaterialDTO{
			ID: m.ID, Code: m.Code, Type: m.Type, Vendor: m.Vendor,
			ColorHex: m.ColorHex, Note: m.Note,
			Density: m.Density, Diameter: m.Diameter, Currency: "RUB",
		}
		if spools, err := a.db.Materials().ListSpools(m.ID); err == nil && len(spools) > 0 {
			dto.PricePerKg = float64(spools[0].Price) / 100
			dto.Currency = spools[0].Currency
			dto.SpoolID = spools[0].ID
			dto.SpoolLeftG = spools[0].WeightLeftG
		}
		out = append(out, dto)
	}
	return out, nil
}

// PrinterDTO — принтер с посчитанной часовой амортизацией.
type PrinterDTO struct {
	models.Printer
	AmortPerHour float64 `json:"amortPerHour"`
}

// GetPrinters возвращает парк с амортизацией по методу принтера.
func (a *App) GetPrinters() ([]PrinterDTO, error) {
	list, err := a.db.Printers().List()
	if err != nil {
		return nil, err
	}
	out := make([]PrinterDTO, 0, len(list))
	for _, p := range list {
		out = append(out, PrinterDTO{Printer: p, AmortPerHour: AmortRate(p)})
	}
	return out, nil
}

// UpdateMaterial правит материал (manager+). Код неизменен.
func (a *App) UpdateMaterial(m models.Material) error {
	if err := a.need(models.RoleManager); err != nil {
		return err
	}
	return a.db.Materials().Update(m)
}

// AddMaterial создаёт материал с катушкой (manager+). Цена ₽/кг.
func (a *App) AddMaterial(code, matType string, density, pricePerKg float64) (int64, error) {
	if err := a.need(models.RoleManager); err != nil {
		return 0, err
	}
	if code == "" {
		return 0, errEmptyCode
	}
	if matType == "" {
		matType = "PETG"
	}
	if density <= 0 {
		density = 1.27
	}
	id, err := a.db.Materials().UpsertByCode(models.Material{
		Code: code, Type: matType, Density: density, Diameter: 1.75,
	})
	if err != nil {
		return 0, err
	}
	if pricePerKg > 0 {
		if _, err := a.db.Materials().AddSpool(models.Spool{
			MaterialID: id, Price: int64(pricePerKg*100 + 0.5),
			Currency: "RUB", FxRate: 1, WeightInitG: 1000, WeightLeftG: 1000,
		}); err != nil {
			return 0, err
		}
	}
	return id, nil
}

// DeleteMaterial удаляет материал (manager+). Блок при катушках — в store.
func (a *App) DeleteMaterial(id int64) error {
	if err := a.need(models.RoleManager); err != nil {
		return err
	}
	return a.db.Materials().Delete(id)
}

// UpdateSpool правит партию: цена/валюта/остаток (manager+).
func (a *App) UpdateSpool(s models.Spool) error {
	if err := a.need(models.RoleManager); err != nil {
		return err
	}
	return a.db.Materials().UpdateSpool(s)
}

// SetMaterialPrice ставит цену материала: обновляет первую катушку
// или создаёт её (manager+). Остальное не трогает.
func (a *App) SetMaterialPrice(materialID, priceMinor int64, currency string) error {
	if err := a.need(models.RoleManager); err != nil {
		return err
	}
	return a.db.Materials().SetMaterialPrice(materialID, priceMinor, currency)
}

// AddPrinter создаёт принтер со всеми характеристиками (manager+).
func (a *App) AddPrinter(p models.Printer) (int64, error) {
	if err := a.need(models.RoleManager); err != nil {
		return 0, err
	}
	if p.HoursPerDay <= 0 {
		p.HoursPerDay = 8
	}
	if p.LifeYears <= 0 {
		p.LifeYears = 3
	}
	if p.CalendarMode == "" {
		p.CalendarMode = "calendar"
	}
	if p.AmortMethod == "" {
		p.AmortMethod = "calendar"
	}
	if p.Currency == "" {
		p.Currency = "RUB"
	}
	return a.db.Printers().Save(p)
}

// UpdatePrinter правит принтер целиком (manager+).
func (a *App) UpdatePrinter(p models.Printer) error {
	if err := a.need(models.RoleManager); err != nil {
		return err
	}
	return a.db.Printers().Update(p)
}

// DeletePrinter удаляет принтер (manager+). Блок при работах — в store.
func (a *App) DeletePrinter(id int64) error {
	if err := a.need(models.RoleManager); err != nil {
		return err
	}
	return a.db.Printers().Delete(id)
}

// AmortRate считает ₽/час по методу принтера.
// calendar (дефолт): цена / (календарные дни СПИ × часов/день).
// workdays: будни ≈ calendar × 5/7. resource: цена / ресурс_часов.
func AmortRate(p models.Printer) float64 {
	price := float64(p.PurchasePrice) / 100
	switch p.AmortMethod {
	case "resource":
		if p.ResourceHours > 0 {
			return price / p.ResourceHours
		}
		return 0
	default:
		days := float64(p.LifeYears * 365)
		if start, err := time.Parse("2006-01-02", p.CommissioningDate); err == nil && p.LifeYears > 0 {
			days = start.AddDate(p.LifeYears, 0, 0).Sub(start).Hours() / 24
		}
		if p.CalendarMode == "workdays" || p.AmortMethod == "workdays" {
			days = days * 5 / 7
		}
		return calc.AmortCalendarHour(price, int(days+0.5), p.HoursPerDay)
	}
}

// Calc считает себестоимость/цену — единственная точка расчёта в UI.
func (a *App) Calc(in calc.Input) calc.Result {
	return calc.Calculate(in)
}

// WeightFromLength — вес прутка по плотности (filament_used из истории, мм).
func (a *App) WeightFromLength(density, diameterMM, lengthMM float64) float64 {
	return calc.WeightFromLength(density, diameterMM, lengthMM)
}

// JobInput — сохранение результата калькулятора в историю.
type JobInput struct {
	Filename  string  `json:"filename"`
	PrinterID int64   `json:"printerId"`
	SpoolID   int64   `json:"spoolId"`
	Status    string  `json:"status"`
	TotalSec  int64   `json:"totalSec"`
	WeightG   float64 `json:"weightG"`
	LengthM   float64 `json:"lengthM"`
	Cost      float64 `json:"cost"`
	Price     float64 `json:"price"`
	Currency  string  `json:"currency"`
}

// SaveJob сохраняет расчёт (manager+). StartAt — сейчас (импорт истории — через CSV).
func (a *App) SaveJob(in JobInput) (int64, error) {
	if err := a.need(models.RoleManager); err != nil {
		return 0, err
	}
	if in.Filename == "" {
		in.Filename = "manual-" + time.Now().Format("20060102-150405")
	}
	if in.Status == "" {
		in.Status = "completed"
	}
	snap, _ := json.Marshal(map[string]any{
		"cost": in.Cost, "price": in.Price, "currency": in.Currency,
	})
	return a.db.Jobs().Upsert(models.Job{
		Filename: in.Filename, StartAt: time.Now().Format("2006-01-02 15:04"),
		PrinterID: in.PrinterID, SpoolID: in.SpoolID, Status: in.Status, Origin: "manual",
		PrintSec: in.TotalSec, TotalSec: in.TotalSec,
		WeightG: in.WeightG, LengthM: in.LengthM, CostSnapshot: string(snap),
	})
}

// DeleteJob удаляет работу из истории (manager+, только manual).
func (a *App) DeleteJob(id int64) error {
	if err := a.need(models.RoleManager); err != nil {
		return err
	}
	return a.db.Jobs().DeleteJob(id)
}

// GetJobs возвращает историю (limit<=0 — всё).
func (a *App) GetJobs(limit int) ([]models.Job, error) {
	return a.db.Jobs().List(limit)
}

// GetLocale / SetLocale — язык UI (ru|en), хранится в settings.
func (a *App) GetLocale() string {
	if v, ok, _ := a.db.GetSetting("locale"); ok && (v == "ru" || v == "en") {
		return v
	}
	return "ru"
}

// SetLocale сохраняет язык UI (manager+).
func (a *App) SetLocale(locale string) error {
	if err := a.need(models.RoleManager); err != nil {
		return err
	}
	if locale != "ru" && locale != "en" {
		locale = "ru"
	}
	return a.db.SetSetting("locale", locale)
}

// GetBaseCurrency — базовая валюта (v1: ручной выбор кода ISO).
func (a *App) GetBaseCurrency() string {
	if v, ok, _ := a.db.GetSetting("base_currency"); ok && v != "" {
		return v
	}
	return "RUB"
}

// SetBaseCurrency сохраняет базовую валюту (manager+).
func (a *App) SetBaseCurrency(code string) error {
	if err := a.need(models.RoleManager); err != nil {
		return err
	}
	if code == "" {
		code = "RUB"
	}
	return a.db.SetSetting("base_currency", code)
}

// HistoryColumns — все колонки истории печати (порядок фиксирован).
var HistoryColumns = []string{
	"filename", "start", "end", "status", "duration", "weight", "length",
	"size", "filament", "slicer", "cost", "price", "origin",
}

// DefaultHistoryColumns — видимые по умолчанию.
var DefaultHistoryColumns = []string{
	"filename", "start", "status", "duration", "weight", "cost", "price", "origin",
}

// GetHistoryColumns возвращает видимые колонки истории.
func (a *App) GetHistoryColumns() []string {
	v, ok, _ := a.db.GetSetting("history_columns")
	if !ok || v == "" {
		return DefaultHistoryColumns
	}
	var cols []string
	if err := json.Unmarshal([]byte(v), &cols); err != nil || len(cols) == 0 {
		return DefaultHistoryColumns
	}
	// Фильтруем неизвестные ключи, сохраняя порядок.
	known := map[string]bool{}
	for _, c := range HistoryColumns {
		known[c] = true
	}
	out := make([]string, 0, len(cols))
	for _, c := range cols {
		if known[c] {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return DefaultHistoryColumns
	}
	return out
}

// SetHistoryColumns сохраняет видимые колонки (manager+).
func (a *App) SetHistoryColumns(cols []string) error {
	if err := a.need(models.RoleManager); err != nil {
		return err
	}
	known := map[string]bool{}
	for _, c := range HistoryColumns {
		known[c] = true
	}
	for _, c := range cols {
		if !known[c] {
			return fmt.Errorf("ERR_COLUMNS: неизвестная колонка %q", c)
		}
	}
	raw, _ := json.Marshal(cols)
	return a.db.SetSetting("history_columns", string(raw))
}

var errEmptyCode = errors.New("ERR_MATERIAL_CODE: пустой код")
