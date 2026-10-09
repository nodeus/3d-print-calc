// Package seed — сид БД legacy-данными (dev/тесты) и EnsureAdmin для чистого
// первого запуска десктопа.
//
// Материалы: таблица листа «Материалы». История: CSV принтера.
// Полный сид — только через CLI tools/seed. Идемпотентно: повтор безопасен,
// непустые таблицы не трогаются.
package seed

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"3dprint-calc/core/auth"
	"3dprint-calc/core/calc"
	"3dprint-calc/core/importers"
	"3dprint-calc/core/models"
	"3dprint-calc/core/store"
)

// material — строка листа «Материалы»: код, цена ₽/кг (минорные/кг),
// дата покупки serial Excel, плотность.
type material struct {
	code    string
	price   int64
	serial  int
	density float64
}

var materials = []material{
	{"PETG-NIT-W3", 96500, 45741, 1.24},
	{"PETG-StarPlast-G1", 91600, 45691, 1.27},
	{"PETG-StarPlast-B1", 96900, 45691, 1.27},
	{"PETG-NIT-W2", 79000, 45666, 1.27},
	{"PETG-NIT-W1", 82600, 45641, 1.27},
	{"PETG-NIT-B1", 79600, 45641, 1.27},
	{"PETG-123D-B1", 54000, 45625, 1.27},
	{"PLA-CR-W1", 0, 45611, 1.27},
	{"PETG-NIT-B2", 86500, 45760, 1.27},
	{"PLA-CR-W2", 100000, 0, 1.27},
	{"PETG-NIT-W4", 86900, 45990, 1.27},
	{"PETG-NIT-B3", 69400, 45990, 1.27},
	{"PETG-NIT-BLUE1", 79400, 45990, 1.27},
	{"PETG-NIT-B4", 86500, 45757, 1.27},
	{"PETG-NIT-B5", 0, 0, 1.27},
}

// Тарифы/ставки по умолчанию для снапшота (лист «Калькулятор печати»).
const (
	DefaultTariff = 7.08
	DefaultPower  = 0.35
	DefaultOpRate = 100.0
)

// Stats — итог сида.
type Stats struct {
	Materials int
	Jobs      int
	Linked    int
	Stubs     int
}

// DB — минимальный интерфейс БД, нужный сиду.
type DB interface {
	Materials() store.Materials
	Printers() store.Printers
	Jobs() store.Jobs
	Users() store.Users
}

// SeedAll сеет принтер, материалы и историю (csv — открытый CSV принтера).
// Непустые таблицы пропускаются.
func SeedAll(db DB, csv io.Reader) (Stats, error) {
	var st Stats
	if err := EnsureAdmin(db); err != nil {
		return st, err
	}
	printerID, err := seedPrinter(db)
	if err != nil {
		return st, err
	}
	n, err := seedMaterials(db)
	if err != nil {
		return st, err
	}
	st.Materials = n
	if csv == nil {
		return st, nil
	}
	existing, err := db.Jobs().List(1)
	if err != nil {
		return st, fmt.Errorf("ERR_SEED_JOBS: %w", err)
	}
	if len(existing) > 0 {
		st.Jobs = -1 // уже засеяно, пропуск
		return st, nil
	}
	rows, err := importers.ParseHistory(csv)
	if err != nil {
		return st, err
	}
	imp, err := ImportRows(db, printerID, rows, "import", nil)
	if err != nil {
		return st, err
	}
	st.Linked, st.Stubs, st.Jobs = imp.Linked, imp.Stubs, len(rows)
	st.Materials += st.Stubs
	return st, nil
}

// ProgressFunc сообщает ход импорта: done из total. Вызывается каждые
// 25 строк и в конце (done == total). nil — без отчётов.
type ProgressFunc func(done, total int)

// ImportRows импортирует разобранные строки истории (upsert по filename+start_at).
// Справочник подтягивается из БД: неизвестные коды → стабы. Возвращает статистику.
// origin — происхождение работ: import (CSV/сид) или moonraker.
func ImportRows(db DB, printerID int64, rows []importers.PrintJob, origin string, onProgress ProgressFunc) (Stats, error) {
	if origin == "" {
		origin = "import"
	}
	var st Stats
	mats := db.Materials()
	list, err := mats.List()
	if err != nil {
		return st, fmt.Errorf("ERR_SEED_MATERIALS: %w", err)
	}
	matIDs := make(map[string]models.Material, len(list))
	spoolIDs := map[string]int64{}
	priceByCode := map[string]float64{}
	for _, m := range list {
		matIDs[m.Code] = m
		if spools, err := mats.ListSpools(m.ID); err == nil && len(spools) > 0 {
			spoolIDs[m.Code] = spools[0].ID
			priceByCode[m.Code] = float64(spools[0].Price) / 100
		}
	}
	amortHour := calc.AmortCalendarHour(47454, 1096, 8) // СПИ 3 года календарных
	jobs := db.Jobs()
	for i, r := range rows {
		code := importers.ParseMaterial(r.Filename)
		m, ok := matIDs[code]
		spoolN := 0
		if !ok {
			// Точного кода нет в базе: работаем с базой + номером катушки
			// (новая парадигма: PETG-NIT-BLACK-1 → PETG-NIT-BLACK + №1).
			// Точный код в базе — приоритет (legacy без номеров).
			if base, n := importers.ParseMaterialSpool(r.Filename); n > 0 {
				code, spoolN = base, n
				m, ok = matIDs[code]
			}
		}
		if !ok && code != "" {
			// Код из имени файла, которого нет в листе «Материалы»
			// (более поздние закупки): стаб со стандартной плотностью.
			// Стаб всегда по базе, без номера катушки.
			if _, _, err := upsertMaterial(mats, code, 0, 1.27, ""); err != nil {
				return st, err
			}
			m, _ = mats.GetByCode(code)
			matIDs[code] = m
			st.Stubs++
			ok = true
		}
		pricePerKg, density := 0.0, 1.27
		var spoolID int64
		if ok {
			pricePerKg, density = priceByCode[code], m.Density
			spoolID = spoolIDs[code]
			// Номер катушки из имени файла: её цена точнее первой.
			if spoolN > 0 {
				if spools, err := mats.ListSpools(m.ID); err == nil {
					for _, s := range spools {
						if s.SpoolNo == spoolN {
							spoolID = s.ID
							pricePerKg = float64(s.Price) / 100
							break
						}
					}
				}
			}
			st.Linked++
		}
		weightG := calc.WeightFromLength(density, 1.75, r.FilamentUsed)
		res := calc.Calculate(calc.Input{
			TotalSec: float64(r.TotalDuration), WeightG: weightG,
			PricePerKg: pricePerKg, Tariff: DefaultTariff, PowerKW: DefaultPower,
			AmortPerHour: amortHour, OpRate: DefaultOpRate,
		})
		snap, _ := json.Marshal(map[string]any{
			"v": 2, "material": code, "weight_g": weightG, "cost": res.Cost, "currency": "RUB",
			"input": map[string]any{
				"tariff": DefaultTariff, "powerKW": DefaultPower,
				"amortPerHour": amortHour, "opRate": DefaultOpRate,
				"prepH": 0, "postH": 0, "designerRate": 0, "modelH": 0,
				"modelPrice": 0, "extraMat": 0, "failRate": 0,
				"marginProfit": 0, "marginDev": 0, "fees": 0,
				"maintPerHour": 0, "purgeWasteG": 0, "copies": 1,
				"vatRate": 0, "roundStep": 0,
			},
		})
		if _, err := jobs.Upsert(models.Job{
			Filename: r.Filename, StartAt: r.StartTime.Format("2006-01-02 15:04"),
			EndAt:     r.EndTime.Format("2006-01-02 15:04"),
			PrinterID: printerID, SpoolID: spoolID, Status: r.Status, Origin: origin,
			PrintSec: r.PrintDuration, TotalSec: r.TotalDuration,
			EstimatedSec: r.EstimatedTime, SizeBytes: r.Size, FilamentTotalMm: r.FilamentTotal,
			WeightG: weightG, LengthM: r.FilamentUsed / 1000, Slicer: r.Slicer,
			CostSnapshot: string(snap),
		}); err != nil {
			return st, err
		}
		if onProgress != nil && ((i+1)%25 == 0 || i+1 == len(rows)) {
			onProgress(i+1, len(rows))
		}
	}
	st.Jobs = len(rows)
	return st, nil
}

// EnsureAdmin создаёт admin/admin123 при пустой таблице пользователей.
// Пароль обязан быть сменён — см. страницу пользователей в приложении.
// Вызывается и десктопом при первом запуске, и полным сидом.
func EnsureAdmin(db DB) error {
	n, err := db.Users().Count()
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	phc, err := auth.Hash("admin123")
	if err != nil {
		return err
	}
	_, err = db.Users().Create(models.User{Login: "admin", PasswordHash: phc, Role: models.RoleAdmin, Locale: "ru"})
	return err
}

// seedPrinter создаёт K1C из блока амортизации Excel либо возвращает
// существующий первый принтер.
func seedPrinter(db DB) (int64, error) {
	list, err := db.Printers().List()
	if err != nil {
		return 0, fmt.Errorf("ERR_SEED_PRINTER: %w", err)
	}
	if len(list) > 0 {
		return list[0].ID, nil
	}
	return db.Printers().Save(models.Printer{
		Name: "K1C", Model: "Creality K1C", PurchasePrice: 4745400, Currency: "RUB",
		CommissioningDate: ExcelDate(45611), LifeYears: 3, HoursPerDay: 8,
		CalendarMode: "calendar", AmortMethod: "calendar", PowerKW: DefaultPower,
	})
}

// seedMaterials сеет справочник (пропуск, если не пуст). Возвращает число материалов.
func seedMaterials(db DB) (int, error) {
	mats := db.Materials()
	existing, err := mats.List()
	if err != nil {
		return 0, fmt.Errorf("ERR_SEED_MATERIALS: %w", err)
	}
	if len(existing) > 0 {
		return len(existing), nil
	}
	for _, sm := range materials {
		if _, _, err := upsertMaterial(mats, sm.code, sm.price, sm.density, ExcelDate(sm.serial)); err != nil {
			return 0, err
		}
	}
	return len(materials), nil
}

// upsertMaterial вставляет материал и (при price>0) одну катушку.
func upsertMaterial(mats store.Materials, code string, price int64, density float64, boughtAt string) (int64, int64, error) {
	typ := code
	if i := strings.IndexByte(typ, '-'); i >= 0 {
		typ = typ[:i]
	}
	id, err := mats.UpsertByCode(models.Material{
		Code: code, Type: typ, Density: density, Diameter: 1.75,
	})
	if err != nil {
		return 0, 0, err
	}
	if price <= 0 {
		return id, 0, nil
	}
	spoolID, err := mats.AddSpool(models.Spool{
		MaterialID: id, Price: price, Currency: "RUB", FxRate: 1,
		BoughtAt: boughtAt, WeightInitG: 1000, WeightLeftG: 1000,
	})
	return id, spoolID, err
}

// ExcelDate переводит serial Excel в ISO-дату (дни от 1899-12-30).
func ExcelDate(serial int) string {
	if serial <= 0 {
		return ""
	}
	return time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC).AddDate(0, 0, serial).Format("2006-01-02")
}
