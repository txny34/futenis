package main

import "testing"

func TestCleanNameCatches(t *testing.T) {
	for _, in := range []string{
		"Pendejo", "P3ND3J0", "pendeeejo", "hijo de puta", "la PUTA madre", "f.u.c.k", "AssHole99",
		"p u t a", "p.u.t.o", "c u l o", "k y s", "ｐｕｔａ", "pùta", "𝐩𝐮𝐭𝐚", "рutа", "nazi", "Hitler88",
		"conchatumadre", "Malparido Pelotudo",
	} {
		if got := cleanName(in); got != fallbackName {
			t.Errorf("cleanName(%q) = %q, want %q", in, got, fallbackName)
		}
	}
}

func TestCleanNameKeepsRealNames(t *testing.T) {
	for _, in := range []string{
		"Tony", "Computadora", "Cómputo", "Vergara", "Maricel", "Controlo todo", "Trapecio", "Scunthorpe",
		"Ñandú", "Ignazio", "Hana Zimmer", "Conchita", "Negro Pérez", "Dickens", "Pussycat Fan", "Αλέξης",
		"Дмитрий", "J. R. Smith",
	} {
		if got := cleanName(in); got != in {
			t.Errorf("cleanName(%q) = %q, want it kept", in, got)
		}
	}
	if got := cleanName("  "); got != "" {
		t.Errorf("blank name = %q, want empty", got)
	}
}
