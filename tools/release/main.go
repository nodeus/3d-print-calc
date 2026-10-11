// Command release — релизная сборка десктопа в папки с версией и датой.
//
// Раскладка артефактов:
//
//	release/<version>/<YYYY-MM-DD>/<os>/<arch>/3dprint-calc(.exe) + build-info.txt + SHA256SUMS
//
// Версия — из desktop/version.go (const Version), дата — сегодня или -date.
// Сборка — через wails (обязан быть на PATH), ldflags прошивает BuildDate.
// Windows-exe пакуются UPX (utils/upx.exe, флаг -upx=false отключает).
//
// Запуск из корня репо:
//
//	go run ./tools/release -platforms windows/amd64
//	go run ./tools/release -platforms windows/amd64,linux/amd64 -version 1.0.0 -date 2026-10-08 -upx=false
package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var versionRe = regexp.MustCompile(`(?m)^\s*Version\s*=\s*"([^"]+)"`)

func main() {
	platforms := flag.String("platforms", "windows/amd64", "платформы через запятую (wails: windows/amd64,linux/amd64,darwin/universal)")
	version := flag.String("version", "", "версия (по умолчанию — Version из desktop/version.go)")
	date := flag.String("date", "", "дата сборки ГГГГ-ММ-ДД (по умолчанию — сегодня)")
	out := flag.String("out", "release", "корень релизных папок")
	upx := flag.Bool("upx", true, "паковать exe через UPX (utils/upx.exe)")
	flag.Parse()
	if err := run(*platforms, *version, *date, *out, *upx); err != nil {
		fmt.Fprintln(os.Stderr, "release:", err)
		os.Exit(1)
	}
}

func run(platforms, version, date, out string, upx bool) error {
	if version == "" {
		var err error
		version, err = readVersion("desktop/version.go")
		if err != nil {
			return err
		}
	}
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	root, err := filepath.Abs(out)
	if err != nil {
		return err
	}
	for _, p := range strings.Split(platforms, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if err := buildOne(p, version, date, root, upx); err != nil {
			return err
		}
	}
	return nil
}

// readVersion достаёт Version из desktop/version.go (единственный источник).
func readVersion(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	m := versionRe.FindSubmatch(raw)
	if m == nil {
		return "", fmt.Errorf("не найден Version в %s", path)
	}
	return string(m[1]), nil
}

func buildOne(platform, version, date, root string, upx bool) error {
	dir := filepath.Join(root, version, date, platform)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	fmt.Printf("== %s -> %s\n", platform, dir)
	ldflags := fmt.Sprintf("-X main.BuildDate=%s", date)
	cmd := exec.Command("wails", "build",
		"-platform", platform,
		"-ldflags", ldflags)
	cmd.Dir = "desktop"
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("wails build %s: %w", platform, err)
	}
	// wails кладёт бинарник в desktop/build/bin.
	binDir := filepath.Join("desktop", "build", "bin")
	exes, err := filepath.Glob(filepath.Join(binDir, "3dprint-calc*"))
	if err != nil || len(exes) == 0 {
		return fmt.Errorf("бинарник не найден в %s", binDir)
	}
	sums := []string{}
	for _, exe := range exes {
		if strings.HasSuffix(exe, ".db") || strings.HasSuffix(exe, ".db-wal") || strings.HasSuffix(exe, ".db-shm") {
			continue
		}
		name := filepath.Base(exe)
		dst := filepath.Join(dir, name)
		if err := copyFile(exe, dst); err != nil {
			return err
		}
		if upx && strings.HasSuffix(strings.ToLower(name), ".exe") {
			if err := packUPX(dst); err != nil {
				return err
			}
		}
		sum, err := sha256File(dst)
		if err != nil {
			return err
		}
		sums = append(sums, fmt.Sprintf("%s  %s", sum, name))
		fmt.Printf("   %s (%s)\n", name, sum[:12])
	}
	info := fmt.Sprintf("version: %s\ndate: %s\nplatform: %s\n", version, date, platform)
	if out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output(); err == nil {
		info += fmt.Sprintf("commit: %s", strings.TrimSpace(string(out)))
		info += "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "build-info.txt"), []byte(info), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "SHA256SUMS"), []byte(strings.Join(sums, "\n")+"\n"), 0644); err != nil {
		return err
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// packUPX сжимает exe через utils/upx.exe (лежит локально, в git не идёт).
// UPX может давать ложные срабатывания антивирусов и чуть более медленный
// холодный старт — см. docs/release.md.
func packUPX(path string) error {
	upx := filepath.Join("utils", "upx.exe")
	if _, err := os.Stat(upx); err != nil {
		return fmt.Errorf("UPX не найден (%s) — соберите с -upx=false: %w", upx, err)
	}
	before, _ := fileSize(path)
	cmd := exec.Command(upx, "--lzma", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("upx: %w", err)
	}
	after, _ := fileSize(path)
	fmt.Printf("   upx: %s -> %s\n", humanSize(before), humanSize(after))
	return nil
}

func fileSize(path string) (int64, error) {
	st, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return st.Size(), nil
}

func humanSize(n int64) string {
	return fmt.Sprintf("%.1f МБ", float64(n)/1048576)
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
