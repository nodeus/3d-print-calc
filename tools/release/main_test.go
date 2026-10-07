package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadVersion(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "version.go")
	src := "package main\n\nvar (\n\tVersion = \"0.2.0\"\n\tBuildDate = \"dev\"\n)\n"
	if err := os.WriteFile(p, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	v, err := readVersion(p)
	if err != nil {
		t.Fatalf("readVersion: %v", err)
	}
	if v != "0.2.0" {
		t.Fatalf("version = %q", v)
	}
	if _, err := readVersion(filepath.Join(dir, "missing.go")); err == nil {
		t.Fatal("отсутствующий файл обязан вернуть ошибку")
	}
}
