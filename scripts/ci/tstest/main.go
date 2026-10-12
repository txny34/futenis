// Command tstest runs the client's TypeScript unit tests (client/test/*.test.ts)
// without adding Node to the game build: each test is bundled with the esbuild
// Go API (three from client/vendor) and run with `node --test` (CI only, like tsc).
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/evanw/esbuild/pkg/api"
)

// prelude: the browser globals the client reads at import time.
const prelude = `globalThis.window ??= globalThis;`

func main() {
	tests, err := filepath.Glob("client/test/*.test.ts")
	if err != nil || len(tests) == 0 {
		fail("no tests in client/test (run from the repo root)")
	}
	out, err := os.MkdirTemp("", "futenis-tstest")
	if err != nil {
		fail(err.Error())
	}
	defer os.RemoveAll(out)
	res := api.Build(api.BuildOptions{
		EntryPoints: tests, Bundle: true, Outdir: out, Platform: api.PlatformNode, Format: api.FormatESModule,
		OutExtension: map[string]string{".js": ".mjs"}, Write: true, LogLevel: api.LogLevelWarning,
		Alias:  map[string]string{"three": "./client/vendor/three/build/three.module.js"},
		Define: map[string]string{"__FUTENIS_SERVER__": `""`},
		Banner: map[string]string{"js": prelude},
	})
	if len(res.Errors) > 0 {
		fail(fmt.Sprintf("%d bundle errors", len(res.Errors)))
	}
	files, _ := filepath.Glob(filepath.Join(out, "*.mjs"))
	cmd := exec.Command("node", append([]string{"--test"}, files...)...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "tstest:", msg)
	os.Exit(1)
}
