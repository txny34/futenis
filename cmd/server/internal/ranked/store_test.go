package ranked

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeRest is a PostgREST stand-in: ratings load/upsert and, unless legacy,
// the ranked_result function (migration 003).
func fakeRest(t *testing.T, legacy bool, rpc *map[string]any, upsert *[]map[string]any) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("apikey") != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodGet:
			if !strings.Contains(r.URL.RawQuery, "user_id=in.%28a%2Cb%29") {
				t.Errorf("filter %q", r.URL.RawQuery)
			}
			_, _ = io.WriteString(w, `[{"user_id":"a","elo_1v1":1100}]`)
		case r.URL.Path == "/rpc/ranked_result":
			if legacy {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_ = json.NewDecoder(r.Body).Decode(rpc)
			w.WriteHeader(http.StatusNoContent)
		default:
			if !strings.Contains(r.Header.Get("Prefer"), "merge-duplicates") {
				t.Errorf("not an upsert: %q", r.Header.Get("Prefer"))
			}
			_ = json.NewDecoder(r.Body).Decode(upsert)
			w.WriteHeader(http.StatusCreated)
		}
	}))
}

func TestSupabaseRatingsLoadAndRecord(t *testing.T) {
	var rpc map[string]any
	var upsert []map[string]any
	srv := fakeRest(t, false, &rpc, &upsert)
	defer srv.Close()
	s := NewSupabase(srv.URL, "secret")
	s.base = srv.URL
	got, err := s.Load(context.Background(), "1v1", []string{"a", "b"})
	if err != nil || got["a"] != 1100 || len(got) != 1 {
		t.Fatalf("load %v %v", got, err)
	}
	if err := s.Save(context.Background(), "1v1", []Row{{ID: "a", Elo: 1116, Won: true, Name: "Ana"}}); err != nil {
		t.Fatal(err)
	}
	rows, _ := rpc["p_rows"].([]any)
	row, _ := rows[0].(map[string]any)
	if rpc["p_mode"] != "1v1" || len(rows) != 1 || row["user_id"] != "a" || row["elo"] != float64(1116) || row["won"] != true || row["name"] != "Ana" {
		t.Fatalf("rpc %v", rpc)
	}
	if upsert != nil {
		t.Fatal("upsert used although ranked_result exists")
	}
	if _, err := s.Load(context.Background(), "3v3", nil); err == nil {
		t.Fatal("unknown mode accepted")
	}
}

func TestSupabaseRatingsFallsBackBeforeTheMigration(t *testing.T) {
	var rpc map[string]any
	var upsert []map[string]any
	srv := fakeRest(t, true, &rpc, &upsert)
	defer srv.Close()
	s := NewSupabase(srv.URL, "secret")
	s.base = srv.URL
	err := s.Save(context.Background(), "1v1", []Row{{ID: "a", Elo: 1116}})
	if legacy, ok := err.(ErrLegacy); !ok || legacy.Err != nil {
		t.Fatalf("want a logged legacy save with the ELO stored, got %v", err)
	}
	if len(upsert) != 1 || upsert[0]["user_id"] != "a" || upsert[0]["elo_1v1"] != float64(1116) {
		t.Fatalf("upsert %v", upsert)
	}
}
