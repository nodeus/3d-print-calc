package export

import (
	"bytes"
	"embed"
	"fmt"
	"regexp"
	"strconv"

	"github.com/jung-kurt/gofpdf"
)

// Шрифты с кириллицей (в core-шрифтах gofpdf её нет).
//
//go:embed fonts/DejaVuSans.ttf fonts/DejaVuSans-Bold.ttf
var fontFS embed.FS

// FooterText — подпись в нижнем колонтитуле каждой страницы.
const FooterText = "nodeus.ru"

// Section — одна таблица отчёта: заголовок, шапка, строки (уже отформатированы).
type Section struct {
	Title   string
	Headers []string
	Rows    [][]string
}

// BuildPDF собирает A4-альбомный отчёт: шапка приложения, титул,
// секции-таблицы, футер nodeus.ru + номера страниц. Кириллица — DejaVu.
func BuildPDF(appHeader, docTitle string, sections []Section) ([]byte, error) {
	pdf := gofpdf.New("L", "mm", "A4", "")
	registerFonts(pdf)
	pdf.SetAuthor("3dprint-calc", true)
	pdf.SetTitle(docTitle, true)
	pdf.AliasNbPages("{nb}")
	// Футер каждой страницы: nodeus.ru слева, номер справа.
	pdf.SetFooterFunc(func() {
		pdf.SetY(-15)
		pdf.SetFont("DejaVu", "", 8)
		pdf.CellFormat(0, 10, FooterText, "", 0, "L", false, 0, "")
		pdf.CellFormat(0, 10, fmt.Sprintf("стр. %d/{nb}", pdf.PageNo()), "", 0, "R", false, 0, "")
	})
	pdf.AddPage()
	pdf.SetFont("DejaVu", "", 10)
	pdf.SetTextColor(100, 100, 100)
	pdf.Cell(0, 7, appHeader)
	pdf.Ln(8)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("DejaVu", "B", 16)
	pdf.Cell(0, 10, docTitle)
	pdf.Ln(12)

	for _, s := range sections {
		if s.Title != "" {
			// Разрыв, если секция не влезет целиком хотя бы шапкой.
			if pdf.GetY() > 150 {
				pdf.AddPage()
			}
			pdf.SetFont("DejaVu", "B", 12)
			pdf.Cell(0, 8, s.Title)
			pdf.Ln(9)
		}
		if len(s.Headers) > 0 {
			table(pdf, s.Headers, s.Rows)
			pdf.Ln(4)
		}
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("ERR_PDF: %w", err)
	}
	return buf.Bytes(), nil
}

func registerFonts(pdf *gofpdf.Fpdf) {
	for _, f := range []struct {
		name string
		file string
		st   string
	}{
		{"DejaVu", "fonts/DejaVuSans.ttf", ""},
		{"DejaVu", "fonts/DejaVuSans-Bold.ttf", "B"},
	} {
		raw, err := fontFS.ReadFile(f.file)
		if err != nil {
			continue
		}
		pdf.AddUTF8FontFromBytes(f.name, f.st, raw)
	}
}

func table(pdf *gofpdf.Fpdf, headers []string, rows [][]string) {
	n := len(headers)
	if n == 0 {
		return
	}
	pageW, _ := pdf.GetPageSize()
	l, _, r, _ := pdf.GetMargins()
	w := (pageW - l - r) / float64(n)

	// Шапка.
	pdf.SetFont("DejaVu", "B", 9)
	pdf.SetFillColor(230, 230, 230)
	for _, h := range headers {
		pdf.CellFormat(w, 7, h, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	// Строки.
	pdf.SetFont("DejaVu", "", 9)
	pdf.SetFillColor(255, 255, 255)
	for i, row := range rows {
		if pdf.GetY() > 170 {
			pdf.AddPage()
			// Повтор шапки на новой странице.
			pdf.SetFont("DejaVu", "B", 9)
			pdf.SetFillColor(230, 230, 230)
			for _, h := range headers {
				pdf.CellFormat(w, 7, h, "1", 0, "C", true, 0, "")
			}
			pdf.Ln(-1)
			pdf.SetFont("DejaVu", "", 9)
			pdf.SetFillColor(255, 255, 255)
		}
		if i%2 == 1 {
			pdf.SetFillColor(245, 245, 245)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		for c := 0; c < n; c++ {
			cell := ""
			if c < len(row) {
				cell = row[c]
			}
			if r, g, b, ok := parseHexColor(cell); ok {
				// Цветная ячейка: залитый квадрат + hex рядом.
				x, y := pdf.GetX(), pdf.GetY()
				pdf.SetFillColor(r, g, b)
				pdf.Rect(x+1, y+1, 4, 4, "F")
				pdf.SetX(x + 6)
				pdf.CellFormat(w-6, 6, cell, "1", 0, "L", true, 0, "")
				continue
			}
			pdf.CellFormat(w, 6, cell, "1", 0, "L", true, 0, "")
		}
		pdf.Ln(-1)
	}
}

var hexColorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// parseHexColor разбирает #rrggbb в компоненты 0..255.
func parseHexColor(s string) (int, int, int, bool) {
	if !hexColorRe.MatchString(s) {
		return 0, 0, 0, false
	}
	r, _ := strconv.ParseUint(s[1:3], 16, 8)
	g, _ := strconv.ParseUint(s[3:5], 16, 8)
	b, _ := strconv.ParseUint(s[5:7], 16, 8)
	return int(r), int(g), int(b), true
}
