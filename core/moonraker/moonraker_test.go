package moonraker

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

const sampleHistory = `{"result": {"count": 2, "jobs": [
  {"filename": "my_rack_PETG-B5_2h1m.gcode", "status": "completed",
   "start_time": 1759495020, "end_time": 1759502462,
   "print_duration": 7024, "total_duration": 7442, "filament_used": 20144.767,
   "metadata": {"estimated_time": 7250, "slicer": "OrcaSlicer 2.4.2"}},
  {"filename": "test_PETG_5m.gcode", "status": "cancelled",
   "start_time": 1759490000, "end_time": 1759490600,
   "print_duration": 121, "total_duration": 531, "filament_used": 174.495,
   "metadata": {}}
]}}`

func testServer(t *testing.T, body string, code int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/server/info" {
			w.Write([]byte(`{"result": {"moonraker_version": "v0.9.3"}}`))
			return
		}
		w.WriteHeader(code)
		w.Write([]byte(body))
	}))
}

// pagedServer отдаёт jobs с учётом limit/start, count = всего.
func pagedServer(t *testing.T, n int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		limit, start := atoi(q.Get("limit"), 100), atoi(q.Get("start"), 0)
		var sb strings.Builder
		sb.WriteString(`{"result": {"count": ` + itoa(n) + `, "jobs": [`)
		for i := start; i < start+limit && i < n; i++ {
			if i > start {
				sb.WriteString(",")
			}
			fmt.Fprintf(&sb, `{"filename": "job_%03d.gcode", "status": "completed",
				"start_time": %d, "end_time": %d,
				"print_duration": 100, "total_duration": 120, "filament_used": 1000,
				"metadata": {}}`, i, 1759495020+i*100, 1759495120+i*100)
		}
		sb.WriteString(`]}}`)
		w.Write([]byte(sb.String()))
	}))
}

func atoi(s string, fb int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return fb
	}
	return n
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

func TestHistoryAllPages(t *testing.T) {
	srv := pagedServer(t, 5)
	defer srv.Close()
	var calls [][2]int
	jobs, err := New(srv.URL).HistoryAll(context.Background(), 2, func(fetched, total int) {
		calls = append(calls, [2]int{fetched, total})
	})
	if err != nil {
		t.Fatalf("HistoryAll: %v", err)
	}
	if len(jobs) != 5 {
		t.Fatalf("jobs = %d, хочу 5", len(jobs))
	}
	for i, j := range jobs {
		want := fmt.Sprintf("job_%03d.gcode", i)
		if j.Filename != want {
			t.Fatalf("jobs[%d] = %q, хочу %q", i, j.Filename, want)
		}
	}
	// 5 работ страницами по 2 + финальный пустой запрос:
	// колбэки (2,5), (4,5), (5,5), (5,5).
	if len(calls) != 4 || calls[0] != [2]int{2, 5} || calls[3] != [2]int{5, 5} {
		t.Fatalf("progress = %v", calls)
	}
}

// realServer имитирует настоящий Moonraker: count = размер СТРАНИЦЫ (не базы),
// страницы кэпятся на capPerPage, job_id уникальные.
func realServer(t *testing.T, n, capPerPage int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/server/history/totals" {
			fmt.Fprintf(w, `{"result": {"job_totals": {"total_jobs": %d}}}`, n)
			return
		}
		q := r.URL.Query()
		limit, start := atoi(q.Get("limit"), 100), atoi(q.Get("start"), 0)
		if limit > capPerPage {
			limit = capPerPage
		}
		var sb strings.Builder
		jobs := []string{}
		for i := start; i < start+limit && i < n; i++ {
			jobs = append(jobs, fmt.Sprintf(`{"job_id": "%06X", "filename": "job_%03d.gcode",
				"status": "completed", "start_time": %d, "end_time": %d,
				"print_duration": 100, "total_duration": 120, "filament_used": 1000,
				"metadata": {}}`, i, i, 1759495020+i*100, 1759495120+i*100))
		}
		fmt.Fprintf(&sb, `{"result": {"count": %d, "jobs": [%s]}}`, len(jobs), strings.Join(jobs, ","))
		w.Write([]byte(sb.String()))
	}))
}

// TestHistoryAllCappedServer — регресс бага 1.0.2: сервер кэпит страницы на 100,
// count = размер страницы. Обязаны забрать все 491, а не первые 100.
func TestHistoryAllCappedServer(t *testing.T) {
	srv := realServer(t, 491, 100)
	defer srv.Close()
	jobs, err := New(srv.URL).HistoryAll(context.Background(), 250, nil)
	if err != nil {
		t.Fatalf("HistoryAll: %v", err)
	}
	if len(jobs) != 491 {
		t.Fatalf("jobs = %d, хочу 491", len(jobs))
	}
	if jobs[0].Filename != "job_000.gcode" || jobs[490].Filename != "job_490.gcode" {
		t.Fatalf("границы: %q .. %q", jobs[0].Filename, jobs[490].Filename)
	}
}

// TestHistoryAllIgnoresStart — сервер игнорирует start (всегда первые N):
// останавливаемся без вечного цикла, дубли выбрасываются.
func TestHistoryAllIgnoresStart(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		limit, _ := strconv.Atoi(q.Get("limit"))
		if limit <= 0 {
			limit = 100
		}
		var sb strings.Builder
		jobs := []string{}
		for i := 0; i < limit && i < 100; i++ {
			jobs = append(jobs, fmt.Sprintf(`{"job_id": "%06X", "filename": "same_%03d.gcode",
				"status": "completed", "start_time": 1759495020, "end_time": 1759495120,
				"print_duration": 100, "total_duration": 120, "filament_used": 1000,
				"metadata": {}}`, i, i))
		}
		fmt.Fprintf(&sb, `{"result": {"count": %d, "jobs": [%s]}}`, len(jobs), strings.Join(jobs, ","))
		w.Write([]byte(sb.String()))
	}))
	defer srv.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		jobs, err := New(srv.URL).HistoryAll(context.Background(), 100, nil)
		if err != nil {
			t.Errorf("HistoryAll: %v", err)
			return
		}
		if len(jobs) != 100 {
			t.Errorf("jobs = %d, хочу 100 уникальных", len(jobs))
		}
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("вечный цикл: сервер без start не останавливается")
	}
}

func TestTotals(t *testing.T) {
	srv := realServer(t, 491, 100)
	defer srv.Close()
	n, err := New(srv.URL).Totals(context.Background())
	if err != nil {
		t.Fatalf("Totals: %v", err)
	}
	if n != 491 {
		t.Fatalf("total_jobs = %d, хочу 491", n)
	}
}

func TestHistoryAllEmpty(t *testing.T) {
	srv := pagedServer(t, 0)
	defer srv.Close()
	jobs, err := New(srv.URL).HistoryAll(context.Background(), 100, nil)
	if err != nil {
		t.Fatalf("HistoryAll: %v", err)
	}
	if jobs == nil || len(jobs) != 0 {
		t.Fatalf("пусто обязано быть []: %#v", jobs)
	}
}

func TestHistoryParse(t *testing.T) {
	srv := testServer(t, sampleHistory, 200)
	defer srv.Close()
	c := New(srv.URL)
	jobs, total, err := c.History(context.Background(), 100, 0)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if total != 2 || len(jobs) != 2 {
		t.Fatalf("jobs=%d total=%d", len(jobs), total)
	}
	j := jobs[0]
	if j.Filename != "my_rack_PETG-B5_2h1m.gcode" || j.Status != "completed" {
		t.Errorf("job = %+v", j)
	}
	if j.PrintSec != 7024 || j.TotalSec != 7442 || j.EstimatedSec != 7250 {
		t.Errorf("durations = %v/%v/%v", j.PrintSec, j.TotalSec, j.EstimatedSec)
	}
	if j.Slicer != "OrcaSlicer 2.4.2" {
		t.Errorf("slicer = %q", j.Slicer)
	}
	wantStart := time.Unix(1759495020, 0).Local()
	if !j.Start.Equal(wantStart) {
		t.Errorf("start = %v, хочу %v", j.Start, wantStart)
	}
	// Пустые metadata: оценка = print_duration, slicer = moonraker.
	if jobs[1].EstimatedSec != 121 || jobs[1].Slicer != "moonraker" {
		t.Errorf("fallback = %+v", jobs[1])
	}
	pj := j.ToPrintJob()
	if pj.Type != "job" || pj.FilamentUsed != 20144.767 || pj.FilamentTotal != 20144.767 {
		t.Errorf("mapped = %+v", pj)
	}
	if pj.EstimatedTime != 7250 || pj.PrintDuration != 7024 {
		t.Errorf("mapped durations = %+v", pj)
	}
}

func TestPing(t *testing.T) {
	srv := testServer(t, sampleHistory, 200)
	defer srv.Close()
	v, err := New(srv.URL).Ping(context.Background())
	if err != nil || v != "v0.9.3" {
		t.Fatalf("Ping = %q, %v", v, err)
	}
}

func TestHistoryHTTPError(t *testing.T) {
	srv := testServer(t, "oops", 500)
	defer srv.Close()
	if _, _, err := New(srv.URL).History(context.Background(), 10, 0); err == nil {
		t.Fatal("500 обязан вернуть ошибку")
	}
}

func TestConnRefused(t *testing.T) {
	c := New("http://127.0.0.1:1")
	c.http.Timeout = time.Second
	if _, _, err := c.History(context.Background(), 10, 0); err == nil {
		t.Fatal("отказ соединения обязан вернуть ошибку")
	}
}

func TestNormalizeBase(t *testing.T) {
	cases := []struct{ in, want string }{
		{"192.168.0.109:7125", "http://192.168.0.109:7125"}, // кейс пользователя
		{"192.168.0.109:7125/", "http://192.168.0.109:7125"},
		{"  192.168.0.109:7125  ", "http://192.168.0.109:7125"},
		{"http://192.168.0.109:7125", "http://192.168.0.109:7125"},
		{"http://192.168.0.109:7125/", "http://192.168.0.109:7125"},
		{"https://printer.local", "https://printer.local"},
		{"", ""},
	}
	for _, c := range cases {
		if got := NormalizeBase(c.in); got != c.want {
			t.Errorf("NormalizeBase(%q) = %q, хочу %q", c.in, got, c.want)
		}
		if got := New(c.in).base; c.want != "" && got != c.want {
			t.Errorf("New(%q).base = %q, хочу %q", c.in, got, c.want)
		}
	}
}
