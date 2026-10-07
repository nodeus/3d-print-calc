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
//
// Поля PurgeWasteG..RoundStep опциональны: нулевое значение сохраняет
// поведение legacy-формулы (AGENTS.md, раздел 3).
type Input struct {
	TotalSec     float64 // общее время печати, сек
	WeightG      float64 // вес изделия, г (все копии на столе суммарно)
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

	MaintPerHour float64 // ТО/износ оснастки, ₽/час (0 = не учитывать)
	PurgeWasteG  float64 // purge-башня/брак/поддержки сверх веса, г (0 = нет)
	Copies       int     // копий на столе; итоги — за стол, per-copy — делением (<=1 = одна)
	VATRate      float64 // НДС, доля (0 = без НДС); применяется после округления
	RoundStep    float64 // шаг округления цены: 1/5/10/50 (<=0 = без округления)
}

// Result — построчный результат расчёта.
type Result struct {
	MatCost   float64
	PurgeCost float64
	ElecCost  float64
	Amort     float64
	Maint     float64
	Labor     float64
	Subtotal  float64
	FailAdj   float64
	Cost      float64 // себестоимость стола
	Price     float64 // цена стола до округления/НДС

	PriceRounded float64 // цена стола после RoundStep
	PriceVAT     float64 // финал стола с НДС
	CostPerCopy  float64 // себестоимость одной копии
	PricePerCopy float64 // финальная цена одной копии
}

// Calculate считает себестоимость и цену. Формулы — см. AGENTS.md раздел 3.
//
// Порядок: subtotal(+maint+purge) → fail → cost → price(+margins+fees) →
// round(step) → VAT. Per-copy — делением финалов на Copies.
func Calculate(in Input) Result {
	hours := in.TotalSec / 3600
	mat := in.PricePerKg / 1000 * in.WeightG
	purge := in.PricePerKg / 1000 * in.PurgeWasteG
	elec := in.Tariff * in.PowerKW * hours
	amort := in.AmortPerHour * hours
	maint := in.MaintPerHour * hours
	labor := in.OpRate*in.PrepH + in.OpRate*in.PostH + in.DesignerRate*in.ModelH + in.ModelPrice
	subtotal := mat + purge + elec + amort + maint + labor + in.ExtraMat
	failAdj := subtotal * in.FailRate
	cost := subtotal + failAdj
	price := cost*(1+in.MarginProfit+in.MarginDev) + in.Fees
	rounded := RoundStep(price, in.RoundStep)
	withVAT := rounded * (1 + in.VATRate)

	copies := in.Copies
	if copies < 1 {
		copies = 1
	}
	return Result{
		MatCost:      mat,
		PurgeCost:    purge,
		ElecCost:     elec,
		Amort:        amort,
		Maint:        maint,
		Labor:        labor,
		Subtotal:     subtotal,
		FailAdj:      failAdj,
		Cost:         cost,
		Price:        price,
		PriceRounded: rounded,
		PriceVAT:     withVAT,
		CostPerCopy:  cost / float64(copies),
		PricePerCopy: withVAT / float64(copies),
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
