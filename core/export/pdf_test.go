package export

import (
	"bytes"
	"testing"
)

func TestBuildPDF(t *testing.T) {
	data, err := BuildPDF("3D print Cost Calculator v0.4.2 (dev)", "Тест: выручка", []Section{
		{Title: "Месяцы", Headers: []string{"Месяц", "Итого", "Заказы"},
			Rows: [][]string{{"2026-10", "1000,00 ₽", "3"}}},
		{Title: "Пустая", Headers: []string{"A"}, Rows: nil},
	})
	if err != nil {
		t.Fatalf("BuildPDF: %v", err)
	}
	if len(data) == 0 || !bytes.HasPrefix(data, []byte("%PDF")) {
		t.Fatalf("не PDF: %d байт", len(data))
	}
	// Футер и кириллица: ищем в распакованном виде нельзя (flate),
	// проверяем размер как эвристику встроенных шрифтов.
	if len(data) < 20000 {
		t.Fatalf("подозрительно маленький PDF: %d", len(data))
	}
}

func TestParseHexColor(t *testing.T) {
	r, g, b, ok := parseHexColor("#ff8000")
	if !ok || r != 255 || g != 128 || b != 0 {
		t.Fatalf("parse: %v %v %v %v", r, g, b, ok)
	}
	for _, s := range []string{"", "red", "#fff", "#gggggg", "123456"} {
		if _, _, _, ok := parseHexColor(s); ok {
			t.Fatalf("%q принят за цвет", s)
		}
	}
}

func TestBuildPDFColorCell(t *testing.T) {
	data, err := BuildPDF("app", "Цвета", []Section{{
		Title:   "",
		Headers: []string{"Код", "Цвет"},
		Rows:    [][]string{{"PETG-R", "#ff0000"}, {"PETG-B", "не цвет"}},
	}})
	if err != nil {
		t.Fatalf("BuildPDF: %v", err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF")) {
		t.Fatal("не PDF")
	}
}
