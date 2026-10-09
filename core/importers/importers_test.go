package importers

import (
	"os"
	"testing"
	"time"
)

func TestParseHistory_RealCSV(t *testing.T) {
	// Фикстура — боевой файл принтера. Тест гоняется из каталога пакета.
	f, err := os.Open("../../csv/print_history.csv")
	if err != nil {
		t.Skipf("нет фикстуры csv/print_history.csv: %v", err)
	}
	defer f.Close()
	jobs, err := ParseHistory(f)
	if err != nil {
		t.Fatalf("ParseHistory: %v", err)
	}
	if len(jobs) != 490 {
		t.Fatalf("строк = %d, хочу 490", len(jobs))
	}
	first := jobs[0]
	if first.Filename != "10-inch-rack-mount-for-tenda-25-gbit-switch_PETG-PPL01_4h17m.gcode" {
		t.Errorf("filename = %q", first.Filename)
	}
	if first.Status != "completed" || first.Type != "job" {
		t.Errorf("type/status = %q/%q", first.Type, first.Status)
	}
	if first.Size != 16565140 {
		t.Errorf("size = %d", first.Size)
	}
	if d := first.FilamentUsed - 42222.138; d > 1e-9 || d < -1e-9 {
		t.Errorf("filament_used = %v (десятичная запятая?)", first.FilamentUsed)
	}
	wantStart := time.Date(2026, 10, 4, 11, 8, 0, 0, time.UTC)
	if !first.StartTime.Equal(wantStart) {
		t.Errorf("start = %v, хочу %v", first.StartTime, wantStart)
	}
	if first.PrintDuration != 15301 || first.TotalDuration != 15728 {
		t.Errorf("durations = %d/%d", first.PrintDuration, first.TotalDuration)
	}
	if first.Slicer != "OrcaSlicer 2.4.2" {
		t.Errorf("slicer = %q", first.Slicer)
	}
	// В файле есть cancelled — парсер обязан их пропускать молча (не ошибка).
	cancelled := 0
	for _, j := range jobs {
		if j.Status == "cancelled" {
			cancelled++
		}
	}
	if cancelled == 0 {
		t.Errorf("ожидались cancelled-строки в фикстуре")
	}
}

func TestParseMaterial(t *testing.T) {
	cases := []struct{ in, want string }{
		{"my_rack_PETG-PPL01_2h1m.gcode", "PETG-PPL01"},
		{"my_rack_PETG-B5_4m38s.gcode", "PETG-B5"},
		{"my_rack_PETG-B5_NIT_0.20mm Standard my_2h1m.gcode", "PETG-B5"}, // NIT — профиль, не материал
		{"seed-planter_1x3_PETG_NIT-W4_0.20mm Standard @Creality K1C my_2h4m.gcode", "PETG-NIT-W4"},
		{"sleeve_12mm_PETG-NIT_W2_16m31s.gcode", "PETG-NIT-W2"},  // код разорван '_' — склеиваем
		{"body_PLA_Creality_0.20mm Standard 150_1h2m.gcode", ""}, // Creality — пресет вендора, не код
		{"Bottom - 0.4 PETG-PC_PETG_NIT-B3_0.20mm Standard 150_31m51s.gcode", "PETG-NIT-B3"},
		{"hook_PETG_1h10m.gcode", ""},  // generic без кода
		{"top_PETG_1h46m.gcode", ""},   // generic без кода
		{"calibration_cube.gcode", ""}, // нет материала
		{"sample_PETG-NIT-W4_30m.gcode", "PETG-NIT-W4"},
	}
	for _, c := range cases {
		if got := ParseMaterial(c.in); got != c.want {
			t.Errorf("ParseMaterial(%q) = %q, хочу %q", c.in, got, c.want)
		}
	}
}

func TestParseMaterialSpool(t *testing.T) {
	cases := []struct {
		in   string
		code string
		n    int
	}{
		{"part_PETG-NIT-BLACK-1_2h10m.gcode", "PETG-NIT-BLACK", 1},
		{"part_PETG-NIT-BLACK-12_2h10m.gcode", "PETG-NIT-BLACK", 12},
		// Номер только через дефис: _1 неотличим от температуры (150),
		// поэтому не разбирается.
		{"part_PETG-NIT-BLACK_1_2h10m.gcode", "PETG-NIT-BLACK", 0},
		{"sample_PETG-NIT-W4_30m.gcode", "PETG-NIT-W4", 0}, // W4 — код, не номер
		{"my_rack_PETG-PPL01_2h1m.gcode", "PETG-PPL01", 0},
		{"hook_PETG_1h10m.gcode", "", 0},
		{"calibration_cube.gcode", "", 0},
	}
	for _, c := range cases {
		if code, n := ParseMaterialSpool(c.in); code != c.code || n != c.n {
			t.Errorf("ParseMaterialSpool(%q) = (%q, %d), хочу (%q, %d)", c.in, code, n, c.code, c.n)
		}
	}
}

func TestParseEstDuration(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
		ok   bool
	}{
		{"my_rack_PETG-PPL01_2h1m.gcode", 2*time.Hour + 1*time.Minute, true},
		{"fast_tower_test_PETG-PPL01_15m13s.gcode", 15*time.Minute + 13*time.Second, true},
		{"top-and-bottom_PETG_3h27m38s.gcode", 3*time.Hour + 27*time.Minute + 38*time.Second, true},
		{"my_rack_PETG-B5_4m38s.gcode", 4*time.Minute + 38*time.Second, true},
		{"skadis-board-160x160_PETG_NIT-W1_0.16mm Optimal_2h33m.gcode", 2*time.Hour + 33*time.Minute, true},
		{"calibration_cube.gcode", 0, false},
		{"model_my.gcode", 0, false}, // хвост "my" — не длительность
	}
	for _, c := range cases {
		got, ok := ParseEstDuration(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("ParseEstDuration(%q) = (%v, %v), хочу (%v, %v)",
				c.in, got, ok, c.want, c.ok)
		}
	}
}
