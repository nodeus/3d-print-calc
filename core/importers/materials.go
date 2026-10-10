package importers

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
)

// Формат выгрузки справочника (Настройки → «Выгрузить материалы»):
//
//	code;type;vendor;price_per_kg;currency;density;diameter
//
// Разделитель `;`, десятичная запятая. Пустые числа и "null" → 0,
// код обязателен.
type MaterialRow struct {
	Code       string
	Type       string
	Vendor     string
	PricePerKg float64
	Currency   string
	Density    float64
	Diameter   float64
}

// ParseMaterials читает CSV справочника материалов. Заголовок обязателен.
func ParseMaterials(r io.Reader) ([]MaterialRow, error) {
	cr := csv.NewReader(r)
	cr.Comma = ';'
	cr.FieldsPerRecord = -1
	rows, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("ERR_CSV_READ: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("ERR_CSV_EMPTY: пустой файл")
	}
	want := []string{"code", "type", "vendor", "price_per_kg", "currency", "density", "diameter"}
	for i, h := range want {
		if i >= len(rows[0]) || strings.TrimSpace(rows[0][i]) != h {
			return nil, fmt.Errorf("ERR_CSV_HEADER: колонка %d: хочу %q", i, h)
		}
	}
	out := make([]MaterialRow, 0, len(rows)-1)
	for n, row := range rows[1:] {
		if len(row) < len(want) {
			return nil, fmt.Errorf("ERR_CSV_ROW: строка %d: мало колонок (%d)", n+2, len(row))
		}
		m, err := parseMaterialRow(row)
		if err != nil {
			return nil, fmt.Errorf("строка %d: %w", n+2, err)
		}
		out = append(out, m)
	}
	return out, nil
}

func parseMaterialRow(row []string) (MaterialRow, error) {
	var m MaterialRow
	var err error
	m.Code = strings.TrimSpace(row[0])
	if m.Code == "" {
		return m, fmt.Errorf("ERR_CSV_CODE: пустой код")
	}
	m.Type = strings.TrimSpace(row[1])
	m.Vendor = strings.TrimSpace(row[2])
	if m.PricePerKg, err = parseFloat(row[3]); err != nil {
		return m, fmt.Errorf("ERR_CSV_FLOAT: price_per_kg: %w", err)
	}
	m.Currency = strings.TrimSpace(row[4])
	if m.Currency == "" {
		m.Currency = "RUB"
	}
	if m.Density, err = parseFloat(row[5]); err != nil {
		return m, fmt.Errorf("ERR_CSV_FLOAT: density: %w", err)
	}
	if m.Diameter, err = parseFloat(row[6]); err != nil {
		return m, fmt.Errorf("ERR_CSV_FLOAT: diameter: %w", err)
	}
	return m, nil
}
