package main

import (
	"crypto/sha256"
	"encoding/base64"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// The pages' CSP <meta> allows their inline import map and <style> by hash:
// editing either block without updating the hash would silently break the page.
var (
	cspMeta     = regexp.MustCompile(`<meta http-equiv="Content-Security-Policy" content="([^"]+)"`)
	inlineBlock = regexp.MustCompile(`(?s)<(script type="importmap"|style)>(.*?)</(?:script|style)>`)
)

func TestPagesCSPHashesMatchInlineBlocks(t *testing.T) {
	pages, err := filepath.Glob(filepath.Join("..", "..", "client", "*.html"))
	if err != nil || len(pages) == 0 {
		t.Fatalf("no pages: %v", err)
	}
	for _, page := range pages {
		b, err := os.ReadFile(page)
		if err != nil {
			t.Fatal(err)
		}
		m := cspMeta.FindSubmatch(b)
		if m == nil {
			t.Errorf("%s: no CSP meta", page)
			continue
		}
		for _, blk := range inlineBlock.FindAllSubmatch(b, -1) {
			sum := sha256.Sum256(blk[2])
			want := "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
			if !regexp.MustCompile(regexp.QuoteMeta(want)).Match(m[1]) {
				t.Errorf("%s: inline <%s> not allowed by the CSP (update its hash to %s)", page, blk[1], want)
			}
		}
	}
}
