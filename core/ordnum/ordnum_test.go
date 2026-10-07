package ordnum

import (
	"testing"
	"time"
)

func TestRender(t *testing.T) {
	at := time.Date(2026, 3, 5, 9, 7, 4, 0, time.UTC)
	cases := []struct {
		tpl  string
		seq  int
		want string
	}{
		{`{YYYY}-{nnnn}`, 42, `2026-0042`},
		{`{YY}-{n}`, 7, `26-7`},
		{`{YYYY}{MM}{DD}-{nn}`, 3, `20260305-03`},
		{`{"Заказ №"}{n}`, 42, `Заказ №42`},
		{`{"{"}{n}`, 1, `{1`},
		{`{Y}-{M}-{D} {H}:{m}:{s} {n}`, 5, `2026-3-5 9:7:4 5`},
		{`Y-M-D вне скобок — литерал`, 5, `Y-M-D вне скобок — литерал`},
		{`order-{nnnn}`, 12345, `order-12345`}, // счётчик длиннее ширины — как есть
		{`{HH}:{mm}:{ss}`, 1, `09:07:04`},
	}
	for _, c := range cases {
		got, err := Render(c.tpl, at, c.seq)
		if err != nil {
			t.Errorf("Render(%q): %v", c.tpl, err)
			continue
		}
		if got != c.want {
			t.Errorf("Render(%q) = %q, хочу %q", c.tpl, got, c.want)
		}
	}
}

func TestRenderErrors(t *testing.T) {
	at := time.Now()
	for _, tpl := range []string{
		`{"незакрыт`,  // нет }
		`{YYYY}-{"x}`, // битый литерал
		`{yyyy}`,      // нижний регистр — не токен
		`{}`,          // пусто
		`{Yn}`,        // смешанный
		`{foo}`,       // неизвестный
	} {
		if _, err := Render(tpl, at, 1); err == nil {
			t.Errorf("Render(%q) обязан вернуть ошибку", tpl)
		}
	}
	if _, err := Render(`{n}`, at, -1); err == nil {
		t.Error("отрицательный счётчик принят")
	}
}

func TestPreview(t *testing.T) {
	at := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	got, err := Preview(`{YYYY}-{nnnn}`, at)
	if err != nil || got != `2026-0042` {
		t.Fatalf("Preview = %q, %v", got, err)
	}
}
