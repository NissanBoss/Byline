package main

// Turning the bytes of a PDF string into text.
//
// A string in a dictionary is either UTF-16 with a byte order mark in front
// or PDFDocEncoding, which is Latin-1 with the gaps filled by typographic
// marks. Producers do not always follow that: some write UTF-8 with no mark
// at all, and a name read as PDFDocEncoding then comes out as "JosÃ©", which
// matches nobody.

import (
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// pdfDocHigh is PDFDocEncoding from 0x80 to 0xA0, where it parts from
// Latin-1. Written as numbers, because one of them is a dash this project
// does not write anywhere.
var pdfDocHigh = [...]rune{
	0x2022, 0x2020, 0x2021, 0x2026, 0x2014, 0x2013, 0x0192, 0x2044,
	0x2039, 0x203A, 0x2212, 0x2030, 0x201E, 0x201C, 0x201D, 0x2018,
	0x2019, 0x201A, 0x2122, 0xFB01, 0xFB02, 0x0141, 0x0152, 0x0160,
	0x0178, 0x017D, 0x0131, 0x0142, 0x0153, 0x0161, 0x017E, 0xFFFD,
	0x20AC,
}

// pdfDocLow is 0x18 to 0x1F, the spacing accents.
var pdfDocLow = [...]rune{0x02D8, 0x02C7, 0x02C6, 0x02D9, 0x02DD, 0x02DB, 0x02DA, 0x02DC}

func pdfText(b []byte) string {
	switch {
	case len(b) >= 2 && b[0] == 0xFE && b[1] == 0xFF:
		return utf16Text(b[2:], true)
	case len(b) >= 2 && b[0] == 0xFF && b[1] == 0xFE:
		return utf16Text(b[2:], false)
	case len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF:
		return strings.ToValidUTF8(string(b[3:]), "?")
	}
	if utf8.Valid(b) && !ascii(b) {
		return string(b)
	}
	var out strings.Builder
	for _, c := range b {
		switch {
		case c >= 0x18 && c <= 0x1F:
			out.WriteRune(pdfDocLow[c-0x18])
		case c >= 0x80 && c <= 0xA0:
			out.WriteRune(pdfDocHigh[c-0x80])
		default:
			out.WriteRune(rune(c))
		}
	}
	return out.String()
}

func utf16Text(b []byte, big bool) string {
	units := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		if big {
			units = append(units, uint16(b[i])<<8|uint16(b[i+1]))
		} else {
			units = append(units, uint16(b[i+1])<<8|uint16(b[i]))
		}
	}
	// Some producers put a language mark inside: 0x1B, two letters, 0x1B.
	clean := units[:0:0]
	for i := 0; i < len(units); i++ {
		if units[i] == 0x1B {
			for i++; i < len(units) && units[i] != 0x1B; i++ {
			}
			continue
		}
		clean = append(clean, units[i])
	}
	return string(utf16.Decode(clean))
}

func ascii(b []byte) bool {
	for _, c := range b {
		if c >= 0x80 {
			return false
		}
	}
	return true
}

// textOf is a string value as text, or "" for anything that is not one.
func textOf(v any) string {
	switch s := v.(type) {
	case []byte:
		return strings.TrimRight(pdfText(s), "\x00")
	case Name:
		return string(s)
	}
	return ""
}

// clockOf reads the time zone off a PDF date, D:20241021143000+02'00'.
// Nothing comes back for Z or no zone at all: that is what a server writes,
// and saying a file was made in UTC tells nobody anything.
func clockOf(date string) string {
	date = strings.TrimPrefix(strings.TrimSpace(date), "D:")
	if len(date) < 15 {
		return ""
	}
	zone := date[14:]
	if zone == "" || zone[0] != '+' && zone[0] != '-' {
		return ""
	}
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, zone[1:])
	if len(digits) < 2 {
		return ""
	}
	h, m := digits[:2], "00"
	if len(digits) >= 4 {
		m = digits[2:4]
	}
	if h == "00" && m == "00" {
		return ""
	}
	return "UTC" + zone[:1] + h + ":" + m
}
