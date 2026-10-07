// Package calc — движок расчёта себестоимости/цены 3D-печати.
//
// Единственное место формул в проекте (см. AGENTS.md).
// Канон: docs/excel-reverse-engineering.md. Баги Excel (E16 +24₽,
// NETWORKDAYS, плотность 1.23) здесь НЕ воспроизводятся.
package calc

// AmortMethod — метод амортизации принтера.
type AmortMethod string

const (
	// AmortCalendar — календарные дни СПИ × часов_в_день. Метод по умолчанию.
	AmortCalendar AmortMethod = "calendar"
	// AmortWorkdays — только рабочие дни (legacy NETWORKDAYS). Опционально.
	AmortWorkdays AmortMethod = "workdays"
	// AmortResource — цена / ресурс_часов.
	AmortResource AmortMethod = "resource"
)

// Input — входные данные одного расчёта.
type Input struct {
	TotalSec     float64 // общее время печати, сек
	WeightG      float64 // вес изделия, г
	PricePerKg   float64 // цена материала за кг
	Tariff       float64 // тариф электроэнергии за кВт·ч
	PowerKW      float64 // мощность принтера, кВт
	AmortPerHour float64 // ставка амортизации, за час
	OpRate       float64 // ставка оператора/час
	PrepH        float64 // часы подготовки
	PostH        float64 // часы постобработки
	DesignerRate float64 // ставка дизайнера/час
	ModelH       float64 // часы моделирования
	ModelPrice   float64 // готовая 3D-модель, фикс
	ExtraMat     float64 // доп. материалы
	FailRate     float64 // доля неудач, 0..1
	MarginProfit float64 // наценка на прибыль, доля
	MarginDev    float64 // наценка на развитие, доля
	Fees         float64 // фиксированные допы
}

// Result — построчный результат расчёта.
type Result struct {
	MatCost  float64
	ElecCost float64
	Amort    float64
	Labor    float64
	Subtotal float64
	FailAdj  float64
	Cost     float64
	Price    float64
}

// Calculate считает себестоимость и цену. Формулы — см. AGENTS.md раздел 3.
func Calculate(in Input) Result {
	hours := in.TotalSec / 3600
	mat := in.PricePerKg / 1000 * in.WeightG
	elec := in.Tariff * in.PowerKW * hours
	amort := in.AmortPerHour * hours
	labor := in.OpRate*in.PrepH + in.OpRate*in.PostH + in.DesignerRate*in.ModelH + in.ModelPrice
	subtotal := mat + elec + amort + labor + in.ExtraMat
	failAdj := subtotal * in.FailRate
	cost := subtotal + failAdj
	price := cost*(1+in.MarginProfit+in.MarginDev) + in.Fees
	return Result{
		MatCost:  mat,
		ElecCost: elec,
		Amort:    amort,
		Labor:    labor,
		Subtotal: subtotal,
		FailAdj:  failAdj,
		Cost:     cost,
		Price:    price,
	}
}

// AmortCalendarHour возвращает часовую ставку календарным методом:
// purchasePrice / (календарных_дней(СПИ) × часов_в_день).
func AmortCalendarHour(purchasePrice float64, calendarDays int, hoursPerDay float64) float64 {
	if calendarDays <= 0 || hoursPerDay <= 0 {
		return 0
	}
	return purchasePrice / (float64(calendarDays) * hoursPerDay)
}
