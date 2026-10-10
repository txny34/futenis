package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A fake PostgREST checks the key, the filter and the upsert shape.
func TestSupabaseRatingsLoadAndSave(t *testing.T) {
	var saved []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("apikey") != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.Method {
		case http.MethodGet:
			if !strings.Contains(r.URL.RawQuery, "user_id=in.%28a%2Cb%29") {
				t.Errorf("filter %q", r.URL.RawQuery)
			}
			_, _ = io.WriteString(w, `[{"user_id":"a","elo_1v1":1100}]`)
		case http.MethodPost:
			if !strings.Contains(r.Header.Get("Prefer"), "merge-duplicates") {
				t.Errorf("not an upsert: %q", r.Header.Get("Prefer"))
			}
			_ = json.NewDecoder(r.Body).Decode(&saved)
			w.WriteHeader(http.StatusCreated)
		}
	}))
	defer srv.Close()
	s := newSupabaseRatings(srv.URL, "secret")
	s.base = srv.URL
	got, err := s.load(context.Background(), "1v1", []string{"a", "b"})
	if err != nil || got["a"] != 1100 || len(got) != 1 {
		t.Fatalf("load %v %v", got, err)
	}
	if err := s.save(context.Background(), "1v1", map[string]int{"a": 1116}); err != nil {
		t.Fatal(err)
	}
	if len(saved) != 1 || saved[0]["user_id"] != "a" || saved[0]["elo_1v1"] != float64(1116) {
		t.Fatalf("saved %v", saved)
	}
	if _, err := s.load(context.Background(), "3v3", nil); err == nil {
		t.Fatal("unknown mode accepted")
	}
}
