// Command server is the online spike's authoritative server
// (docs/design/online-spike.md): WebSocket rooms at /ws?room=CODE running the
// same native sim as the browser.
//
// Usage: go run ./cmd/server [-addr 127.0.0.1:8091] [-origins localhost:8090]
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"
)

const (
	shutdownTimeout   = 3 * time.Second
	readHeaderTimeout = 5 * time.Second
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8091", "listen address (use :8091 to expose on the LAN)")
	origins := flag.String("origins", "localhost:8090,127.0.0.1:8090", "comma-separated allowed browser origins (host:port)")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	mux := http.NewServeMux()
	mux.Handle("/ws", newHub(strings.Split(*origins, ","), log.Default()))
	srv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: readHeaderTimeout}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		_ = srv.Shutdown(sctx)
	}()
	log.Printf("futenis online server on ws://%s/ws", *addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server: %v", err)
	}
}
