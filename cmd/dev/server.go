package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"path"
	"path/filepath"
	"sync"
	"time"
)

const (
	readHeaderTimeout = 5 * time.Second
	shutdownTimeout   = 5 * time.Second
	// rebuildInterval: page loads within this window of the last successful
	// build start reuse it, so a LAN client cannot flood `go build`.
	rebuildInterval = 2 * time.Second
)

// builderFunc is what the handler needs from a builder (eases testing).
type builderFunc func(ctx context.Context) error

// newHandler serves only root/client via http.FileServer (no manual path
// joining from request input, no directory listings) and rebuilds on page
// loads, throttled by rebuildInterval using now as the clock.
func newHandler(root string, build builderFunc, logger *log.Logger, now func() time.Time) http.Handler {
	files := http.FileServer(noListingFS{http.Dir(filepath.Join(root, "client"))})
	rebuild := throttle(build, now)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.URL.Path == "/" || r.URL.Path == "/index.html" || r.URL.Path == "/character.html" {
			if err := rebuild(r.Context()); err != nil {
				logger.Printf("build failed:\n%v", err)
				http.Error(w, "build failed — see server log", http.StatusInternalServerError)
				return
			}
		}
		files.ServeHTTP(w, r)
	})
}

// throttle skips build when the last successful build started less than
// rebuildInterval ago. Failed builds never throttle.
func throttle(build builderFunc, now func() time.Time) builderFunc {
	var (
		mu      sync.Mutex
		lastOK  time.Time
		hasLast bool
	)
	return func(ctx context.Context) error {
		mu.Lock()
		defer mu.Unlock()
		start := now()
		if hasLast && start.Sub(lastOK) < rebuildInterval {
			return nil
		}
		if err := build(ctx); err != nil {
			return err
		}
		lastOK, hasLast = start, true
		return nil
	}
}

// noListingFS hides directories that have no index.html, so the file server
// answers 404 instead of a listing.
type noListingFS struct{ fs http.FileSystem }

func (n noListingFS) Open(name string) (http.File, error) {
	f, err := n.fs.Open(name)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if !st.IsDir() {
		return f, nil
	}
	idx, err := n.fs.Open(path.Join(name, "index.html"))
	if err != nil {
		_ = f.Close()
		return nil, fs.ErrNotExist
	}
	_ = idx.Close()
	return f, nil
}

// serve runs the HTTP server until ctx is cancelled, then shuts down gracefully.
func serve(ctx context.Context, addr, root string, b *builder, logger *log.Logger) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           newHandler(root, b.Build, logger, time.Now),
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
