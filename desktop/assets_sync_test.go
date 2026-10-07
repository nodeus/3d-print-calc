package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAssetsInSync гарантирует: копии в desktop/assets побайтово равны канону.
// Канон: migrations/*.sql, csv/print_history.csv (см. AGENTS.md).
func TestAssetsInSync(t *testing.T) {
	pairs := map[string]string{
		"assets/seed.csv":                 "../csv/print_history.csv",
		"assets/migrations/0001_init.sql": "../migrations/0001_init.sql",
	}
	for got, want := range pairs {
		a, err := os.ReadFile(filepath.Join(".", got))
		if err != nil {
			t.Fatalf("%s: %v", got, err)
		}
		b, err := os.ReadFile(filepath.Join(".", want))
		if err != nil {
			t.Fatalf("%s: %v", want, err)
		}
		if string(a) != string(b) {
			t.Errorf("%s рассинхронизирован с %s: скопируй канон в assets", got, want)
		}
	}
}
