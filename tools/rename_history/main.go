// Tool rename_history переписывает имена файлов в истории печати из старой
// нотации материалов в новую (КОД-N под катушки).
//
// Вход: родная выгрузка принтера (csv/print_history.csv) + два маппинга:
// docs/materials-export.csv (старые коды, распарсенные из имён) и
// docs/materials-export_renamed.csv (новые коды построчно).
// Спорные (ошибочно названные в слайсере) фиксятся на первый номер:
//
//	PETG-NIT-BLACK-4-1 → PETG-NIT-BLACK-4 (катушка 4)
//	PETG-STARPLAST-BLACK-1-1 → PETG-STARPLAST-BLACK-1 (катушка 1)
//	PETG-NIT-WHITE-4-1 → PETG-NIT-WHITE-4 (катушка 4)
//
// Меняется только токен материала в поле filename, остальные байты
// (разделители, запятые, даты, кодировка, окончания строк) — untouched.
// Строки без кода или без маппинга — без изменений, попадают в отчёт.
//
// Запуск из корня репо:
//
//	go run ./tools/rename_history
package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"3dprint-calc/core/importers"
)

// disputeFixes — ошибочные суффиксы слайсера: полный новый код → исправленный.
var disputeFixes = map[string]string{
	"PETG-NIT-BLACK-4-1":       "PETG-NIT-BLACK-4",
	"PETG-STARPLAST-BLACK-1-1": "PETG-STARPLAST-BLACK-1",
	"PETG-NIT-WHITE-4-1":       "PETG-NIT-WHITE-4",
}

func main() {
	in := flag.String("in", "csv/print_history.csv", "родная выгрузка")
	oldMap := flag.String("old", "docs/materials-export.csv", "старые коды")
	newMap := flag.String("new", "docs/materials-export_renamed.csv", "новые коды")
	out := flag.String("out", "csv/print_history_updated.csv", "выходной файл")
	flag.Parse()
	if err := run(*in, *oldMap, *newMap, *out); err != nil {
		fmt.Fprintln(os.Stderr, "ERR:", err)
		os.Exit(1)
	}
}

// flexRe ищет старый код с любой границей (_/-/начало/точка) и любыми
// разделителями внутри: -PETG-NIT-W1_ тоже находится. Хвост-сепаратор
// поглощается второй группой (RE2 без lookahead) и возвращается на место.
func flexRe(code string) *regexp.Regexp {
	var sb strings.Builder
	sb.WriteString(`(?i)(?:^|[_-])(`)
	for _, r := range code {
		if r == '-' {
			sb.WriteString(`[_-]`)
		} else {
			sb.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	sb.WriteString(`)([_.-])`)
	return regexp.MustCompile(sb.String())
}

// nitFirstRe ловит перевёрнутый порядок: NIT-PETG-B3 → PETG-NIT-B3
// (NIT здесь вендор/профиль, полный код собирается как PETG-NIT-<хвост>).
var nitFirstRe = regexp.MustCompile(`(?i)(?:^|[_-])NIT-(PETG)-([A-Za-z0-9-]+)([_.-])`)

func readLines(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Делим с сохранением окончаний строк.
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	// Финальный \n даёт пустой хвост — убираем, допишем при записи.
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	return lines, nil
}

func run(inPath, oldPath, newPath, outPath string) error {
	oldLines, err := readLines(oldPath)
	if err != nil {
		return err
	}
	newLines, err := readLines(newPath)
	if err != nil {
		return err
	}
	if len(oldLines) != len(newLines) {
		return fmt.Errorf("маппинги разной длины: %d vs %d", len(oldLines), len(newLines))
	}
	// Маппинг старый код (upper) → новый код, построчно, без шапки.
	mapping := map[string]string{}
	for i := 1; i < len(oldLines); i++ {
		oldF := strings.Split(oldLines[i], ";")
		newF := strings.Split(newLines[i], ";")
		if len(oldF) == 0 || len(newF) == 0 || oldF[0] == "" || newF[0] == "" {
			continue
		}
		newCode := strings.ToUpper(strings.TrimSpace(newF[0]))
		if fix, ok := disputeFixes[newCode]; ok {
			newCode = fix
		}
		mapping[strings.ToUpper(strings.TrimSpace(oldF[0]))] = newCode
	}
	// Проверка новых кодов новой нотацией: база + номер.
	targets := map[string]bool{}
	for _, nc := range mapping {
		targets[nc] = true
	}
	for tc := range targets {
		if base, n := importers.ParseMaterialSpool("x_" + tc + "_2h0m.gcode"); n <= 0 || base == "" {
			return fmt.Errorf("цель %q не разбирается новой нотацией", tc)
		}
	}

	inLines, err := readLines(inPath)
	if err != nil {
		return err
	}
	if len(inLines) == 0 {
		return fmt.Errorf("пустой вход")
	}
	out := make([]string, 0, len(inLines))
	out = append(out, inLines[0]) // шапка как есть
	var renamed, noCode, unmapped int
	perTarget := map[string]int{}
	var unmappedCodes []string
	seenUnmapped := map[string]bool{}
	var noCodeSamples []string
	for _, line := range inLines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		field := line
		rest := ""
		if i := strings.Index(line, ";"); i >= 0 {
			field, rest = line[:i], line[i:]
		}
		code := importers.ParseMaterial(field)
		via := "parse"
		if code == "" {
			// Гибкое совпадение по маппингу (дефисная граница, регистр).
			for old, nc := range mapping {
				if loc := flexRe(old).FindStringSubmatchIndex(field); loc != nil {
					code = old
					_ = nc
					break
				}
			}
			if code == "" {
				// Перевёрнутый порядок NIT-PETG-B3.
				if m := nitFirstRe.FindStringSubmatch(field); m != nil {
					cand := strings.ToUpper(m[1] + "-NIT-" + m[2])
					if _, ok := mapping[cand]; ok {
						code = cand
						via = "nitfirst"
					}
				}
			} else {
				via = "flex"
			}
		}
		if code == "" {
			noCode++
			if len(noCodeSamples) < 10 {
				noCodeSamples = append(noCodeSamples, field)
			}
			out = append(out, line)
			continue
		}
		newCode, ok := mapping[strings.ToUpper(code)]
		if !ok {
			unmapped++
			if !seenUnmapped[code] {
				seenUnmapped[code] = true
				unmappedCodes = append(unmappedCodes, code)
			}
			out = append(out, line)
			continue
		}
		var loc []int
		if via == "nitfirst" {
			// Меняем весь NIT-... спан на новый код c '_' границей;
			// хвостовой сепаратор (группа 3) возвращаем.
			loc = nitFirstRe.FindStringSubmatchIndex(field)
			if loc == nil || len(loc) < 8 {
				out = append(out, line)
				continue
			}
			tail := field[loc[6]:loc[7]]
			if loc[0] > 0 {
				field = field[:loc[0]] + "_" + newCode + tail + field[loc[1]:]
			} else {
				field = newCode + tail + field[loc[1]:]
			}
		} else {
			loc = flexRe(code).FindStringSubmatchIndex(field)
			if loc == nil {
				unmapped++
				if !seenUnmapped[code] {
					seenUnmapped[code] = true
					unmappedCodes = append(unmappedCodes, code+" (токен не найден)")
				}
				out = append(out, line)
				continue
			}
			// loc[2..3] — группа токена; границу '-' перед ним чиним на '_',
			// чтобы импортёру парсилось.
			start := loc[2]
			if start > 0 && field[start-1] == '-' {
				field = field[:start-1] + "_" + field[start:]
				loc = flexRe(code).FindStringSubmatchIndex(field)
				if loc == nil {
					out = append(out, line)
					continue
				}
			}
			field = field[:loc[2]] + newCode + field[loc[3]:]
		}
		// Верификация: новый токен обязан парситься импортёром.
		if got := importers.ParseMaterial(field); !strings.EqualFold(got, newCode) {
			fmt.Printf("WARN: %s → %s (парсер видит %q)\n", line, field, got)
		}
		renamed++
		perTarget[newCode]++
		out = append(out, field+rest)
	}
	sort.Strings(unmappedCodes)
	fmt.Printf("всего строк: %d\nпереименовано: %d\nбез кода: %d\nбез маппинга: %d\n",
		len(inLines)-1, renamed, noCode, unmapped)
	if len(unmappedCodes) > 0 {
		fmt.Printf("коды без маппинга: %s\n", strings.Join(unmappedCodes, ", "))
	}
	if len(noCodeSamples) > 0 {
		fmt.Println("--- без кода (примеры) ---")
		for _, s := range noCodeSamples {
			fmt.Println(s)
		}
	}
	fmt.Println("--- по целевым кодам ---")
	keys := make([]string, 0, len(perTarget))
	for k := range perTarget {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("%s: %d\n", k, perTarget[k])
	}
	return os.WriteFile(outPath, []byte(strings.Join(out, "\n")+"\n"), 0644)
}
