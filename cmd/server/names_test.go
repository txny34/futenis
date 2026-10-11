package main

import "testing"

func TestCleanName(t *testing.T) {
	for in, want := range map[string]string{
		"Tony":          "Tony",
		"  ":            "",
		"Pendejo":       fallbackName,
		"P3ND3J0":       fallbackName,
		"pendeeejo":     fallbackName,
		"hijo de puta":  fallbackName,
		"la PUTA madre": fallbackName,
		"f.u.c.k":       fallbackName,
		"AssHole99":     fallbackName,
		"Computadora":   "Computadora",
		"Cómputo":       "Cómputo",
		"Vergara":       "Vergara",
		"Maricel":       "Maricel",
		"Controlo todo": "Controlo todo",
		"Trapecio":      "Trapecio",
		"Scunthorpe":    "Scunthorpe",
		"Ñandú":         "Ñandú",
	} {
		if got := cleanName(in); got != want {
			t.Errorf("cleanName(%q) = %q, want %q", in, got, want)
		}
	}
}
