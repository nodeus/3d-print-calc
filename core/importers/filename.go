package importers

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Соглашения OrcaSlicer в именах файлов:
//   - код через дефис:  _PETG-PPL01_, _PETG-STAR-G1_, _PLA-Creality_
//   - код через подчёркивание: _PETG_NIT-W4_ (= материал PETG-NIT-W4),
//     _PETG-B5_NIT_ — здесь NIT уже профиль принтера, а не материал
//   - generic без кода: _PETG_1h10m.gcode
//   - оценка времени в хвосте: _2h4m.gcode, _22m33s.gcode, _3h27m38s.gcode
var (
	materialRe = regexp.MustCompile(`_(P(?:ETG|LA)(?:[_-][A-Za-z0-9-]+)?)(_|-)`)
	durationRe = regexp.MustCompile(`^(?:(\d+)h)?(?:(\d+)m)?(?:(\d+)s)?$`)
	// Суффикс номера катушки: только цифры (BLACK-1 → 1; W4/G1 — не номер).
	spoolSuffixRe = regexp.MustCompile(`-(\d+)$`)
)

// profileTokens — сегменты профилей слайсера, не являющиеся материалом.
var profileTokens = map[string]bool{
	"standard": true, "optimal": true, "fine": true, "draft": true,
	"my": true, "nit": true,
}

// ParseMaterial извлекает код материала из имени файла, нормализуя
// подчёркивание в дефис под справочник (`PETG_NIT-W4` → `PETG-NIT-W4`).
// Дефисный токен продлевается следующим сегментом, если тот кодообразный
// (`PETG-NIT` + `W2` → `PETG-NIT-W2`; профиль `NIT` после `PETG-B5` — нет).
// Возвращает "" для generic PETG/PLA, длительностей и профилей.
// Точное совпадение со справочником проверяет вызывающий код (по БД).
func ParseMaterial(filename string) string {
	m := materialRe.FindStringSubmatch(filename)
	if m == nil {
		return ""
	}
	token := m[1]
	if i := strings.IndexByte(token, '_'); i >= 0 {
		suffix := token[i+1:]
		if !isMaterialSuffix(suffix) {
			return ""
		}
		return token[:i] + "-" + suffix
	}
	if token == "PETG" || token == "PLA" {
		// Голый префикс (суффикс не отделён '_'/'-', напр. _PETG_NIT-W4.gcode):
		// материал может идти следующим сегментом. m[0] включает разделитель.
		idx := strings.Index(filename, m[0])
		if seg := nextSegment(filename, idx+len(m[0])-1); isMaterialSuffix(seg) {
			return token + "-" + seg
		}
		return ""
	}
	// Дефисная форма: возможно, код разорван подчёркиванием (PETG-NIT_W2).
	idx := strings.Index(filename, m[0])
	if seg := nextSegment(filename, idx+len(m[0])-1); isMaterialSuffix(seg) {
		return token + "-" + seg
	}
	return token
}

// nextSegment возвращает сегмент после позиции pos (до '_' или '.'), где
// pos — индекс разделителя.
func nextSegment(s string, pos int) string {
	if pos < 0 || pos+1 >= len(s) {
		return ""
	}
	rest := s[pos+1:]
	end := strings.IndexAny(rest, "_.")
	if end >= 0 {
		rest = rest[:end]
	}
	return rest
}

// isMaterialSuffix проверяет, похож ли сегмент на код материала:
// буквы + цифры (коды типа NIT-W4, W2, PPL01 — всегда с цифрой),
// не длительность, не токен профиля, без точек.
func isMaterialSuffix(seg string) bool {
	if seg == "" || strings.Contains(seg, ".") {
		return false
	}
	hasLetter, hasDigit := false, false
	for _, r := range seg {
		if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return false
		}
		switch {
		case (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z'):
			hasLetter = true
		case r >= '0' && r <= '9':
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return false
	}
	if durationRe.MatchString(seg) {
		return false
	}
	return !profileTokens[strings.ToLower(seg)]
}

// ParseMaterialSpool извлекает код материала и номер катушки из имени файла:
// "PETG-NIT-BLACK-1" → ("PETG-NIT-BLACK", 1). Без числового суффикса —
// номер 0. Точное совпадение со справочником проверяет вызывающий код.
func ParseMaterialSpool(filename string) (string, int) {
	code := ParseMaterial(filename)
	if m := spoolSuffixRe.FindStringSubmatch(code); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			return strings.TrimSuffix(code, m[0]), n
		}
	}
	return code, 0
}

// ParseEstDuration извлекает оценку длительности из хвоста имени файла
// (часть после последнего '_' без '.gcode'): 2h4m, 22m33s, 3h27m38s.
// Возвращает ok=false, если хвост — не длительность.
func ParseEstDuration(filename string) (d time.Duration, ok bool) {
	base := strings.TrimSuffix(filename, ".gcode")
	i := strings.LastIndex(base, "_")
	if i < 0 {
		return 0, false
	}
	seg := base[i+1:]
	m := durationRe.FindStringSubmatch(seg)
	if m == nil || (m[1] == "" && m[2] == "" && m[3] == "") {
		return 0, false
	}
	// Хвост из одних минут без часов/секунд вида "150" — не длительность.
	if m[1] == "" && m[2] == "" && m[3] == "" {
		return 0, false
	}
	h, _ := strconv.Atoi("0" + m[1])
	min, _ := strconv.Atoi("0" + m[2])
	sec, _ := strconv.Atoi("0" + m[3])
	if h == 0 && min == 0 && sec == 0 {
		return 0, false
	}
	return time.Duration(h)*time.Hour +
		time.Duration(min)*time.Minute +
		time.Duration(sec)*time.Second, true
}
