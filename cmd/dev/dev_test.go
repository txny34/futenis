package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestFindFirstPrefersLibWasm(t *testing.T) {
	c := wasmExecCandidates("/go")
	got, err := findFirst(c, func(string) bool { return true })
	if err != nil || got != filepath.Join("/go", "lib", "wasm", "wasm_exec.js") {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestFindFirstFallsBackToMisc(t *testing.T) {
	c := wasmExecCandidates("/go")
	got, err := findFirst(c, func(p string) bool { return p == c[1] })
	if err != nil || got != c[1] {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestFindFirstNoneExist(t *testing.T) {
	if _, err := findFirst(wasmExecCandidates("/go"), func(string) bool { return false }); err == nil {
		t.Fatal("want error")
	}
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	if err := os.WriteFile(src, []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(src, dst); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(dst); err != nil || string(b) != "hi" {
		t.Fatalf("got %q, %v", b, err)
	}
}

func newTestServer(t *testing.T, build builderFunc) *httptest.Server {
	t.Helper()
	return newTestServerClock(t, build, time.Now)
}

func newTestServerClock(t *testing.T, build builderFunc, now func() time.Time) *httptest.Server {
	t.Helper()
	root := t.TempDir()
	client := filepath.Join(root, "client")
	if err := os.MkdirAll(client, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(client, "index.html"), []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(client, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(client, "src", "main.ts"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "secret"), []byte("nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(newHandler(root, build, log.New(io.Discard, "", 0), now))
	t.Cleanup(srv.Close)
	return srv
}

func get(t *testing.T, url string) (int, string, string) {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, string(body), res.Header.Get("Cache-Control")
}

func TestHandlerServesIndexAndRebuilds(t *testing.T) {
	builds := 0
	srv := newTestServer(t, func(context.Context) error { builds++; return nil })
	code, body, cc := get(t, srv.URL+"/")
	if code != 200 || body != "ok" || cc != "no-store" || builds != 1 {
		t.Fatalf("code=%d body=%q cc=%q builds=%d", code, body, cc, builds)
	}
}

func TestHandlerBuildFailureIs500WithoutDetails(t *testing.T) {
	srv := newTestServer(t, func(context.Context) error { return errors.New("/abs/secret/path boom") })
	code, body, _ := get(t, srv.URL+"/index.html")
	if code != 500 || strings.Contains(body, "/abs/secret/path") || strings.Contains(body, "boom") {
		t.Fatalf("code=%d body=%q", code, body)
	}
	if !strings.Contains(body, "see server log") {
		t.Fatalf("body=%q", body)
	}
}

func TestHandlerNoDirectoryListing(t *testing.T) {
	srv := newTestServer(t, func(context.Context) error { return nil })
	for _, p := range []string{"/src/", "/src"} {
		if code, body, _ := get(t, srv.URL+p); code != 404 || strings.Contains(body, "main.ts") {
			t.Fatalf("%s: code=%d body=%q", p, code, body)
		}
	}
	if code, body, _ := get(t, srv.URL+"/src/main.ts"); code != 200 || body != "x" {
		t.Fatalf("file: code=%d body=%q", code, body)
	}
}

func TestHandlerNosniff(t *testing.T) {
	srv := newTestServer(t, func(context.Context) error { return nil })
	res, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if got := res.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("nosniff=%q", got)
	}
}

func TestHandlerThrottlesRebuilds(t *testing.T) {
	var mu sync.Mutex
	clock := time.Unix(1000, 0)
	now := func() time.Time { mu.Lock(); defer mu.Unlock(); return clock }
	advance := func(d time.Duration) { mu.Lock(); clock = clock.Add(d); mu.Unlock() }
	builds := 0
	srv := newTestServerClock(t, func(context.Context) error { builds++; return nil }, now)
	get(t, srv.URL+"/")
	advance(time.Second)
	get(t, srv.URL+"/index.html")
	if builds != 1 {
		t.Fatalf("builds=%d within window, want 1", builds)
	}
	advance(1500 * time.Millisecond)
	get(t, srv.URL+"/character.html")
	if builds != 2 {
		t.Fatalf("builds=%d after window, want 2", builds)
	}
}

func TestHandlerFailedBuildNotThrottled(t *testing.T) {
	builds := 0
	clock := time.Unix(1000, 0)
	srv := newTestServerClock(t, func(context.Context) error { builds++; return errors.New("x") },
		func() time.Time { return clock })
	get(t, srv.URL+"/")
	get(t, srv.URL+"/")
	if builds != 2 {
		t.Fatalf("builds=%d, failed builds must not throttle", builds)
	}
}

func TestHandlerNoTraversal(t *testing.T) {
	srv := newTestServer(t, func(context.Context) error { return nil })
	if code, body, _ := get(t, srv.URL+"/../secret"); code == 200 && body == "nope" {
		t.Fatal("served file outside client/")
	}
}
