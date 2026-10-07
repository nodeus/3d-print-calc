// Command seed — первичный сид app.db из legacy-данных.
//
// Материалы: таблица из docs/excel-reverse-engineering.md (лист «Материалы»,
// цены ₽/кг, даты — serial Excel). История: csv/print_history.csv.
// Идемпотентно (upsert по code и filename+start_at) — повторный запуск безопасен.
//
// Запуск из корня репо:
//
//	go run ./tools/seed -db app.db
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"3dprint-calc/core/calc"
	"3dprint-calc/core/importers"
	"3dprint-calc/core/models"
	sqlite "3dprint-calc/store-sqlite"
)

// seedMaterial — строка листа «Материалы»: код, цена ₽/кг, дата serial Excel, плотность.
type seedMaterial struct {
	code    string
	price   int64 // ₽/кг, минорные единицы/кг
	serial  int   // дата покупки serial Excel, 0 = нет
	density float64
}

var seedMaterials = []seedMaterial{
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
	defaultTariff  = 7.08
	defaultPowerKW = 0.35
	defaultOpRate  = 100.0
)

// upsertMaterial вставляет материал и (при price>0) одну катушку.
// Возвращает id материала и id катушки (0 — нет катушки).
func upsertMaterial(mats interface {
	UpsertByCode(models.Material) (int64, error)
	AddSpool(models.Spool) (int64, error)
}, code string, price int64, density float64, boughtAt string,
) (int64, int64, error) {
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

// excelDate переводит serial Excel в ISO-дату (дни от 1899-12-30).
func excelDate(serial int) string {
	if serial <= 0 {
		return ""
	}
	return time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC).AddDate(0, 0, serial).Format("2006-01-02")
}

func main() {
	dbPath := flag.String("db", "app.db", "путь к SQLite-файлу")
	csvPath := flag.String("csv", "csv/print_history.csv", "история печати принтера")
	migDir := flag.String("migrations", "migrations", "каталог миграций")
	flag.Parse()
	if err := run(*dbPath, *csvPath, *migDir); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
}

func run(dbPath, csvPath, migDir string) error {
	db, err := sqlite.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.Migrate(migDir); err != nil {
		return err
	}

	// 1. Принтер K1C (блок A6:B20 калькулятора).
	printerID, err := db.Printers().Save(models.Printer{
		Name: "K1C", Model: "Creality K1C", PurchasePrice: 4745400, Currency: "RUB",
		CommissioningDate: excelDate(45611), LifeYears: 3, HoursPerDay: 8,
		CalendarMode: "calendar", AmortMethod: "calendar", PowerKW: defaultPowerKW,
	})
	if err != nil {
		return err
	}

	// 2. Материалы + по одной катушке с ценой листа.
	mats := db.Materials()
	matIDs := map[string]models.Material{}
	spoolIDs := map[string]int64{}
	for _, sm := range seedMaterials {
		id, spoolID, err := upsertMaterial(mats, sm.code, sm.price, sm.density, excelDate(sm.serial))
		if err != nil {
			return err
		}
		matIDs[sm.code], _ = mats.GetByCode(sm.code)
		_ = id
		if spoolID != 0 {
			spoolIDs[sm.code] = spoolID
		}
	}

	// 3. История печати → jobs со снапшотом себестоимости.
	f, err := os.Open(csvPath)
	if err != nil {
		return err
	}
	defer f.Close()
	rows, err := importers.ParseHistory(f)
	if err != nil {
		return err
	}
	amortHour := calc.AmortCalendarHour(47454, 1096, 8) // СПИ 3 года календарных
	jobs := db.Jobs()
	linked, stubs := 0, 0
	priceByCode := map[string]float64{}
	for _, sm := range seedMaterials {
		priceByCode[sm.code] = float64(sm.price) / 100
	}
	for _, r := range rows {
		code := importers.ParseMaterial(r.Filename)
		m, ok := matIDs[code]
		if !ok && code != "" {
			// Код из имени файла, которого нет в листе «Материалы»
			// (более поздние закупки): стаб со стандартной плотностью.
			if _, _, err := upsertMaterial(mats, code, 0, 1.27, ""); err != nil {
				return err
			}
			m, _ = mats.GetByCode(code)
			matIDs[code] = m
			stubs++
			ok = true
		}
		pricePerKg, density := 0.0, 1.27
		var spoolID int64
		if ok {
			pricePerKg, density = priceByCode[code], m.Density
			spoolID = spoolIDs[code] // 0 — катушки нет (стаб): связь через cost_snapshot
			linked++
		}
		weightG := calc.WeightFromLength(density, 1.75, r.FilamentUsed) // filament_used уже в мм
		res := calc.Calculate(calc.Input{
			TotalSec: float64(r.TotalDuration), WeightG: weightG,
			PricePerKg: pricePerKg, Tariff: defaultTariff, PowerKW: defaultPowerKW,
			AmortPerHour: amortHour, OpRate: defaultOpRate,
		})
		snap, _ := json.Marshal(map[string]any{
			"material": code, "weight_g": weightG, "cost": res.Cost, "currency": "RUB",
		})
		if _, err := jobs.Upsert(models.Job{
			Filename: r.Filename, StartAt: r.StartTime.Format("2006-01-02 15:04"),
			PrinterID: printerID, SpoolID: spoolID, Status: r.Status,
			PrintSec: r.PrintDuration, TotalSec: r.TotalDuration,
			WeightG: weightG, LengthM: r.FilamentUsed / 1000, CostSnapshot: string(snap),
		}); err != nil {
			return err
		}
	}
	fmt.Printf("seed ok: db=%s materials=%d jobs=%d linked=%d stubs=%d\n",
		dbPath, len(seedMaterials)+stubs, len(rows), linked, stubs)
	return nil
}
