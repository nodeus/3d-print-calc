package export

import (
	"bytes"
	"compress/zlib"
	"io"
	"regexp"
	"strings"
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

// TestBuildPDFColorCellKeepsRowFill: квадратик красится цветом материала,
// а заливка строки обязана вернуться к зебре — иначе весь остаток строки
// заливается цветом материала (баг v1.0.8). Проверяем порядок операторов
// в распакованном content-стриме: красная заливка → rect → белая заливка.
func TestBuildPDFColorCellKeepsRowFill(t *testing.T) {
	data, err := BuildPDF("app", "Цвета", []Section{{
		Headers: []string{"Код", "Цвет", "Тип"},
		Rows:    [][]string{{"PETG-R", "#ff0000", "PETG"}},
	}})
	if err != nil {
		t.Fatalf("BuildPDF: %v", err)
	}
	text := pdfContentText(t, data)
	// Операторы gofpdf: цветная заливка — «r g b rg», белая — «1.000 g»,
	// залитый квадрат — «x y w h re f» (h отрицательный), рамки ячеек — «re B».
	red := strings.Index(text, "1.000 0.000 0.000 rg")
	rect := regexp.MustCompile(`-?[\d.]+ -?[\d.]+ -?[\d.]+ -?[\d.]+ re f`).FindStringIndex(text)
	white := -1
	if rect != nil {
		if i := strings.Index(text[rect[1]:], "1.000 g"); i >= 0 {
			white = rect[1] + i
		}
	}
	if red < 0 || rect == nil || white < 0 || !(red < rect[0] && rect[0] < white) {
		t.Fatalf("порядок операторов бит: red=%d rect=%v white=%d", red, rect, white)
	}
}

// pdfContentText склеивает распакованные content-стримы PDF.
func pdfContentText(t *testing.T, data []byte) string {
	t.Helper()
	reStream := regexp.MustCompile(`(?s)stream\r?\n(.+?)\r?\nendstream`)
	var sb strings.Builder
	for _, m := range reStream.FindAllSubmatch(data, -1) {
		zr, err := zlib.NewReader(bytes.NewReader(m[1]))
		if err != nil {
			continue // метаданные/шрифты — не flate
		}
		raw, err := io.ReadAll(zr)
		zr.Close()
		if err != nil {
			continue
		}
		sb.Write(raw)
		sb.WriteByte('\n')
	}
	if sb.Len() == 0 {
		t.Fatal("content-стримы не распаковались")
	}
	return sb.String()
}
