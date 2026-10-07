package calc

import "math"

// RoundStep округляет value до кратного step (1/5/10/50).
// step <= 0 — без округления. Половины — вверх (math.Round).
func RoundStep(value, step float64) float64 {
	if step <= 0 {
		return value
	}
	return math.Round(value/step) * step
}

// ToBase пересчитывает сумму в базовой валюте через снапшот курса.
// fxRate — единиц базовой валюты за единицу валюты суммы (1 = та же валюта).
func ToBase(amount, fxRate float64) float64 {
	if fxRate <= 0 {
		return amount
	}
	return amount * fxRate
}

// WeightFromLength возвращает вес прутка, г, по плотности материала.
//
// Заменяет захардкоженную 1.23 из Excel: density берётся из справочника
// materials (обычно 1.27, вариативно 1.24). diameterMM — диаметр прутка
// (1.75), lengthMM — длина в миллиметрах (filament_used из истории).
func WeightFromLength(density, diameterMM, lengthMM float64) float64 {
	r := diameterMM / 2
	return density * math.Pi * r * r * lengthMM / 1000
}
