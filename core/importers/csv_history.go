// Package importers — импорт истории печати принтера.
//
// Формат Moonraker/Fluidd CSV: `;`-разделитель, десятичная запятая,
// даты `DD.MM.YYYY HH:MM`:
//
//	filename;type;status;size;start_time;end_time;estimated_time;
//	print_duration;total_duration;filament_total;filament_used;slicer
//
// filament_* — миллиметры прутка, длительности — секунды.
// Импорт идемпотентен по паре (filename, start_time) — см. store-sqlite.
package importers

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// PrintJob — одна строка истории печати.
type PrintJob struct {
	Filename      string
	Type          string
	Status        string
	Size          int64
	StartTime     time.Time
	EndTime       time.Time
	EstimatedTime int64
	PrintDuration int64
	TotalDuration int64
	FilamentTotal float64 // мм
	FilamentUsed  float64 // мм
	Slicer        string
}

const csvDateLayout = "02.01.2006 15:04"

// ParseHistory читает CSV истории печати. Заголовок обязателен и проверяется.
func ParseHistory(r io.Reader) ([]PrintJob, error) {
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
	want := []string{"filename", "type", "status", "size", "start_time", "end_time",
		"estimated_time", "print_duration", "total_duration", "filament_total", "filament_used", "slicer"}
	for i, h := range want {
		if i >= len(rows[0]) || strings.TrimSpace(rows[0][i]) != h {
			return nil, fmt.Errorf("ERR_CSV_HEADER: колонка %d: хочу %q", i, h)
		}
	}
	out := make([]PrintJob, 0, len(rows)-1)
	for n, row := range rows[1:] {
		if len(row) < len(want) {
			return nil, fmt.Errorf("ERR_CSV_ROW: строка %d: мало колонок (%d)", n+2, len(row))
		}
		j, err := parseRow(row)
		if err != nil {
			return nil, fmt.Errorf("строка %d: %w", n+2, err)
		}
		out = append(out, j)
	}
	return out, nil
}

func parseRow(row []string) (PrintJob, error) {
	var j PrintJob
	var err error
	j.Filename = row[0]
	j.Type = row[1]
	j.Status = row[2]
	// Принтер пишет пустые поля и буквальный "null" (см. строку 294 фикстуры):
	// числа обязаны быть толерантными, даты — строгие (ключ идемпотентности).
	if j.Size, err = parseInt(row[3]); err != nil {
		return j, fmt.Errorf("ERR_CSV_SIZE: %w", err)
	}
	if j.StartTime, err = time.Parse(csvDateLayout, row[4]); err != nil {
		return j, fmt.Errorf("ERR_CSV_DATE: start_time: %w", err)
	}
	if j.EndTime, err = time.Parse(csvDateLayout, row[5]); err != nil {
		return j, fmt.Errorf("ERR_CSV_DATE: end_time: %w", err)
	}
	if j.EstimatedTime, err = parseInt(row[6]); err != nil {
		return j, fmt.Errorf("ERR_CSV_INT: estimated_time: %w", err)
	}
	if j.PrintDuration, err = parseInt(row[7]); err != nil {
		return j, fmt.Errorf("ERR_CSV_INT: print_duration: %w", err)
	}
	if j.TotalDuration, err = parseInt(row[8]); err != nil {
		return j, fmt.Errorf("ERR_CSV_INT: total_duration: %w", err)
	}
	if j.FilamentTotal, err = parseFloat(row[9]); err != nil {
		return j, fmt.Errorf("ERR_CSV_FLOAT: filament_total: %w", err)
	}
	if j.FilamentUsed, err = parseFloat(row[10]); err != nil {
		return j, fmt.Errorf("ERR_CSV_FLOAT: filament_used: %w", err)
	}
	j.Slicer = row[11]
	return j, nil
}

// parseInt разбирает целое; пусто и "null" от принтера → 0.
func parseInt(s string) (int64, error) {
	if t := strings.TrimSpace(s); t == "" || strings.EqualFold(t, "null") {
		return 0, nil
	}
	return strconv.ParseInt(strings.TrimSpace(s), 10, 64)
}

// parseFloat разбирает число с десятичной запятой ("42206,25");
// пусто и "null" → 0.
func parseFloat(s string) (float64, error) {
	if t := strings.TrimSpace(s); t == "" || strings.EqualFold(t, "null") {
		return 0, nil
	}
	return parseCommaFloat(s)
}

// parseCommaFloat разбирает число с десятичной запятой ("42206,25").
func parseCommaFloat(s string) (float64, error) {
	return strconv.ParseFloat(strings.Replace(s, ",", ".", 1), 64)
}
