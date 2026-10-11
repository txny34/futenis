// Command server is the online spike's authoritative server
// (docs/design/online-spike.md): WebSocket rooms at /ws?room=CODE running the
// same native sim as the browser.
//
// Usage: go run ./cmd/server [-addr 127.0.0.1:8091] [-origins localhost:8090] [-trust-proxy]
//
// Hosting (Fly.io): PORT overrides -addr (listening on all interfaces),
// FUTENIS_ORIGINS overrides -origins and FUTENIS_TRUST_PROXY=1 sets -trust-proxy.
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

	"github.com/txny34/futenis/cmd/server/internal/auth"
	"github.com/txny34/futenis/cmd/server/internal/ranked"
)

const (
	shutdownTimeout   = 3 * time.Second
	readHeaderTimeout = 5 * time.Second
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8091", "listen address (use :8091 to expose on the LAN)")
	origins := flag.String("origins", "localhost:8090,127.0.0.1:8090", "comma-separated allowed browser origins (host:port)")
	trustProxy := flag.Bool("trust-proxy", false, "client IP from X-Forwarded-For (only behind a proxy that sets it)")
	flag.Parse()
	applyEnv(addr, origins, trustProxy)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	mux := http.NewServeMux()
	h := newHub(splitList(*origins), log.Default())
	h.proxied = *trustProxy
	configureRanked(h, log.Default())
	mux.Handle("/ws", h)
	mux.HandleFunc("/lobby", h.ServeLobby)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
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

// applyEnv lets the hosting platform configure the server without flags.
func applyEnv(addr, origins *string, trustProxy *bool) {
	if p := os.Getenv("PORT"); p != "" {
		*addr = ":" + p
	}
	if o := os.Getenv("FUTENIS_ORIGINS"); o != "" {
		*origins = o
	}
	if os.Getenv("FUTENIS_TRUST_PROXY") == "1" {
		*trustProxy = true
	}
}

// splitList splits a comma-separated flag, trimming spaces and dropping empties.
func splitList(s string) []string {
	var out []string
	for _, v := range strings.Split(s, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// configureRanked enables ranked queues when the Supabase project and its
// secret key are set (Fly secrets SUPABASE_URL / SUPABASE_SECRET_KEY).
func configureRanked(h *hub, logger *log.Logger) {
	project, key := os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_SECRET_KEY")
	if project == "" || key == "" {
		logger.Printf("ranked disabled: SUPABASE_URL / SUPABASE_SECRET_KEY not set")
		return
	}
	base := strings.TrimRight(project, "/")
	h.auth = auth.NewVerifier(base+"/auth/v1/.well-known/jwks.json", base+"/auth/v1")
	h.ratings = ranked.NewSupabase(base, key)
	logger.Printf("ranked enabled (%s)", base)
}
