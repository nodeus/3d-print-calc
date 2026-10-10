package sqlite

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"3dprint-calc/core/importers"
	"3dprint-calc/core/models"
	"3dprint-calc/store-sqlite/seed"
)

// BackupVersion — формат backup.zip. Менять при несовместимых изменениях.
const BackupVersion = 1

// Manifest — описание бэкапа (meta.json внутри zip, dry-run наружу).
type Manifest struct {
	Version   int              `json:"version"`
	CreatedAt string           `json:"created_at"`
	Counts    map[string]int64 `json:"counts"`
	Files     []string         `json:"files,omitempty"`
}

// TableCounts возвращает число строк по ключевым таблицам.
func (d *DB) TableCounts() (map[string]int64, error) {
	out := map[string]int64{}
	for _, t := range []string{"materials", "spools", "printers", "jobs", "orders", "customers", "users"} {
		var n int64
		if err := d.sql.QueryRow(`SELECT COUNT(*) FROM ` + t).Scan(&n); err != nil {
			return nil, fmt.Errorf("ERR_BACKUP_COUNT: %w", err)
		}
		out[t] = n
	}
	return out, nil
}

// CreateBackup пишет backup.zip: app.db (консистентный снапшот через
// VACUUM INTO) + meta.json + jobs.csv + materials.csv.
func (d *DB) CreateBackup(w io.Writer) error {
	tmp, err := os.CreateTemp("", "3dprint-backup-*.db")
	if err != nil {
		return fmt.Errorf("ERR_BACKUP: %w", err)
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)
	if _, err := d.sql.Exec(`VACUUM INTO '` + strings.ReplaceAll(tmpPath, `'`, `''`) + `'`); err != nil {
		return fmt.Errorf("ERR_BACKUP_SNAPSHOT: %w", err)
	}
	dbBytes, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("ERR_BACKUP: %w", err)
	}
	counts, err := d.TableCounts()
	if err != nil {
		return err
	}
	meta, _ := json.Marshal(Manifest{
		Version: BackupVersion, CreatedAt: time.Now().UTC().Format(time.RFC3339), Counts: counts,
	})
	zw := zip.NewWriter(w)
	add := func(name string, data []byte) error {
		fw, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = fw.Write(data)
		return err
	}
	if err := add("app.db", dbBytes); err != nil {
		return fmt.Errorf("ERR_BACKUP: %w", err)
	}
	if err := add("meta.json", meta); err != nil {
		return fmt.Errorf("ERR_BACKUP: %w", err)
	}
	var jobsCSV, matsCSV bytes.Buffer
	if err := d.ExportJobsCSV(&jobsCSV); err != nil {
		return err
	}
	if err := add("jobs.csv", jobsCSV.Bytes()); err != nil {
		return fmt.Errorf("ERR_BACKUP: %w", err)
	}
	if err := d.ExportMaterialsCSV(&matsCSV); err != nil {
		return err
	}
	if err := add("materials.csv", matsCSV.Bytes()); err != nil {
		return fmt.Errorf("ERR_BACKUP: %w", err)
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("ERR_BACKUP: %w", err)
	}
	return nil
}

// InspectBackup проверяет бэкап без записи: zip цел, meta валидна,
// app.db — настоящий SQLite. Возвращает манифест для dry-run в UI.
func InspectBackup(data []byte) (Manifest, error) {
	var m Manifest
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return m, fmt.Errorf("ERR_BACKUP_INSPECT: не zip: %w", err)
	}
	var metaData, dbHead []byte
	for _, f := range zr.File {
		m.Files = append(m.Files, f.Name)
		rc, err := f.Open()
		if err != nil {
			return m, fmt.Errorf("ERR_BACKUP_INSPECT: %w", err)
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return m, fmt.Errorf("ERR_BACKUP_INSPECT: %w", err)
		}
		switch f.Name {
		case "meta.json":
			metaData = content
		case "app.db":
			dbHead = content
		}
	}
	if metaData == nil {
		return m, fmt.Errorf("ERR_BACKUP_INSPECT: нет meta.json")
	}
	if err := json.Unmarshal(metaData, &m); err != nil {
		return m, fmt.Errorf("ERR_BACKUP_INSPECT: битая meta.json: %w", err)
	}
	if m.Version != BackupVersion {
		return m, fmt.Errorf("ERR_BACKUP_INSPECT: версия %d, поддерживается %d", m.Version, BackupVersion)
	}
	if dbHead == nil || len(dbHead) < 16 || string(dbHead[:16]) != "SQLite format 3\x00" {
		return m, fmt.Errorf("ERR_BACKUP_INSPECT: app.db — не SQLite")
	}
	return m, nil
}

// RestoreAppDB валидирует бэкап и заменяет файл БД.
// Соединение обязано быть закрыто вызывающим кодом до вызова.
func RestoreAppDB(data []byte, destPath string) error {
	m, err := InspectBackup(data)
	if err != nil {
		return err
	}
	_ = m
	zr, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	for _, f := range zr.File {
		if f.Name != "app.db" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("ERR_BACKUP_RESTORE: %w", err)
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return fmt.Errorf("ERR_BACKUP_RESTORE: %w", err)
		}
		tmp := destPath + ".restore-tmp"
		if err := os.WriteFile(tmp, content, 0600); err != nil {
			return fmt.Errorf("ERR_BACKUP_RESTORE: %w", err)
		}
		// Проверяем, что БД открывается и есть таблица jobs.
		check, err := Open(tmp)
		if err != nil {
			os.Remove(tmp)
			return fmt.Errorf("ERR_BACKUP_RESTORE: %w", err)
		}
		var n int
		cerr := check.sql.QueryRow(`SELECT COUNT(*) FROM jobs`).Scan(&n)
		check.Close()
		if cerr != nil {
			os.Remove(tmp)
			return fmt.Errorf("ERR_BACKUP_RESTORE: не база приложения: %w", cerr)
		}
		if err := os.Rename(tmp, destPath); err != nil {
			os.Remove(tmp)
			return fmt.Errorf("ERR_BACKUP_RESTORE: %w", err)
		}
		// WAL-хвосты от прошлого файла недействительны после замены.
		for _, ext := range []string{"-wal", "-shm", "-journal"} {
			os.Remove(destPath + ext)
		}
		return nil
	}
	return fmt.Errorf("ERR_BACKUP_RESTORE: нет app.db в архиве")
}

const csvDateOut = "02.01.2006 15:04"

// ExportJobsCSV выгружает историю в формате принтера (пригодно для реимпорта).
func (d *DB) ExportJobsCSV(w io.Writer) error {
	cw := csv.NewWriter(w)
	cw.Comma = ';'
	header := []string{"filename", "type", "status", "size", "start_time", "end_time",
		"estimated_time", "print_duration", "total_duration", "filament_total", "filament_used", "slicer"}
	if err := cw.Write(header); err != nil {
		return fmt.Errorf("ERR_EXPORT_CSV: %w", err)
	}
	jobs, err := d.Jobs().List(0)
	if err != nil {
		return err
	}
	for _, j := range jobs {
		rec := []string{
			j.Filename, "job", j.Status, itoa(j.SizeBytes),
			reformatDate(j.StartAt), reformatDate(j.EndAt), itoa(j.EstimatedSec),
			itoa(j.PrintSec), itoa(j.TotalSec),
			commaFloat(j.FilamentTotalMm), commaFloat(j.LengthM * 1000), j.Slicer,
		}
		if err := cw.Write(rec); err != nil {
			return fmt.Errorf("ERR_EXPORT_CSV: %w", err)
		}
	}
	cw.Flush()
	return cw.Error()
}

// ExportMaterialsCSV выгружает справочник: code;type;vendor;price_per_kg;currency;density;diameter.
func (d *DB) ExportMaterialsCSV(w io.Writer) error {
	cw := csv.NewWriter(w)
	cw.Comma = ';'
	if err := cw.Write([]string{"code", "type", "vendor", "price_per_kg", "currency", "density", "diameter"}); err != nil {
		return fmt.Errorf("ERR_EXPORT_CSV: %w", err)
	}
	mats, err := d.Materials().List()
	if err != nil {
		return err
	}
	for _, m := range mats {
		price, currency := "0", "RUB"
		if spools, err := d.Materials().ListSpools(m.ID); err == nil && len(spools) > 0 {
			price = commaFloat(float64(spools[0].Price) / 100)
			currency = spools[0].Currency
		}
		rec := []string{m.Code, m.Type, m.Vendor, price, currency, commaFloat(m.Density), commaFloat(m.Diameter)}
		if err := cw.Write(rec); err != nil {
			return fmt.Errorf("ERR_EXPORT_CSV: %w", err)
		}
	}
	cw.Flush()
	return cw.Error()
}

// ImportMaterials импортирует справочник материалов. dryRun=true —
// только отчёт без записи.
//
// Новая парадигма «материал + катушки»: строка с кодом вида КОД-N —
// это N-я катушка базового материала (создаётся материал при отсутствии
// и катушка с этим номером и ценой строки, хоть нулевой). Строка без
// номера — материал (+ первая катушка при цене > 0, legacy).
// Точный код из базы — приоритет (legacy без номеров).
// Существующее не трогается вообще (ни поля, ни цены, ни катушки);
// дубль номера внутри файла — ошибка со строкой. bought_at пусто
// (дата неизвестна). Повтор идемпотентен.
func (d *DB) ImportMaterials(r io.Reader, dryRun bool) (ImportReport, error) {
	var rep ImportReport
	rep.DryRun = dryRun
	rows, err := importers.ParseMaterials(r)
	if err != nil {
		return rep, err
	}
	rep.Total = len(rows)
	mats := d.Materials()
	list, err := mats.List()
	if err != nil {
		return rep, fmt.Errorf("ERR_IMPORT: %w", err)
	}
	type spoolPlan struct {
		n        int
		price    float64
		currency string
		row      int
	}
	type matPlan struct {
		isNew  bool // базы не было в БД — создаём (существующие не трогаем)
		fields importers.MaterialRow
		spools []spoolPlan
		bare   bool // строка без номера (legacy: первая катушка при цене)
	}
	knownMats := map[string]bool{}
	existingNos := map[string]map[int]bool{}
	for _, m := range list {
		knownMats[m.Code] = true
	}
	plans := map[string]*matPlan{}
	order := []string{}
	for i, row := range rows {
		rowNo := i + 2
		if knownMats[row.Code] {
			rep.Existing++
			continue
		}
		base, n := importers.SplitSpoolNo(row.Code)
		if n == 0 {
			base = row.Code
		} else if knownMats[base] {
			// База есть: проверяем номер среди её катушек.
			if existingNos[base] == nil {
				existingNos[base] = map[int]bool{}
				if mid, err := matIDByCode(list, base); err == nil {
					if spools, err := mats.ListSpools(mid); err == nil {
						for _, s := range spools {
							existingNos[base][s.SpoolNo] = true
						}
					}
				}
			}
			if existingNos[base][n] {
				rep.Existing++
				continue
			}
		}
		p, ok := plans[base]
		if !ok {
			p = &matPlan{isNew: !knownMats[base], fields: row}
			plans[base] = p
			order = append(order, base)
		}
		if n == 0 {
			if p.bare {
				return rep, fmt.Errorf("ERR_IMPORT: строка %d: дубль материала %q", rowNo, base)
			}
			p.bare = true
			// Поля материала — из первой строки базы.
			if len(p.spools) == 0 {
				p.fields = row
			}
		} else {
			for _, s := range p.spools {
				if s.n == n {
					return rep, fmt.Errorf("ERR_IMPORT: строка %d: дубль катушки %q №%d", rowNo, base, n)
				}
			}
			if existingNos[base] == nil {
				existingNos[base] = map[int]bool{}
			}
			existingNos[base][n] = true
			p.spools = append(p.spools, spoolPlan{n: n, price: row.PricePerKg, currency: row.Currency, row: rowNo})
		}
		rep.New++
	}
	if dryRun {
		return rep, nil
	}
	for _, base := range order {
		p := plans[base]
		var mid int64
		if p.isNew {
			density := p.fields.Density
			if density <= 0 {
				density = 1.27
			}
			matType := p.fields.Type
			if matType == "" {
				matType = "PETG"
			}
			var err error
			mid, err = mats.UpsertByCode(models.Material{
				Code: base, Type: matType, Vendor: p.fields.Vendor,
				Density: density, Diameter: p.fields.Diameter,
			})
			if err != nil {
				return rep, fmt.Errorf("ERR_IMPORT: %w", err)
			}
		} else {
			var err error
			mid, err = matIDByCode(list, base)
			if err != nil {
				return rep, err
			}
		}
		mkSpool := func(n int, price float64, currency string) error {
			if currency == "" {
				currency = "RUB"
			}
			_, err := mats.AddSpool(models.Spool{
				MaterialID: mid, SpoolNo: n, Price: int64(price*100 + 0.5),
				Currency: currency, FxRate: 1,
				WeightInitG: 1000, WeightLeftG: 1000,
			})
			return err
		}
		if p.bare && p.fields.PricePerKg > 0 {
			if err := mkSpool(0, p.fields.PricePerKg, p.fields.Currency); err != nil {
				return rep, fmt.Errorf("ERR_IMPORT: %w", err)
			}
		}
		for _, s := range p.spools {
			if err := mkSpool(s.n, s.price, s.currency); err != nil {
				return rep, fmt.Errorf("ERR_IMPORT: строка %d: %w", s.row, err)
			}
		}
	}
	return rep, nil
}

// matIDByCode ищет id материала по коду в готовом списке.
func matIDByCode(list []models.Material, code string) (int64, error) {
	for _, m := range list {
		if m.Code == code {
			return m.ID, nil
		}
	}
	return 0, fmt.Errorf("ERR_IMPORT: нет материала %q", code)
}

// ImportReport — итог импорта истории.
type ImportReport struct {
	Total         int  `json:"total"`
	New           int  `json:"new"`
	Existing      int  `json:"existing"`
	Stubs         int  `json:"stubs"`
	SpoolStubs    int  `json:"spoolStubs"`
	DryRun        bool `json:"dryRun"`
	LifetimeTotal int  `json:"lifetimeTotal,omitempty"` // всего за всё время (Moonraker totals)
}

// ImportHistory импортирует CSV принтера. dryRun=true — только отчёт без записи.
// onProgress — прогресс применения (nil — молча).
func (d *DB) ImportHistory(r io.Reader, dryRun bool, onProgress seed.ProgressFunc) (ImportReport, error) {
	var rep ImportReport
	rep.DryRun = dryRun
	rows, err := importers.ParseHistory(r)
	if err != nil {
		return rep, err
	}
	rep.Total = len(rows)
	fresh, exist, err := d.ClassifyJobs(rows)
	if err != nil {
		return rep, err
	}
	rep.New, rep.Existing = fresh, exist
	if dryRun || rep.New == 0 {
		return rep, nil
	}
	printers, err := d.Printers().List()
	if err != nil {
		return rep, fmt.Errorf("ERR_IMPORT: %w", err)
	}
	if len(printers) == 0 {
		return rep, fmt.Errorf("ERR_IMPORT: нет принтеров — создайте принтер перед импортом")
	}
	st, err := seed.ImportRows(d, printers[0].ID, rows, "import", onProgress)
	if err != nil {
		return rep, err
	}
	rep.Stubs = st.Stubs
	rep.SpoolStubs = st.SpoolStubs
	return rep, nil
}

// ClassifyJobs делит строки на новые/существующие (по filename+start_at).
// Записи нет — используется dry-run превью CSV и Moonraker.
func (d *DB) ClassifyJobs(rows []importers.PrintJob) (fresh, exist int, err error) {
	existing, err := d.Jobs().List(0)
	if err != nil {
		return 0, 0, fmt.Errorf("ERR_IMPORT: %w", err)
	}
	keys := make(map[string]bool, len(existing))
	for _, j := range existing {
		keys[j.Filename+"\x00"+j.StartAt] = true
	}
	for _, row := range rows {
		key := row.Filename + "\x00" + row.StartTime.Format("2006-01-02 15:04")
		if keys[key] {
			exist++
		} else {
			fresh++
		}
	}
	return fresh, exist, nil
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}
func commaFloat(v float64) string {
	return strings.Replace(strconv.FormatFloat(v, 'f', -1, 64), ".", ",", 1)
}

func reformatDate(iso string) string {
	// Храним "2006-01-02 15:04" → отдаём "02.01.2006 15:04".
	if t, err := time.Parse("2006-01-02 15:04", iso); err == nil {
		return t.Format(csvDateOut)
	}
	return iso
}
