package calc

import "testing"

func TestRoundStep(t *testing.T) {
	cases := []struct {
		value, step, want float64
	}{
		{279.153, 10, 280},
		{274.9, 10, 270},
		{275, 10, 280}, // половины — вверх
		{123.4, 5, 125},
		{121.1, 5, 120},
		{99.9, 1, 100},
		{785.318, 50, 800},
		{100, 0, 100},  // без округления
		{100, -5, 100}, // без округления
	}
	for _, c := range cases {
		if got := RoundStep(c.value, c.step); got != c.want {
			t.Errorf("RoundStep(%v, %v) = %v, want %v", c.value, c.step, got, c.want)
		}
	}
}

func TestToBase(t *testing.T) {
	if got := ToBase(100, 90); got != 9000 {
		t.Errorf("ToBase = %v, want 9000", got)
	}
	if got := ToBase(100, 0); got != 100 {
		t.Errorf("нулевой курс обязан быть нейтральным: %v", got)
	}
}
