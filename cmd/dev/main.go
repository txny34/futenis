// Command dev is the futenis development server. It builds the WASM sim and
// bundles the TypeScript client with esbuild's Go API (no Node), then serves
// client/ over HTTP, rebuilding whenever the page is reloaded.
//
// Usage: go run ./cmd/dev [-addr :8090] [-root .]
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

func main() {
	// All interfaces: WSL2 NAT does not forward loopback-only binds to Windows.
	addr := flag.String("addr", ":8090", "listen address")
	root := flag.String("root", ".", "repo root")
	flag.Parse()

	absRoot, err := filepath.Abs(*root)
	if err != nil {
		log.Fatalf("resolve root: %v", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	b := newBuilder(absRoot)
	if err := b.Build(ctx); err != nil {
		log.Printf("initial build failed:\n%v", err)
	} else {
		log.Printf("initial build ok")
	}

	log.Printf("futenis dev server on http://%s (root %s)", *addr, absRoot)
	if err := serve(ctx, *addr, absRoot, b, log.Default()); err != nil {
		log.Fatalf("server: %v", err)
	}
	log.Printf("server stopped")
}
