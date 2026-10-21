package main

// Glyph names and the base encodings, which is how a simple font says what
// letter each of its codes draws.
//
// The full Adobe glyph list is four thousand names. What is here is what
// shows up in papers: letters, digits, punctuation, the accented letters of
// European names, ligatures, and the names TeX's fonts give their accents.
// A name not known comes out as nothing, and a font with too many of those
// is reported as unread rather than searched and called clean.

import (
	"strconv"
	"strings"
)

var glyphNames = map[string]rune{
	"space": ' ', "exclam": '!', "quotedbl": '"', "numbersign": '#', "dollar": '$', "percent": '%',
	"ampersand": '&', "quotesingle": '\'', "quoteright": 0x2019, "parenleft": '(', "parenright": ')',
	"asterisk": '*', "plus": '+', "comma": ',', "hyphen": '-', "minus": 0x2212, "period": '.', "slash": '/',
	"zero": '0', "one": '1', "two": '2', "three": '3', "four": '4', "five": '5', "six": '6', "seven": '7',
	"eight": '8', "nine": '9', "colon": ':', "semicolon": ';', "less": '<', "equal": '=', "greater": '>',
	"question": '?', "at": '@', "bracketleft": '[', "backslash": '\\', "bracketright": ']',
	"asciicircum": '^', "underscore": '_', "grave": 0x60, "quoteleft": 0x2018, "braceleft": '{', "bar": '|',
	"braceright": '}', "asciitilde": '~', "exclamdown": 0xA1, "cent": 0xA2, "sterling": 0xA3,
	"fraction": 0x2044, "yen": 0xA5, "florin": 0x192, "section": 0xA7, "currency": 0xA4,
	"quotedblleft": 0x201C, "guillemotleft": 0xAB, "guilsinglleft": 0x2039, "guilsinglright": 0x203A,
	"endash": 0x2013, "emdash": 0x2014, "dagger": 0x2020, "daggerdbl": 0x2021, "periodcentered": 0xB7,
	"paragraph": 0xB6, "bullet": 0x2022, "quotesinglbase": 0x201A, "quotedblbase": 0x201E,
	"quotedblright": 0x201D, "guillemotright": 0xBB, "ellipsis": 0x2026, "perthousand": 0x2030,
	"questiondown": 0xBF, "germandbls": 0xDF, "ae": 0xE6, "AE": 0xC6, "oe": 0x153, "OE": 0x152,
	"oslash": 0xF8, "Oslash": 0xD8, "lslash": 0x142, "Lslash": 0x141, "dotlessi": 0x131, "dotlessj": 0x237,
	"thorn": 0xFE, "Thorn": 0xDE, "eth": 0xF0, "Eth": 0xD0, "dcroat": 0x111, "Dcroat": 0x110,
	"ordfeminine": 0xAA, "ordmasculine": 0xBA, "copyright": 0xA9, "registered": 0xAE, "trademark": 0x2122,
	"degree": 0xB0, "plusminus": 0xB1, "multiply": 0xD7, "divide": 0xF7, "mu": 0xB5, "logicalnot": 0xAC,
	"brokenbar": 0xA6, "onehalf": 0xBD, "onequarter": 0xBC, "threequarters": 0xBE, "onesuperior": 0xB9,
	"twosuperior": 0xB2, "threesuperior": 0xB3, "nbspace": 0xA0, "nonbreakingspace": 0xA0, "sfthyphen": 0xAD,
	"Euro": 0x20AC, "euro": 0x20AC, "visiblespace": 0x2423, "compwordmark": 0x200C, "cwm": 0x200C,
	"ff": 0xFB00, "fi": 0xFB01, "fl": 0xFB02, "ffi": 0xFB03, "ffl": 0xFB04,
	"quotedblleftbase": 0x201E, "angbracketleft": 0x2329, "angbracketright": 0x232A,
	"Alpha": 0x391, "Beta": 0x392, "Gamma": 0x393, "Delta": 0x394, "Theta": 0x398, "Lambda": 0x39B,
	"Xi": 0x39E, "Pi": 0x3A0, "Sigma": 0x3A3, "Upsilon": 0x3A5, "Phi": 0x3A6, "Psi": 0x3A8, "Omega": 0x3A9,
	"alpha": 0x3B1, "beta": 0x3B2, "gamma": 0x3B3, "delta": 0x3B4, "epsilon": 0x3B5, "zeta": 0x3B6,
	"eta": 0x3B7, "theta": 0x3B8, "iota": 0x3B9, "kappa": 0x3BA, "lambda": 0x3BB, "nu": 0x3BD, "xi": 0x3BE,
	"pi": 0x3C0, "rho": 0x3C1, "sigma": 0x3C3, "tau": 0x3C4, "upsilon": 0x3C5, "phi": 0x3C6, "chi": 0x3C7,
	"psi": 0x3C8, "omega": 0x3C9,
}

// accents are the glyphs that only put an accent on a letter drawn beside
// them. In text they are dropped: TeX's old fonts set é as an acute and
// an e, and the name has to come out as one word.
var accents = map[string]bool{
	"acute": true, "grave": true, "circumflex": true, "dieresis": true, "tilde": true, "ring": true,
	"cedilla": true, "caron": true, "breve": true, "macron": true, "ogonek": true, "dotaccent": true,
	"hungarumlaut": true, "commaaccent": true,
	// The stroke TeX draws through an l to make it an ł.
	"suppress": true,
}

// glyphRune reads a glyph name: one from the list, a letter with an accent
// spelled out (eacute, Scaron), or the uniXXXX and uXXXX forms that say
// the code point in hex. The suffixes fonts add for variants (a.sc,
// one.oldstyle, f_i) are taken off first.
func glyphRune(name string) (string, bool) {
	if accents[name] {
		return "", true
	}
	if i := strings.IndexByte(name, '.'); i > 0 {
		name = name[:i]
	}
	if strings.Contains(name, "_") {
		var out strings.Builder
		for _, part := range strings.Split(name, "_") {
			s, ok := glyphRune(part)
			if !ok {
				return "", false
			}
			out.WriteString(s)
		}
		return out.String(), true
	}
	if r, ok := glyphNames[name]; ok {
		return string(r), true
	}
	if len(name) == 1 {
		c := name[0]
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' {
			return name, true
		}
	}
	for _, a := range accented {
		if base, ok := strings.CutSuffix(name, a.accent); ok && len(base) == 1 {
			if r, ok := withAccent[base+a.accent]; ok {
				return string(r), true
			}
		}
	}
	if hex, ok := strings.CutPrefix(name, "uni"); ok && len(hex) >= 4 && len(hex)%4 == 0 {
		var out []rune
		for i := 0; i+4 <= len(hex); i += 4 {
			v, err := strconv.ParseUint(hex[i:i+4], 16, 32)
			if err != nil {
				return "", false
			}
			out = append(out, rune(v))
		}
		return string(out), true
	}
	if hex, ok := strings.CutPrefix(name, "u"); ok && len(hex) >= 4 && len(hex) <= 6 {
		if v, err := strconv.ParseUint(hex, 16, 32); err == nil {
			return string(rune(v)), true
		}
	}
	return "", false
}

// winAnsiHigh is code page 1252 from 0x80 to 0x9F; above that it is
// Latin-1, and below it is ASCII.
var winAnsiHigh = [32]rune{
	0x20AC, 0, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021, 0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0, 0x017D, 0,
	0, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014, 0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0, 0x017E, 0x0178,
}

// macRomanHigh is Mac OS Roman from 0x80 up.
var macRomanHigh = [128]rune{
	0xC4, 0xC5, 0xC7, 0xC9, 0xD1, 0xD6, 0xDC, 0xE1, 0xE0, 0xE2, 0xE4, 0xE3, 0xE5, 0xE7, 0xE9, 0xE8,
	0xEA, 0xEB, 0xED, 0xEC, 0xEE, 0xEF, 0xF1, 0xF3, 0xF2, 0xF4, 0xF6, 0xF5, 0xFA, 0xF9, 0xFB, 0xFC,
	0x2020, 0xB0, 0xA2, 0xA3, 0xA7, 0x2022, 0xB6, 0xDF, 0xAE, 0xA9, 0x2122, 0xB4, 0xA8, 0x2260, 0xC6, 0xD8,
	0x221E, 0xB1, 0x2264, 0x2265, 0xA5, 0xB5, 0x2202, 0x2211, 0x220F, 0x3C0, 0x222B, 0xAA, 0xBA, 0x3A9, 0xE6, 0xF8,
	0xBF, 0xA1, 0xAC, 0x221A, 0x192, 0x2248, 0x2206, 0xAB, 0xBB, 0x2026, 0xA0, 0xC0, 0xC3, 0xD5, 0x152, 0x153,
	0x2013, 0x2014, 0x201C, 0x201D, 0x2018, 0x2019, 0xF7, 0x25CA, 0xFF, 0x178, 0x2044, 0x20AC, 0x2039, 0x203A, 0xFB01, 0xFB02,
	0x2021, 0xB7, 0x201A, 0x201E, 0x2030, 0xC2, 0xCA, 0xC1, 0xCB, 0xC8, 0xCD, 0xCE, 0xCF, 0xCC, 0xD3, 0xD4,
	0xF8FF, 0xD2, 0xDA, 0xDB, 0xD9, 0x131, 0x2C6, 0x2DC, 0xAF, 0x2D8, 0x2D9, 0x2DA, 0xB8, 0x2DD, 0x2DB, 0x2C7,
}

// standardHigh is Adobe's StandardEncoding above 0xA0, where it is not
// Latin-1 at all. Codes not listed draw nothing.
var standardHigh = map[byte]rune{
	0xA1: 0xA1, 0xA2: 0xA2, 0xA3: 0xA3, 0xA4: 0x2044, 0xA5: 0xA5, 0xA6: 0x192, 0xA7: 0xA7, 0xA8: 0xA4,
	0xA9: '\'', 0xAA: 0x201C, 0xAB: 0xAB, 0xAC: 0x2039, 0xAD: 0x203A, 0xAE: 0xFB01, 0xAF: 0xFB02,
	0xB1: 0x2013, 0xB2: 0x2020, 0xB3: 0x2021, 0xB4: 0xB7, 0xB6: 0xB6, 0xB7: 0x2022, 0xB8: 0x201A,
	0xB9: 0x201E, 0xBA: 0x201D, 0xBB: 0xBB, 0xBC: 0x2026, 0xBD: 0x2030, 0xBF: 0xBF, 0xD0: 0x2014,
	0xE1: 0xC6, 0xE3: 0xAA, 0xE8: 0x141, 0xE9: 0xD8, 0xEA: 0x152, 0xEB: 0xBA, 0xF1: 0xE6, 0xF5: 0x131,
	0xF8: 0x142, 0xF9: 0xF8, 0xFA: 0x153, 0xFB: 0xDF,
}

// baseEncoding gives the 256 strings a named encoding maps codes to.
func baseEncoding(name Name) [256]string {
	var out [256]string
	for c := 0x20; c < 0x7F; c++ {
		out[c] = string(rune(c))
	}
	switch name {
	case "WinAnsiEncoding":
		for c := 0x80; c < 0xA0; c++ {
			if r := winAnsiHigh[c-0x80]; r != 0 {
				out[c] = string(r)
			}
		}
		for c := 0xA0; c < 0x100; c++ {
			out[c] = string(rune(c))
		}
	case "MacRomanEncoding":
		for c := 0x80; c < 0x100; c++ {
			out[c] = string(macRomanHigh[c-0x80])
		}
	default:
		// StandardEncoding, which is also what a Type 1 font with no
		// encoding of its own falls back on.
		out[0x27] = string(rune(0x2019))
		out[0x60] = string(rune(0x2018))
		for c, r := range standardHigh {
			out[c] = string(r)
		}
	}
	return out
}
