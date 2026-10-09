package main

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"3dprint-calc/core/calc"
	"3dprint-calc/core/models"
	"3dprint-calc/core/store"
	sqlite "3dprint-calc/store-sqlite"
)

//go:embed all:assets/migrations
var migrationsFS embed.FS

// MaterialDTO — материал с ценой первой катушки для калькулятора.
type MaterialDTO struct {
	ID             int64   `json:"id"`
	Code           string  `json:"code"`
	Type           string  `json:"type"`
	Vendor         string  `json:"vendor"`
	ColorHex       string  `json:"colorHex"`
	Note           string  `json:"note"`
	BoughtAt       string  `json:"boughtAt"`
	Archived       bool    `json:"archived"`
	Density        float64 `json:"density"`
	Diameter       float64 `json:"diameter"`
	DefaultWeightG float64 `json:"defaultWeightG"`
	PricePerKg     float64 `json:"pricePerKg"`
	Currency       string  `json:"currency"`
	SpoolID        int64   `json:"spoolId"`
	SpoolLeftG     float64 `json:"spoolLeftG"`
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
			ColorHex: m.ColorHex, Note: m.Note, DefaultWeightG: m.DefaultSpoolWeightG,
			Density: m.Density, Diameter: m.Diameter, Currency: "RUB",
			BoughtAt: m.BoughtAt, Archived: m.Archived,
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
func (a *App) AddMaterial(code, matType string, density, pricePerKg, weightG float64) (int64, error) {
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
	if weightG <= 0 {
		weightG = 1000
	}
	id, err := a.db.Materials().UpsertByCode(models.Material{
		Code: code, Type: matType, Density: density, Diameter: 1.75,
		DefaultSpoolWeightG: weightG,
		BoughtAt:            time.Now().Format("2006-01-02"),
	})
	if err != nil {
		return 0, err
	}
	// UpsertByCode вес по умолчанию не трогает — выставляем явно.
	m, err := a.db.Materials().GetByCode(code)
	if err != nil {
		return 0, err
	}
	m.DefaultSpoolWeightG = weightG
	if err := a.db.Materials().Update(m); err != nil {
		return 0, err
	}
	if pricePerKg > 0 {
		if _, err := a.db.Materials().AddSpool(models.Spool{
			MaterialID: id, Price: int64(pricePerKg*100 + 0.5),
			Currency: "RUB", FxRate: 1, BoughtAt: time.Now().Format("2006-01-02"),
			WeightInitG: weightG, WeightLeftG: weightG,
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

// SpoolDTO — партия для фронта (деньги — основные единицы).
type SpoolDTO struct {
	ID          int64   `json:"id"`
	MaterialID  int64   `json:"materialId"`
	Price       float64 `json:"price"`
	Currency    string  `json:"currency"`
	BoughtAt    string  `json:"boughtAt"`
	WeightInitG float64 `json:"weightInitG"`
	WeightLeftG float64 `json:"weightLeftG"`
	Archived    bool    `json:"archived"`
}

func toSpoolDTO(s models.Spool) SpoolDTO {
	return SpoolDTO{
		ID: s.ID, MaterialID: s.MaterialID, Price: float64(s.Price) / 100,
		Currency: s.Currency, BoughtAt: s.BoughtAt,
		WeightInitG: s.WeightInitG, WeightLeftG: s.WeightLeftG, Archived: s.Archived,
	}
}

// GetSpools возвращает партии материала.
func (a *App) GetSpools(materialID int64) ([]SpoolDTO, error) {
	list, err := a.db.Materials().ListSpools(materialID)
	if err != nil {
		return nil, err
	}
	out := make([]SpoolDTO, 0, len(list))
	for _, s := range list {
		out = append(out, toSpoolDTO(s))
	}
	return out, nil
}

// AddSpool добавляет партию-закупку (manager+). Пустая дата — сегодня.
func (a *App) AddSpool(s SpoolDTO) (int64, error) {
	if err := a.need(models.RoleManager); err != nil {
		return 0, err
	}
	if s.MaterialID == 0 {
		return 0, fmt.Errorf("ERR_SPOOL_SAVE: нет материала")
	}
	if s.BoughtAt == "" {
		s.BoughtAt = time.Now().Format("2006-01-02")
	}
	if s.Currency == "" {
		s.Currency = "RUB"
	}
	if s.WeightInitG <= 0 {
		s.WeightInitG = 1000
	}
	return a.db.Materials().AddSpool(models.Spool{
		MaterialID: s.MaterialID, Price: int64(s.Price*100 + 0.5), Currency: s.Currency,
		FxRate: 1, BoughtAt: s.BoughtAt,
		WeightInitG: s.WeightInitG, WeightLeftG: s.WeightInitG,
	})
}

// DeleteSpool удаляет партию (manager+). Блок при привязанных работах — в store.
func (a *App) DeleteSpool(id int64) error {
	if err := a.need(models.RoleManager); err != nil {
		return err
	}
	return a.db.Materials().DeleteSpool(id)
}

// SetMaterialPrice ставит цену материала: обновляет первую катушку
// или создаёт её (manager+). Остальное не трогает.
func (a *App) SetMaterialPrice(materialID, priceMinor int64, currency string) error {
	if err := a.need(models.RoleManager); err != nil {
		return err
	}
	return a.db.Materials().SetMaterialPrice(materialID, priceMinor, currency)
}

// colorRe — hex-цвет #rrggbb для фаворитов.
var colorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// GetMaterialColorFavs возвращает цвета-фавориты (settings.material_color_favs).
func (a *App) GetMaterialColorFavs() []string {
	v, ok, _ := a.db.GetSetting("material_color_favs")
	if !ok || v == "" {
		return []string{}
	}
	var cols []string
	if err := json.Unmarshal([]byte(v), &cols); err != nil {
		return []string{}
	}
	out := make([]string, 0, len(cols))
	for _, c := range cols {
		if colorRe.MatchString(c) {
			out = append(out, strings.ToLower(c))
		}
	}
	return out
}

// SetMaterialColorFavs сохраняет фавориты (manager+, валидация hex).
func (a *App) SetMaterialColorFavs(cols []string) error {
	if err := a.need(models.RoleManager); err != nil {
		return err
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(cols))
	for _, c := range cols {
		c = strings.ToLower(strings.TrimSpace(c))
		if !colorRe.MatchString(c) || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	raw, _ := json.Marshal(out)
	return a.db.SetSetting("material_color_favs", string(raw))
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
// Material/Tariff/... — контекст расчёта для снапшота v2 (перенос и пересчёт).
type JobInput struct {
	Filename     string  `json:"filename"`
	PrinterID    int64   `json:"printerId"`
	SpoolID      int64   `json:"spoolId"`
	Status       string  `json:"status"`
	TotalSec     int64   `json:"totalSec"`
	WeightG      float64 `json:"weightG"`
	LengthM      float64 `json:"lengthM"`
	Cost         float64 `json:"cost"`
	Price        float64 `json:"price"`
	Currency     string  `json:"currency"`
	Material     string  `json:"material"`
	Tariff       float64 `json:"tariff"`
	PowerKW      float64 `json:"powerKW"`
	AmortPerHour float64 `json:"amortPerHour"`
	OpRate       float64 `json:"opRate"`
	PrepH        float64 `json:"prepH"`
	PostH        float64 `json:"postH"`
	DesignerRate float64 `json:"designerRate"`
	ModelH       float64 `json:"modelH"`
	ModelPrice   float64 `json:"modelPrice"`
	ExtraMat     float64 `json:"extraMat"`
	FailRate     float64 `json:"failRate"`
	MarginProfit float64 `json:"marginProfit"`
	MarginDev    float64 `json:"marginDev"`
	Fees         float64 `json:"fees"`
	MaintPerHour float64 `json:"maintPerHour"`
	PurgeWasteG  float64 `json:"purgeWasteG"`
	Copies       int     `json:"copies"`
	VATRate      float64 `json:"vatRate"`
	RoundStep    float64 `json:"roundStep"`
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
	// Катушка по материалу (вместо вечного 0): для привязки и пересчёта.
	spoolID := in.SpoolID
	if spoolID == 0 && in.Material != "" {
		if m, err := a.db.Materials().GetByCode(in.Material); err == nil {
			if spools, err := a.db.Materials().ListSpools(m.ID); err == nil && len(spools) > 0 {
				spoolID = spools[0].ID
			}
		}
	}
	snap, _ := json.Marshal(map[string]any{
		"v": 2, "material": in.Material, "currency": in.Currency,
		"cost": in.Cost, "price": in.Price,
		"input": map[string]any{
			"tariff": in.Tariff, "powerKW": in.PowerKW,
			"amortPerHour": in.AmortPerHour, "opRate": in.OpRate,
			"prepH": in.PrepH, "postH": in.PostH,
			"designerRate": in.DesignerRate, "modelH": in.ModelH,
			"modelPrice": in.ModelPrice, "extraMat": in.ExtraMat,
			"failRate": in.FailRate, "marginProfit": in.MarginProfit,
			"marginDev": in.MarginDev, "fees": in.Fees,
			"maintPerHour": in.MaintPerHour, "purgeWasteG": in.PurgeWasteG,
			"copies": in.Copies, "vatRate": in.VATRate, "roundStep": in.RoundStep,
		},
	})
	return a.db.Jobs().Upsert(models.Job{
		Filename: in.Filename, StartAt: time.Now().Format("2006-01-02 15:04"),
		PrinterID: in.PrinterID, SpoolID: spoolID, Status: in.Status, Origin: "manual",
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

// JobEdit — поля правки работы истории (материал отдельно: не колонка jobs).
// Расчётные поля (тарифы, ставки, наценки...) фронт шлёт полным набором:
// бэкенд пересчитывает снапшот строго по ним, старый контекст не используется.
type JobEdit struct {
	ID           int64   `json:"id"`
	Filename     string  `json:"filename"`
	Status       string  `json:"status"`
	StartAt      string  `json:"startAt"`
	EndAt        string  `json:"endAt"`
	PrintSec     int64   `json:"printSec"`
	TotalSec     int64   `json:"totalSec"`
	EstimatedSec int64   `json:"estimatedSec"`
	SizeBytes    int64   `json:"sizeBytes"`
	WeightG      float64 `json:"weightG"`
	LengthM      float64 `json:"lengthM"`
	Slicer       string  `json:"slicer"`
	Material     string  `json:"material"`
	Tariff       float64 `json:"tariff"`
	PowerKW      float64 `json:"powerKW"`
	AmortPerHour float64 `json:"amortPerHour"`
	MaintPerHour float64 `json:"maintPerHour"`
	PurgeWasteG  float64 `json:"purgeWasteG"`
	OpRate       float64 `json:"opRate"`
	PrepH        float64 `json:"prepH"`
	PostH        float64 `json:"postH"`
	DesignerRate float64 `json:"designerRate"`
	ModelH       float64 `json:"modelH"`
	ModelPrice   float64 `json:"modelPrice"`
	ExtraMat     float64 `json:"extraMat"`
	FailRate     float64 `json:"failRate"`
	MarginProfit float64 `json:"marginProfit"`
	MarginDev    float64 `json:"marginDev"`
	Fees         float64 `json:"fees"`
	Copies       int     `json:"copies"`
	VATRate      float64 `json:"vatRate"`
	RoundStep    float64 `json:"roundStep"`
}

// UpdateManualJob правит работу истории с автопересчётом (manager+).
// Расчёт идёт строго по переданным полям (см. JobEdit), не по старому снапшоту.
func (a *App) UpdateManualJob(in JobEdit) error {
	if err := a.need(models.RoleManager); err != nil {
		return err
	}
	return a.db.Jobs().UpdateManual(models.Job{
		ID: in.ID, Filename: in.Filename, Status: in.Status,
		StartAt: in.StartAt, EndAt: in.EndAt,
		PrintSec: in.PrintSec, TotalSec: in.TotalSec, EstimatedSec: in.EstimatedSec,
		SizeBytes: in.SizeBytes, WeightG: in.WeightG, LengthM: in.LengthM, Slicer: in.Slicer,
	}, in.Material, store.JobCalcInput{
		Tariff: in.Tariff, PowerKW: in.PowerKW, AmortPerHour: in.AmortPerHour,
		MaintPerHour: in.MaintPerHour, PurgeWasteG: in.PurgeWasteG,
		OpRate: in.OpRate, PrepH: in.PrepH, PostH: in.PostH,
		DesignerRate: in.DesignerRate, ModelH: in.ModelH, ModelPrice: in.ModelPrice,
		ExtraMat: in.ExtraMat, FailRate: in.FailRate,
		MarginProfit: in.MarginProfit, MarginDev: in.MarginDev, Fees: in.Fees,
		Copies: in.Copies, VATRate: in.VATRate, RoundStep: in.RoundStep,
	})
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

// ListFxRates — курсы валют.
func (a *App) ListFxRates() ([]sqlite.FxRate, error) {
	return a.db.ListFxRates()
}

// SetFxRate задаёт курс валюты на сегодня (manager+).
func (a *App) SetFxRate(currency string, rate float64) error {
	if err := a.need(models.RoleManager); err != nil {
		return err
	}
	return a.db.SetFxRate(currency, rate)
}

// DeleteFxRate удаляет курс (manager+).
func (a *App) DeleteFxRate(currency, day string) error {
	if err := a.need(models.RoleManager); err != nil {
		return err
	}
	return a.db.DeleteFxRate(currency, day)
}

// ConvertCurrency пересчитывает все суммы в target и переключает базу (manager+).
func (a *App) ConvertCurrency(target string, rate float64) error {
	if err := a.need(models.RoleManager); err != nil {
		return err
	}
	return a.db.ConvertCurrency(target, rate)
}

// HistoryColumns — все колонки истории печати (порядок фиксирован).
// Вторая группа — расчётные поля из снапшота (все редактируемые в правке).
var HistoryColumns = []string{
	"filename", "start", "end", "status", "duration", "weight", "length",
	"size", "filament", "slicer", "cost", "price", "origin",
	"material", "tariff", "power", "amort", "maint", "purge",
	"oprate", "prep", "post", "designer", "modelh", "modelprice",
	"extra", "fail", "marginprofit", "margindev", "fees",
	"copies", "vat", "round",
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
