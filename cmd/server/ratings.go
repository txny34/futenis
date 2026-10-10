package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// ratingStore reads and writes ranked ELO per user and mode ("1v1"/"2v2").
// Only the server writes: the table's RLS allows public reads and no writes,
// and the server uses a secret key that bypasses RLS.
type ratingStore interface {
	load(ctx context.Context, mode string, ids []string) (map[string]int, error)
	save(ctx context.Context, mode string, ratings map[string]int) error
}

// supabaseRatings talks to PostgREST: table public.ratings(user_id uuid pk,
// elo_1v1 int, elo_2v2 int, updated_at timestamptz).
type supabaseRatings struct {
	base   string // https://<ref>.supabase.co/rest/v1
	key    string // secret API key (Fly secret SUPABASE_SECRET_KEY), never logged
	client *http.Client
}

func newSupabaseRatings(projectURL, key string) *supabaseRatings {
	return &supabaseRatings{base: strings.TrimRight(projectURL, "/") + "/rest/v1", key: key, client: &http.Client{Timeout: 5 * time.Second}}
}

func column(mode string) (string, error) {
	switch mode {
	case "1v1", "2v2":
		return "elo_" + mode, nil
	}
	return "", fmt.Errorf("ratings: unknown mode %q", mode)
}

func (s *supabaseRatings) do(ctx context.Context, method, path string, body []byte, prefer string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, s.base+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("apikey", s.key)
	req.Header.Set("Content-Type", "application/json")
	if prefer != "" {
		req.Header.Set("Prefer", prefer)
	}
	return s.client.Do(req)
}

func (s *supabaseRatings) load(ctx context.Context, mode string, ids []string) (map[string]int, error) {
	col, err := column(mode)
	if err != nil {
		return nil, err
	}
	q := url.Values{"select": {"user_id," + col}, "user_id": {"in.(" + strings.Join(ids, ",") + ")"}}
	res, err := s.do(ctx, http.MethodGet, "/ratings?"+q.Encode(), nil, "")
	if err != nil {
		return nil, fmt.Errorf("ratings load: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ratings load: status %d", res.StatusCode)
	}
	var rows []map[string]any
	if err := json.NewDecoder(res.Body).Decode(&rows); err != nil {
		return nil, fmt.Errorf("ratings load: %w", err)
	}
	out := map[string]int{}
	for _, r := range rows {
		id, _ := r["user_id"].(string)
		if v, ok := r[col].(float64); ok && id != "" {
			out[id] = int(v)
		}
	}
	return out, nil
}

func (s *supabaseRatings) save(ctx context.Context, mode string, ratings map[string]int) error {
	col, err := column(mode)
	if err != nil {
		return err
	}
	rows := make([]map[string]any, 0, len(ratings))
	for id, v := range ratings {
		rows = append(rows, map[string]any{"user_id": id, col: v, "updated_at": time.Now().UTC().Format(time.RFC3339)})
	}
	body, _ := json.Marshal(rows)
	res, err := s.do(ctx, http.MethodPost, "/ratings?on_conflict=user_id", body, "resolution=merge-duplicates,return=minimal")
	if err != nil {
		return fmt.Errorf("ratings save: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		return fmt.Errorf("ratings save: status %d", res.StatusCode)
	}
	return nil
}

// memRatings is the in-process store for tests and local dev without Supabase.
type memRatings struct {
	mu sync.Mutex
	m  map[string]map[string]int
}

func newMemRatings() *memRatings { return &memRatings{m: map[string]map[string]int{}} }

func (s *memRatings) load(_ context.Context, mode string, ids []string) (map[string]int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]int{}
	for _, id := range ids {
		if v, ok := s.m[mode][id]; ok {
			out[id] = v
		}
	}
	return out, nil
}

func (s *memRatings) save(_ context.Context, mode string, r map[string]int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m[mode] == nil {
		s.m[mode] = map[string]int{}
	}
	for id, v := range r {
		s.m[mode][id] = v
	}
	return nil
}
