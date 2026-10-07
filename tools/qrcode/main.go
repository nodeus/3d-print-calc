// Command qrcode — разовая генерация QR-кода доната в assets фронтенда.
//
// Запуск из корня репо:
//
//	go run ./tools/qrcode
//
// Пишет desktop/frontend/src/assets/donate-qr.png (коммитится, ~1 КБ).
// Библиотека skip2/go-qrcode — pure Go, в рантайм приложения не входит.
package main

import (
	"fmt"
	"os"

	qrcode "github.com/skip2/go-qrcode"
)

const (
	donateURL = "https://pay.cloudtips.ru/p/463b1251"
	outPath   = "desktop/frontend/src/assets/donate-qr.png"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "qrcode:", err)
		os.Exit(1)
	}
}

func run() error {
	if err := os.MkdirAll("desktop/frontend/src/assets", 0755); err != nil {
		return err
	}
	// Medium+Higher? Средний уровень коррекции, 256px — читается с экрана.
	if err := qrcode.WriteFile(donateURL, qrcode.Medium, 256, outPath); err != nil {
		return err
	}
	fmt.Println("wrote", outPath)
	return nil
}
