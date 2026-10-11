package main

import (
	"strings"
	"unicode"
)

// Public names (shared lobby, global leaderboard) go through cleanName: a
// name with a slur or an insult becomes fallbackName. Names are normalized
// first (lowercase, no accents, leetspeak, repeated letters), so "P3ND3J0"
// or "pendeeejo" are caught. Long stems match anywhere (spaced or glued
// names); short words only as whole words, so "computadora" stays clean.
const fallbackName = "Jugador"

var (
	bannedStems = squeezeAll([]string{
		"pendej", "boludo", "pelotud", "concha", "culiad", "culead", "mierda", "chupala", "chupamela",
		"hijodeput", "hdp", "putit", "putaso", "putazo", "sorete", "garcha", "pajero", "cogeme", "violad",
		"nazi", "hitler", "nigger", "nigga", "faggot", "whore", "fuck", "bitch", "asshole", "dickhead",
		"motherf", "pussy", "killyourself",
	})
	bannedWords = toSet(squeezeAll([]string{
		"puta", "puto", "pija", "forro", "orto", "culo", "cagon", "trolo", "verga", "marica", "maricon",
		"mogolico", "mongolico", "idiota", "imbecil", "tarado", "negro", "sudaca", "bolita", "paragua",
		"fag", "dick", "cock", "shit", "slut", "nig", "coon", "spic", "kike", "chink", "tranny", "cunt",
		"retard", "retarded", "rape", "kys",
	}))
	leet = map[rune]rune{'0': 'o', '1': 'i', '3': 'e', '4': 'a', '5': 's', '7': 't', '8': 'b', '@': 'a', '$': 's', '!': 'i'}
	bare = strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n")
)

// cleanName returns the sanitized name ("" stays ""), or fallbackName when it is offensive.
func cleanName(s string) string {
	s = sanitizeName(s)
	if s != "" && offensive(s) {
		return fallbackName
	}
	return s
}

// offensive reports a banned stem anywhere in the glued name or a banned word.
func offensive(s string) bool {
	words := normalizedWords(s)
	glued := strings.Join(words, "")
	for _, stem := range bannedStems {
		if strings.Contains(glued, stem) {
			return true
		}
	}
	for _, w := range words {
		if bannedWords[w] {
			return true
		}
	}
	return false
}

// normalizedWords lowercases, strips accents, undoes leetspeak, squeezes
// repeated letters and splits on anything that is not a letter.
func normalizedWords(s string) []string {
	s = bare.Replace(strings.ToLower(s))
	var words []string
	var cur strings.Builder
	var last rune
	flush := func() {
		if cur.Len() > 0 {
			words = append(words, cur.String())
			cur.Reset()
		}
		last = 0
	}
	for _, r := range s {
		if l, ok := leet[r]; ok {
			r = l
		}
		if !unicode.IsLetter(r) {
			flush()
			continue
		}
		if r != last {
			cur.WriteRune(r)
		}
		last = r
	}
	flush()
	return words
}

// squeezeAll applies the name normalization to a word list (same squeezing of repeated letters).
func squeezeAll(ws []string) []string {
	out := make([]string, len(ws))
	for i, w := range ws {
		out[i] = strings.Join(normalizedWords(w), "")
	}
	return out
}

func toSet(ws []string) map[string]bool {
	m := make(map[string]bool, len(ws))
	for _, w := range ws {
		m[w] = true
	}
	return m
}
