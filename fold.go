package main

// Accents, both ways.
//
// A name has to match whether the file spells it José, Jose or, in a paper
// set with TeX's old fonts, as an accent drawn next to a letter. So names are
// compared with their accents taken off. The same table puts accents back on
// when a font names its glyphs eacute and udieresis, which is how the text of
// half the papers on arXiv is stored.

import (
	"strings"
	"unicode"
)

// accented lists, for each accent, the letters it goes on and what they
// become with it.
var accented = []struct{ accent, bare, with string }{
	{"acute", "AaEeIiOoUuYyCcNnSsZzLlRr", "ÁáÉéÍíÓóÚúÝýĆćŃńŚśŹźĹĺŔŕ"},
	{"grave", "AaEeIiOoUu", "ÀàÈèÌìÒòÙù"},
	{"circumflex", "AaEeIiOoUuCcGgHhJjSsWwYy", "ÂâÊêÎîÔôÛûĈĉĜĝĤĥĴĵŜŝŴŵŶŷ"},
	{"dieresis", "AaEeIiOoUuYy", "ÄäËëÏïÖöÜüŸÿ"},
	{"tilde", "AaOoNnIiUu", "ÃãÕõÑñĨĩŨũ"},
	{"ring", "AaUu", "ÅåŮů"},
	{"cedilla", "CcSsTtGgKkLlNnRr", "ÇçŞşŢţĢģĶķĻļŅņŖŗ"},
	{"commaaccent", "SsTtGgKkLlNnRr", "ȘșȚțĢģĶķĻļŅņŖŗ"},
	{"caron", "SsZzCcEeRrNnDdTtLl", "ŠšŽžČčĚěŘřŇňĎďŤťĽľ"},
	{"ogonek", "AaEeIiUu", "ĄąĘęĮįŲų"},
	{"dotaccent", "ZzEeGgCcI", "ŻżĖėĠġĊċİ"},
	{"hungarumlaut", "OoUu", "ŐőŰű"},
	{"breve", "GgAaUu", "ĞğĂăŬŭ"},
	{"macron", "AaEeIiOoUu", "ĀāĒēĪīŌōŪū"},
	{"slash", "Oo", "Øø"},
}

// letters are the ones that are not a letter with an accent, and fold to
// more than one.
var letters = map[rune]string{
	'ß': "ss", 'æ': "ae", 'Æ': "ae", 'œ': "oe", 'Œ': "oe", 'þ': "th", 'Þ': "th",
	'ð': "d", 'Ð': "d", 'đ': "d", 'Đ': "d", 'ł': "l", 'Ł': "l", 'ı': "i", 'ħ': "h", 'Ħ': "h",
	0xFB00: "ff", 0xFB01: "fi", 0xFB02: "fl", 0xFB03: "ffi", 0xFB04: "ffl", 0xFB05: "st", 0xFB06: "st",
}

var bareOf = map[rune]rune{}
var withAccent = map[string]rune{}

func init() {
	for _, a := range accented {
		bare, with := []rune(a.bare), []rune(a.with)
		if len(bare) != len(with) {
			panic("fold table for " + a.accent + " does not line up")
		}
		for i := range bare {
			bareOf[with[i]] = bare[i]
			withAccent[string(bare[i])+a.accent] = with[i]
		}
	}
}

// fold is a rune as it is compared: lower case, accent off.
func fold(r rune) string {
	if s, ok := letters[r]; ok {
		return s
	}
	if b, ok := bareOf[r]; ok {
		r = b
	}
	return string(unicode.ToLower(r))
}

// folded is a string with every rune folded and everything that is not a
// letter or a digit turned into one space. back says, for each byte of the
// folded string, which rune of the original it came from, so a match can be
// shown in the words of the file.
type folded struct {
	text  string
	back  []int
	runes []rune
}

func foldText(s string) folded { return foldWith(s, false) }

// foldName folds the way a person's name is compared, with digits taken
// for gaps: a paper puts each author's affiliation number against the
// name, Vardi1 and Doe1, Jane2, and the name has to be found all the same.
// Logins and addresses keep their digits, since jdoe4 is not jdoe.
func foldName(s string) folded { return foldWith(s, true) }

func foldWith(s string, loose bool) folded {
	runes := []rune(s)
	var b strings.Builder
	back := make([]int, 0, len(s)+2)
	b.WriteByte(' ')
	back = append(back, 0)
	space := true
	for i, r := range runes {
		if unicode.IsLetter(r) || unicode.IsDigit(r) && !loose {
			f := fold(r)
			b.WriteString(f)
			for range len(f) {
				back = append(back, i)
			}
			space = false
			continue
		}
		// An accent on its own, the way TeX's old fonts draw one beside
		// its letter, is not a gap between two words.
		if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Sk, r) {
			continue
		}
		if !space {
			b.WriteByte(' ')
			back = append(back, i)
			space = true
		}
	}
	if !space {
		b.WriteByte(' ')
		back = append(back, len(runes))
	}
	return folded{text: b.String(), back: back, runes: runes}
}

// original is the stretch of the source a folded match came from.
func (f folded) original(start, end int) string {
	if start < 0 || end > len(f.back) || start >= end {
		return ""
	}
	from := f.back[start]
	to := len(f.runes)
	if end < len(f.back) {
		to = f.back[end]
	}
	for from < to && from < len(f.runes) && !unicode.IsLetter(f.runes[from]) && !unicode.IsDigit(f.runes[from]) {
		from++
	}
	if to > len(f.runes) {
		to = len(f.runes)
	}
	return strings.Join(strings.Fields(string(f.runes[from:to])), " ")
}
