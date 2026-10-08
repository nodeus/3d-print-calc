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

func TestCalculate_FullCycle(t *testing.T) {
	// Сквозной кейс: стол 4 копии, purge 10г, ТО 2₽/ч, fail 5%,
	// маржи 0.3+0.1, округление до 10, НДС 20%.
	in := Input{
		TotalSec:     7200, // 2ч
		WeightG:      100,
		PricePerKg:   1000, // 1₽/г
		Tariff:       7,
		PowerKW:      0.35,
		AmortPerHour: 5.5,
		OpRate:       100,
		PrepH:        0.1,
		PostH:        0.5,
		MaintPerHour: 2,
		PurgeWasteG:  10,
		FailRate:     0.05,
		MarginProfit: 0.3,
		MarginDev:    0.1,
		Copies:       4,
		RoundStep:    10,
		VATRate:      0.2,
	}
	got := Calculate(in)
	// mat=100, purge=10, elec=4.9, amort=11, maint=4, labor=60 → subtotal=189.9
	approx := func(v, want float64) {
		t.Helper()
		if d := v - want; d > 1e-9 || d < -1e-9 {
			t.Fatalf("got %v, want %v", v, want)
		}
	}
	approx(got.MatCost, 100)
	approx(got.PurgeCost, 10)
	approx(got.ElecCost, 4.9)
	approx(got.Amort, 11)
	approx(got.Maint, 4)
	approx(got.Labor, 60)
	approx(got.Subtotal, 189.9)
	approx(got.FailAdj, 9.495)
	approx(got.Cost, 199.395)
	// price=199.395*1.4=279.153 → round(10)=280 → VAT=336 → round(10)=340.
	approx(got.Price, 279.153)
	approx(got.PriceRounded, 280)
	approx(got.PriceVAT, 340)
	approx(got.CostPerCopy, 199.395/4)
	approx(got.PricePerCopy, 85)
}

func TestCalculate_RoundAfterVAT(t *testing.T) {
	// НДС возвращает копейки (140 × 1.22 = 170.8): при включённом шаге
	// округляем и финал.
	in := Input{
		TotalSec: 3600, WeightG: 100, PricePerKg: 1000,
		RoundStep: 10, VATRate: 0.22,
	}
	got := Calculate(in)
	// mat=100, fail=0 → cost=100 → price=100 → round=100 → VAT 122 → round=120.
	if got.PriceVAT != 120 {
		t.Fatalf("PriceVAT = %v, хочу 120", got.PriceVAT)
	}
	// Без шага копейки законны.
	in.RoundStep = 0
	got = Calculate(in)
	if diff := got.PriceVAT - 122; diff > 1e-9 || diff < -1e-9 {
		t.Fatalf("PriceVAT без шага = %v, хочу 122", got.PriceVAT)
	}
}

func TestCalculate_LegacyDefaults(t *testing.T) {
	// Нулевые новые поля = старое поведение: финал совпадает с Price,
	// per-copy равны итогам.
	in := Input{TotalSec: 3600, WeightG: 50, PricePerKg: 800}
	got := Calculate(in)
	if got.PriceRounded != got.Price || got.PriceVAT != got.Price {
		t.Fatalf("без опций округление/НДС обязаны быть нейтральны: %+v", got)
	}
	if got.CostPerCopy != got.Cost || got.PricePerCopy != got.PriceVAT {
		t.Fatalf("Copies<=1: per-copy обязаны равняться итогам: %+v", got)
	}
}

func TestWeightFromLength_MatchesExcel(t *testing.T) {
	// Сверка с Excel: 1.23*PI()*(1.75/2)^2*26141.233/1000 = 77.338751740825941.
	got := WeightFromLength(1.23, 1.75, 26141.233)
	if d := got - 77.338751740825941; d > 1e-9 || d < -1e-9 {
		t.Fatalf("weight = %v, want 77.338751740825941", got)
	}
	// С плотностью из справочника (1.27) вес больше — именно это чинит баг 1.23.
	if heavier := WeightFromLength(1.27, 1.75, 26141.233); heavier <= got {
		t.Fatalf("плотность 1.27 обязана давать больший вес: %v <= %v", heavier, got)
	}
}
