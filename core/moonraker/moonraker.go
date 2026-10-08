// Package moonraker — клиент Creality K1C (Moonraker): history/list,
// проверка связи. Офлайн-first: без сети приложение работает на локальных данных.
//
// API: GET {base}/server/history/list?limit=&start=&order=desc
// → {"result": {"count": N, "jobs": [...]}}.
// GET {base}/server/info → {"result": {"moonraker_version": ...}}.
package moonraker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"3dprint-calc/core/importers"
)

// Client — HTTP-клиент Moonraker. base — напр. http://192.168.1.50:7125.
type Client struct {
	base string
	http *http.Client
}

// NormalizeBase чистит адрес: пробелы, слэш на конце, схема по умолчанию http://.
// Пользователи вводят голый IP с портом (192.168.0.109:7125) — без схемы
// http.NewRequest такой URL отвергает (first path segment...colon).
func NormalizeBase(baseURL string) string {
	base := strings.TrimSpace(baseURL)
	base = strings.TrimRight(base, "/")
	if base != "" && !strings.Contains(base, "://") {
		base = "http://" + base
	}
	return base
}

// New создаёт клиента с таймаутом 10с.
func New(baseURL string) *Client {
	return &Client{
		base: NormalizeBase(baseURL),
		http: &http.Client{Timeout: 10 * time.Second},
	}
}

// Job — одна работа из history/list.
type Job struct {
	Filename     string
	Status       string
	Start        time.Time
	End          time.Time
	PrintSec     float64
	TotalSec     float64
	EstimatedSec float64
	FilamentUsed float64 // мм
	Slicer       string
}

// ToPrintJob маппит в формат истории принтера.
// FilamentTotal неизвестен API — равен использованному; Type всегда "job".
func (j Job) ToPrintJob() importers.PrintJob {
	return importers.PrintJob{
		Filename: j.Filename, Type: "job", Status: j.Status,
		StartTime: j.Start, EndTime: j.End,
		EstimatedTime: int64(j.EstimatedSec + 0.5),
		PrintDuration: int64(j.PrintSec + 0.5),
		TotalDuration: int64(j.TotalSec + 0.5),
		FilamentTotal: j.FilamentUsed, FilamentUsed: j.FilamentUsed,
		Slicer: j.Slicer,
	}
}

type historyResponse struct {
	Result struct {
		Count int `json:"count"`
		Jobs  []struct {
			Filename      string  `json:"filename"`
			Status        string  `json:"status"`
			StartTime     float64 `json:"start_time"`
			EndTime       float64 `json:"end_time"`
			PrintDuration float64 `json:"print_duration"`
			TotalDuration float64 `json:"total_duration"`
			FilamentUsed  float64 `json:"filament_used"`
			Metadata      struct {
				EstimatedTime float64 `json:"estimated_time"`
				Slicer        string  `json:"slicer"`
			} `json:"metadata"`
		} `json:"jobs"`
	} `json:"result"`
}

// History возвращает работы (новые первыми), total — всего на сервере.
func (c *Client) History(ctx context.Context, limit, start int64) ([]Job, int, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	url := fmt.Sprintf("%s/server/history/list?limit=%d&start=%d&order=desc", c.base, limit, start)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("ERR_MOONRAKER: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("ERR_MOONRAKER_CONN: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("ERR_MOONRAKER_HTTP: статус %d", resp.StatusCode)
	}
	var hr historyResponse
	if err := json.NewDecoder(resp.Body).Decode(&hr); err != nil {
		return nil, 0, fmt.Errorf("ERR_MOONRAKER_PARSE: %w", err)
	}
	out := make([]Job, 0, len(hr.Result.Jobs))
	for _, j := range hr.Result.Jobs {
		est := j.Metadata.EstimatedTime
		if est <= 0 {
			est = j.PrintDuration
		}
		slicer := j.Metadata.Slicer
		if slicer == "" {
			slicer = "moonraker"
		}
		out = append(out, Job{
			Filename: j.Filename, Status: j.Status,
			Start: unixToLocal(j.StartTime), End: unixToLocal(j.EndTime),
			PrintSec: j.PrintDuration, TotalSec: j.TotalDuration,
			EstimatedSec: est, FilamentUsed: j.FilamentUsed, Slicer: slicer,
		})
	}
	return out, hr.Result.Count, nil
}

// HistoryAll забирает ВСЮ историю постранично (новые первыми).
// pageSize — размер страницы (1..500, иначе 250). onPage вызывается после
// каждой страницы (nil — молча). Останавливается на пустой странице или
// при достижении total с сервера.
func (c *Client) HistoryAll(ctx context.Context, pageSize int64, onPage func(fetched, total int)) ([]Job, error) {
	if pageSize <= 0 || pageSize > 500 {
		pageSize = 250
	}
	out := []Job{}
	var start int64
	for {
		if err := ctx.Err(); err != nil {
			return out, fmt.Errorf("ERR_MOONRAKER: %w", err)
		}
		page, total, err := c.History(ctx, pageSize, start)
		if err != nil {
			return out, err
		}
		out = append(out, page...)
		if onPage != nil {
			onPage(len(out), total)
		}
		if len(page) == 0 || (total > 0 && len(out) >= total) {
			return out, nil
		}
		start += int64(len(page))
	}
}

// Ping проверяет связь: GET {base}/server/info.
func (c *Client) Ping(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/server/info", nil)
	if err != nil {
		return "", fmt.Errorf("ERR_MOONRAKER: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("ERR_MOONRAKER_CONN: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ERR_MOONRAKER_HTTP: статус %d", resp.StatusCode)
	}
	var v struct {
		Result struct {
			MoonrakerVersion string `json:"moonraker_version"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return "", fmt.Errorf("ERR_MOONRAKER_PARSE: %w", err)
	}
	return v.Result.MoonrakerVersion, nil
}

// unixToLocal переводит unix-метку в локальное время ПК
// (принтер и ПК обычно в одном часовом поясе).
func unixToLocal(ts float64) time.Time {
	if ts <= 0 {
		return time.Time{}
	}
	sec := int64(ts)
	return time.Unix(sec, int64((ts-float64(sec))*1e9)).Local()
}
