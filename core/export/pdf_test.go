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
