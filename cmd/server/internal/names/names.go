package names

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Public names (shared lobby, global leaderboard) go through Clean: a
// name with a slur or an insult becomes Fallback. Names are normalized
// first (lowercase, fullwidth and accents folded, leetspeak, repeated letters
// squeezed), so "P3ND3J0", "ｐｕｔａ" or "pendeeejo" are caught. Stems match
// inside a word; across words only long stems (≥ gluedStemMin letters) and
// spelled-out names ("p u t a") are checked, so "Ignazio" or "Hana Zimmer"
// stay clean. Mixed Latin/Cyrillic/Greek and math-styled letters are refused
// outright: they exist to dodge filters.
const (
	// MaxRunes caps a public name (lobby tag, leaderboard).
	MaxRunes      = 16
	Fallback      = "Jugador"
	gluedStemMin  = 6
	spelledOutMax = 2 // letters per word for a name read as spelled out
)

var (
	bannedStems = squeezeAll([]string{
		"pendej", "boludo", "pelotud", "culiad", "culead", "mierda", "chupala", "chupamela", "hijodeput",
		"conchatumadre", "conchadetumadre", "laconchadetu", "putit", "putaso", "putazo", "sorete", "garcha",
		"pajero", "cogeme", "violad", "hitler", "nigger", "nigga", "faggot", "whore", "fuck", "bitch",
		"asshole", "dickhead", "motherf", "killyourself",
	})
	bannedWords = toSet(squeezeAll([]string{
		"puta", "puto", "putos", "putas", "pija", "forro", "orto", "culo", "cagon", "trolo", "verga", "marica",
		"maricon", "mogolico", "mongolico", "idiota", "imbecil", "tarado", "sudaca", "hdp", "nazi", "nazis",
		"fag", "dick", "cock", "shit", "slut", "nig", "coon", "spic", "kike", "chink", "tranny", "cunt",
		"retard", "retarded", "rape", "kys", "pussy",
	}))
	leet = map[rune]rune{'0': 'o', '1': 'i', '|': 'i', '3': 'e', '4': 'a', '5': 's', '7': 't', '8': 'b', '@': 'a', '$': 's', '!': 'i'}
	bare = strings.NewReplacer(
		"á", "a", "à", "a", "â", "a", "ä", "a", "ã", "a", "å", "a", "é", "e", "è", "e", "ê", "e", "ë", "e",
		"í", "i", "ì", "i", "î", "i", "ï", "i", "ó", "o", "ò", "o", "ô", "o", "ö", "o", "õ", "o",
		"ú", "u", "ù", "u", "û", "u", "ü", "u", "ñ", "n", "ç", "c", "ý", "y", "ÿ", "y",
	)
)

// Clean returns the sanitized name ("" stays ""), or Fallback when it is offensive.
// The full input is checked too: cutting to the length cap must not hide a word.
func Clean(raw string) string {
	s := Sanitize(raw)
	if s != "" && (evasive(raw) || offensive(raw) || offensive(s)) {
		return Fallback
	}
	return s
}

// evasive: math-styled letters, or Latin mixed with Cyrillic/Greek look-alikes.
func evasive(s string) bool {
	latin, other := false, false
	for _, r := range s {
		switch {
		case r >= 0x1D400 && r <= 0x1D7FF: // Mathematical Alphanumeric Symbols
			return true
		case unicode.In(r, unicode.Cyrillic, unicode.Greek):
			other = true
		case unicode.Is(unicode.Latin, foldWidth(r)):
			latin = true
		}
	}
	return latin && other
}

// offensive: a banned word, a stem inside a word, a long stem across words,
// or a spelled-out banned word.
func offensive(s string) bool {
	words := normalizedWords(s)
	spelled := len(words) > 1
	for _, w := range words {
		if bannedWords[w] || containsAny(w, bannedStems, 0) {
			return true
		}
		spelled = spelled && len([]rune(w)) <= spelledOutMax
	}
	glued := strings.Join(words, "")
	return containsAny(glued, bannedStems, gluedStemMin) || (spelled && (bannedWords[glued] || containsAny(glued, bannedStems, 0)))
}

func containsAny(s string, stems []string, minLen int) bool {
	for _, st := range stems {
		if len(st) >= minLen && strings.Contains(s, st) {
			return true
		}
	}
	return false
}

// foldWidth maps fullwidth ASCII (U+FF01..U+FF5E) to ASCII.
func foldWidth(r rune) rune {
	if r >= 0xFF01 && r <= 0xFF5E {
		return r - 0xFEE0
	}
	return r
}

// normalizedWords lowercases, folds width and accents, undoes leetspeak,
// squeezes repeated letters and splits on anything that is not a letter.
func normalizedWords(s string) []string {
	var folded strings.Builder
	for _, r := range s {
		folded.WriteRune(foldWidth(r))
	}
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
	for _, r := range bare.Replace(strings.ToLower(folded.String())) {
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

// Sanitize keeps printable runes only (no control, format/bidi overrides
// or combining marks, so no "zalgo" stacks), trims spaces and caps the length.
// Invalid UTF-8 is dropped.
func Sanitize(s string) string {
	var b strings.Builder
	n := 0
	for _, r := range strings.TrimSpace(s) {
		if r == utf8.RuneError || !unicode.IsPrint(r) || unicode.In(r, unicode.Cf, unicode.Mn, unicode.Me) {
			continue
		}
		if n == MaxRunes {
			break
		}
		b.WriteRune(r)
		n++
	}
	return strings.TrimSpace(b.String())
}
