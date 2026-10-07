package calc

import "testing"

func TestCalculate_MaterialAndElec(t *testing.T) {
	// Кейс knife-sharpener: ориентиры из Excel (вес 134.94г, цена 869₽/кг, тариф 7.08).
	got := Calculate(Input{
		TotalSec:   0.16214120370370369 * 86400,
		WeightG:    134.94029209963961,
		PricePerKg: 869,
		Tariff:     7.08,
		PowerKW:    0.35,
	})
	if diff := got.MatCost - 117.26311383458682; diff > 1e-6 || diff < -1e-6 {
		t.Fatalf("mat = %v, want ~117.263", got.MatCost)
	}
	if diff := got.ElecCost - 9.642861666666665; diff > 1e-6 || diff < -1e-6 {
		t.Fatalf("elec = %v, want ~9.6429", got.ElecCost)
	}
}

func TestCalculate_NoPlus24Regression(t *testing.T) {
	// Регресс бага Excel E16 (=B19*G5+24): амортизация обязана быть B19*G5*24,
	// без фиксированной добавки 24₽. При hours≈3.89 и ставке 7.2782 ждём ≈28.32, а не ≈52.32.
	in := Input{TotalSec: 0.16214120370370369 * 86400, AmortPerHour: 7.278220858895706}
	got := Calculate(in)
	want := 7.278220858895706 * (in.TotalSec / 3600)
	if diff := got.Amort - want; diff > 1e-9 || diff < -1e-9 {
		t.Fatalf("amort = %v, want %v (баг +24?)", got.Amort, want)
	}
	if got.Amort > 24 && in.TotalSec/3600 < 4 {
		t.Fatalf("amort suspiciously large: %v", got.Amort)
	}
}

func TestAmortCalendarHour(t *testing.T) {
	// СПИ 3 года ≈ 1096 календарных дней × 8ч = 8768ч; 47454/8768 ≈ 5.41₽/ч.
	got := AmortCalendarHour(47454, 1096, 8)
	if got < 5.3 || got > 5.6 {
		t.Fatalf("calendar amort = %v, want ~5.41", got)
	}
	if z := AmortCalendarHour(1, 0, 8); z != 0 {
		t.Fatalf("zero days must give 0, got %v", z)
	}
}
