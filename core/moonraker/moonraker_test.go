package moonraker

import (
	"context"
	"net/http"
	"net/http/httptest"
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
