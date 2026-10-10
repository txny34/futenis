package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/evanw/esbuild/pkg/api"
)

// builder produces client/dist from wasm/ and client/src. Build calls are
// serialized so concurrent requests never run two builds at once.
type builder struct {
	root string
	mu   sync.Mutex
}

func newBuilder(root string) *builder { return &builder{root: root} }

func (b *builder) dist(name string) string {
	return filepath.Join(b.root, "client", "dist", name)
}

// Build runs every build step in order and returns the first error.
func (b *builder) Build(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := os.MkdirAll(b.dist(""), 0o755); err != nil {
		return fmt.Errorf("mkdir dist: %w", err)
	}
	if err := b.copyWasmExec(ctx); err != nil {
		return fmt.Errorf("wasm_exec.js: %w", err)
	}
	if err := b.buildWasm(ctx); err != nil {
		return fmt.Errorf("futenis.wasm: %w", err)
	}
	if err := b.bundleTS(); err != nil {
		return fmt.Errorf("bundle main.js/preview.js: %w", err)
	}
	return nil
}

// wasmExecCandidates lists where wasm_exec.js may live, newest Go layout first.
func wasmExecCandidates(goroot string) []string {
	return []string{
		filepath.Join(goroot, "lib", "wasm", "wasm_exec.js"),
		filepath.Join(goroot, "misc", "wasm", "wasm_exec.js"),
	}
}

// findFirst returns the first path for which exists reports true.
func findFirst(paths []string, exists func(string) bool) (string, error) {
	for _, p := range paths {
		if exists(p) {
			return p, nil
		}
	}
	return "", fmt.Errorf("none of %v exist", paths)
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func (b *builder) copyWasmExec(ctx context.Context) error {
	out, err := exec.CommandContext(ctx, "go", "env", "GOROOT").Output()
	if err != nil {
		return fmt.Errorf("go env GOROOT: %w", err)
	}
	src, err := findFirst(wasmExecCandidates(strings.TrimSpace(string(out))), fileExists)
	if err != nil {
		return err
	}
	return copyFile(src, b.dist("wasm_exec.js"))
}

func copyFile(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close() // read-only: close error carries no data-loss risk
	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	defer func() {
		if cerr := out.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close %s: %w", dst, cerr)
		}
	}()
	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy to %s: %w", dst, err)
	}
	return nil
}

func (b *builder) buildWasm(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "go", "build", "-o", b.dist("futenis.wasm"), "./wasm")
	cmd.Dir = b.root
	cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	var stderr strings.Builder
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build: %w\n%s", err, stderr.String())
	}
	return nil
}

func (b *builder) bundleTS() error {
	server := os.Getenv("FUTENIS_SERVER")
	sourcemap := api.SourceMapInline
	if server != "" {
		sourcemap = api.SourceMapNone // deployed build: do not ship the TS sources
	}
	result := api.Build(api.BuildOptions{
		EntryPointsAdvanced: []api.EntryPoint{
			{InputPath: filepath.Join(b.root, "client", "src", "main.ts"), OutputPath: "main"},
			{InputPath: filepath.Join(b.root, "client", "src", "character", "preview.ts"), OutputPath: "preview"},
			{InputPath: filepath.Join(b.root, "client", "src", "online.ts"), OutputPath: "online"},
		},
		Outdir:    b.dist(""),
		Bundle:    true,
		Write:     true,
		Format:    api.FormatESModule,
		Target:    api.ES2020,
		Sourcemap: sourcemap,
		External:  []string{"three", "three/*"}, // provided by the import map
		// Online server base URL for deployed builds (e.g. wss://futenis-server.fly.dev); empty = same host :8091.
		Define:   map[string]string{"__FUTENIS_SERVER__": strconv.Quote(server)},
		LogLevel: api.LogLevelSilent,
	})
	if len(result.Errors) == 0 {
		return nil
	}
	msgs := api.FormatMessages(result.Errors, api.FormatMessagesOptions{Kind: api.ErrorMessage})
	return errors.New(strings.Join(msgs, "\n"))
}
