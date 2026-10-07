package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAssetsInSync гарантирует: копии миграций в desktop/assets побайтово
// равны канону (см. AGENTS.md). Новые миграции подхватываются автоматически —
// копируй файл в assets. CSV первым запуском больше не сеется (чистый старт).
func TestAssetsInSync(t *testing.T) {
	canonMig, err := filepath.Glob("../migrations/*.sql")
	if err != nil || len(canonMig) == 0 {
		t.Fatalf("нет канонических миграций: %v", err)
	}
	pairs := map[string]string{}
	for _, f := range canonMig {
		pairs["assets/migrations/"+filepath.Base(f)] = f
	}
	// Лишних копий без канона тоже быть не должно.
	assetMig, err := filepath.Glob("assets/migrations/*.sql")
	if err != nil {
		t.Fatal(err)
	}
	if len(assetMig) != len(canonMig) {
		t.Fatalf("assets/migrations: %d файлов, канон: %d", len(assetMig), len(canonMig))
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
