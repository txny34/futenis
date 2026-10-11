package names

import "testing"

func TestCleanNameCatches(t *testing.T) {
	for _, in := range []string{
		"Pendejo", "P3ND3J0", "pendeeejo", "hijo de puta", "la PUTA madre", "f.u.c.k", "AssHole99",
		"p u t a", "p.u.t.o", "c u l o", "k y s", "ｐｕｔａ", "pùta", "𝐩𝐮𝐭𝐚", "рutа", "nazi", "Hitler88",
		"conchatumadre", "Malparido Pelotudo",
	} {
		if got := Clean(in); got != Fallback {
			t.Errorf("cleanName(%q) = %q, want %q", in, got, Fallback)
		}
	}
}

func TestCleanNameKeepsRealNames(t *testing.T) {
	for _, in := range []string{
		"Tony", "Computadora", "Cómputo", "Vergara", "Maricel", "Controlo todo", "Trapecio", "Scunthorpe",
		"Ñandú", "Ignazio", "Hana Zimmer", "Conchita", "Negro Pérez", "Dickens", "Pussycat Fan", "Αλέξης",
		"Дмитрий", "J. R. Smith",
	} {
		if got := Clean(in); got != in {
			t.Errorf("cleanName(%q) = %q, want it kept", in, got)
		}
	}
	if got := Clean("  "); got != "" {
		t.Errorf("blank name = %q, want empty", got)
	}
}

func TestSanitizeName(t *testing.T) {
	for in, want := range map[string]string{
		"  txny44 ":                  "txny44",
		"a\x00b\nc\u202ed":           "abcd", // control and bidi-override runes dropped
		"ñandú ⚽":                    "ñandú ⚽",
		"abcdefghijklmnopqrstuvwxyz": "abcdefghijklmnop",
		"\xff\xfe":                   "",
	} {
		if got := Sanitize(in); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestSanitizeNameDropsCombiningMarks(t *testing.T) {
	if got := Sanitize("z̶̶̶a⃝"); got != "za" {
		t.Fatalf("got %q: stacked combining marks (zalgo) kept", got)
	}
}
