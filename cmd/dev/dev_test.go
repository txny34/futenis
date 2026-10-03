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
	"testing"
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
	root := t.TempDir()
	client := filepath.Join(root, "client")
	if err := os.MkdirAll(client, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(client, "index.html"), []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "secret"), []byte("nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(newHandler(root, build, log.New(io.Discard, "", 0)))
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

func TestHandlerBuildFailureIs500(t *testing.T) {
	srv := newTestServer(t, func(context.Context) error { return errors.New("boom") })
	if code, _, _ := get(t, srv.URL+"/index.html"); code != 500 {
		t.Fatalf("code=%d", code)
	}
}

func TestHandlerNoTraversal(t *testing.T) {
	srv := newTestServer(t, func(context.Context) error { return nil })
	if code, body, _ := get(t, srv.URL+"/../secret"); code == 200 && body == "nope" {
		t.Fatal("served file outside client/")
	}
}
