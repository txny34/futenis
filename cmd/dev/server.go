package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"time"
)

const (
	readHeaderTimeout = 5 * time.Second
	shutdownTimeout   = 5 * time.Second
)

// builderFunc is what the handler needs from a builder (eases testing).
type builderFunc func(ctx context.Context) error

// newHandler serves only root/client via http.FileServer (no manual path
// joining from request input) and rebuilds on page loads.
func newHandler(root string, build builderFunc, logger *log.Logger) http.Handler {
	files := http.FileServer(http.Dir(filepath.Join(root, "client")))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			if err := build(r.Context()); err != nil {
				logger.Printf("build failed:\n%v", err)
				http.Error(w, "build failed:\n"+err.Error(), http.StatusInternalServerError)
				return
			}
		}
		files.ServeHTTP(w, r)
	})
}

// serve runs the HTTP server until ctx is cancelled, then shuts down gracefully.
func serve(ctx context.Context, addr, root string, b *builder, logger *log.Logger) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           newHandler(root, b.Build, logger),
		ReadHeaderTimeout: readHeaderTimeout,
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()

	select {
	case err := <-errc:
		return fmt.Errorf("listen: %w", err)
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-errc; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("listen: %w", err)
	}
	return nil
}
