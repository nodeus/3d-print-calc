// Package ordnum — номера заказов по шаблону.
//
// Токены (CLDR-подобные, правило удвоения — повтор буквы расширяет поле):
//
//	YYYY — год 4 цифры, YY — год 2 цифры
//	MM — месяц 2 цифры, M — без нуля; DD/D, HH/H, mm/m, ss/s — аналогично
//	n, nn, nnnn… — счётчик; длина серии = минимальная ширина (nnnn + 42 → 0042)
//	{"..."} — произвольный текст как есть
//
// Остальные символы — литералы. Незакрытая {" — ошибка ERR_ORDNUM.
package ordnum

import (
	"fmt"
	"strings"
	"time"
)

// DefaultTemplate — шаблон по умолчанию: 2026-0042.
const DefaultTemplate = "{YYYY}-{nnnn}"

// Render подставляет дату и счётчик в шаблон.
// Конструкция {…}: внутри либо токен (YYYY, nnnn…), либо литерал {"..."}.
// Всё вне скобок и одиночные { без пары — литералы… нет: незакрытая {
// и неизвестное содержимое — ошибка ERR_ORDNUM (ловит опечатки вроде {yyyy}).
func Render(template string, at time.Time, seq int) (string, error) {
	var b strings.Builder
	i := 0
	for i < len(template) {
		if template[i] != '{' {
			b.WriteByte(template[i])
			i++
			continue
		}
		rel := strings.IndexByte(template[i:], '}')
		if rel < 0 {
			return "", fmt.Errorf("ERR_ORDNUM: незакрытая {")
		}
		inner := template[i+1 : i+rel]
		if strings.HasPrefix(inner, `"`) {
			if len(inner) < 2 || !strings.HasSuffix(inner, `"`) {
				return "", fmt.Errorf("ERR_ORDNUM: битый литерал {\"...\"}")
			}
			b.WriteString(inner[1 : len(inner)-1])
		} else if inner == "" {
			return "", fmt.Errorf("ERR_ORDNUM: пустые {}")
		} else {
			c := inner[0]
			if !isTokenLetter(c) {
				return "", fmt.Errorf("ERR_ORDNUM: неизвестный токен {%s}", inner)
			}
			for k := 1; k < len(inner); k++ {
				if inner[k] != c {
					return "", fmt.Errorf("ERR_ORDNUM: смешанный токен {%s}", inner)
				}
			}
			v, err := evalRun(c, len(inner), at, seq)
			if err != nil {
				return "", err
			}
			b.WriteString(v)
		}
		i += rel + 1
	}
	return b.String(), nil
}

func isTokenLetter(c byte) bool {
	switch c {
	case 'Y', 'M', 'D', 'H', 'm', 's', 'n':
		return true
	}
	return false
}

// evalRun вычисляет серию: len — длина серии (правило удвоения).
func evalRun(c byte, length int, at time.Time, seq int) (string, error) {
	if seq < 0 {
		return "", fmt.Errorf("ERR_ORDNUM: отрицательный счётчик")
	}
	pad := func(v, width int) string {
		s := fmt.Sprintf("%d", v)
		for len(s) < width {
			s = "0" + s
		}
		return s
	}
	switch c {
	case 'Y':
		if length == 2 {
			return pad(at.Year()%100, 2), nil
		}
		return fmt.Sprintf("%d", at.Year()), nil
	case 'M':
		if length >= 2 {
			return pad(int(at.Month()), 2), nil
		}
		return fmt.Sprintf("%d", at.Month()), nil
	case 'D':
		if length >= 2 {
			return pad(at.Day(), 2), nil
		}
		return fmt.Sprintf("%d", at.Day()), nil
	case 'H':
		if length >= 2 {
			return pad(at.Hour(), 2), nil
		}
		return fmt.Sprintf("%d", at.Hour()), nil
	case 'm':
		if length >= 2 {
			return pad(at.Minute(), 2), nil
		}
		return fmt.Sprintf("%d", at.Minute()), nil
	case 's':
		if length >= 2 {
			return pad(at.Second(), 2), nil
		}
		return fmt.Sprintf("%d", at.Second()), nil
	case 'n':
		return pad(seq, length), nil
	}
	return "", fmt.Errorf("ERR_ORDNUM: неизвестный токен %q", string(c))
}

// Preview рендерит шаблон на примере (счётчик-образец 42).
func Preview(template string, at time.Time) (string, error) {
	return Render(template, at, 42)
}
