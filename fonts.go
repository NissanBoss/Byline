package main

// Fonts, as far as knowing which letter each code in a string draws.
//
// The text of a PDF page is not text. It is a run of codes, and the font
// says what each code looks like. Which letter it is comes, in order of how
// much to trust it, from the font's ToUnicode table, from the encoding the
// font names with its differences, or from the encoding inside the font
// program itself, which is where TeX's Computer Modern keeps its own. A
// font that has none of these draws shapes, and its words cannot be read;
// that is said in the report, because a name in a font that could not be
// read has not been looked for.

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"
)

type codeRange struct {
	lo, hi uint32
	n      int // bytes
}

type font struct {
	name string
	// cid is a composite font, whose codes are usually two bytes.
	cid    bool
	ranges []codeRange
	uni    map[uint32]string // from ToUnicode
	simple [256]string       // from the encoding, for a simple font
	known  bool              // simple holds something
	widths map[uint32]float64
	dw     float64
	// unread says why the font's letters cannot be known at all.
	unread string
	// used and unknown count codes drawn and codes no table names, so a
	// font that turns out to be pictures of letters is noticed.
	used, unknown int
}

// mathFonts draw symbols, and an unknown glyph in them is a summation sign,
// not a lost letter.
var mathFont = regexp.MustCompile(`(?i)(?:^|\+)(?:cmmi|cmsy|cmex|cmbsy|cmmib|msam|msbm|eufm|eurm|rsfs|stmary|wasy|lasy|txsy|txex|txmi|pxsy|pxex|pxmi|symbol|zapf|dingbat|mtmi|mtsy|esint|bbm|dsrom|lmmath|stix.*math|cambria.?math|xits.?math|latinmodern.?math|newtx.?math|euler|fourier.?m)`)

func (d *Document) loadFont(v any) *font {
	fd, _ := d.Resolve(v).(Dict)
	f := &font{uni: map[uint32]string{}, widths: map[uint32]float64{}, dw: 1000}
	if fd == nil {
		f.unread = "a font the page names is not in the file"
		return f
	}
	f.name = textOf(d.Resolve(fd["BaseFont"]))
	sub := d.Resolve(fd["Subtype"])
	if tu, ok := d.Resolve(fd["ToUnicode"]).(*Stream); ok {
		if body, err := d.Decoded(tu); err == nil {
			f.readCMap(body)
		}
	}
	switch sub {
	case Name("Type0"):
		f.cid = true
		desc := Dict(nil)
		if arr, ok := d.Resolve(fd["DescendantFonts"]).(Array); ok && len(arr) > 0 {
			desc, _ = d.Resolve(arr[0]).(Dict)
		}
		if desc != nil {
			if w, ok := asFloat(d.Resolve(desc["DW"])); ok {
				f.dw = w
			}
			d.cidWidths(f, d.Resolve(desc["W"]))
		}
		if len(f.ranges) == 0 {
			f.ranges = []codeRange{{0, 0xFFFF, 2}}
		}
		if len(f.uni) == 0 {
			// Identity with no table is glyph numbers, which only the
			// font program could turn back into letters.
			f.unread = "its letters are stored as glyph numbers with no table saying which letter each is"
		}
	default:
		f.ranges = []codeRange{{0, 0xFF, 1}}
		first, _ := asInt(d.Resolve(fd["FirstChar"]))
		if ws, ok := d.Resolve(fd["Widths"]).(Array); ok {
			for i, w := range ws {
				if v, ok := asFloat(d.Resolve(w)); ok {
					f.widths[uint32(first+i)] = v
				}
			}
		} else {
			f.dw = 500 // one of the fourteen standard fonts, with no widths given
		}
		d.simpleEncoding(f, fd, sub)
		if !f.known && len(f.uni) == 0 {
			if sub == Name("Type3") {
				f.unread = "it draws its letters as pictures and does not say which they are"
			} else {
				f.unread = "it does not say which letter each of its codes is"
			}
		}
	}
	return f
}

func asFloat(v any) (float64, bool) {
	f, ok := v.(float64)
	return f, ok
}

func (d *Document) cidWidths(f *font, v any) {
	arr, ok := v.(Array)
	if !ok {
		return
	}
	for i := 0; i < len(arr); {
		first, ok := asInt(d.Resolve(arr[i]))
		if !ok || i+1 >= len(arr) {
			return
		}
		if list, ok := d.Resolve(arr[i+1]).(Array); ok {
			for k, w := range list {
				if x, ok := asFloat(d.Resolve(w)); ok {
					f.widths[uint32(first+k)] = x
				}
			}
			i += 2
			continue
		}
		if i+2 >= len(arr) {
			return
		}
		last, _ := asInt(d.Resolve(arr[i+1]))
		w, _ := asFloat(d.Resolve(arr[i+2]))
		if last-first > 1<<16 {
			return
		}
		for c := first; c <= last; c++ {
			f.widths[uint32(c)] = w
		}
		i += 3
	}
}

// simpleEncoding fills a simple font's table from its encoding, its
// differences, and when it names no encoding, the font program's own.
func (d *Document) simpleEncoding(f *font, fd Dict, sub any) {
	enc := d.Resolve(fd["Encoding"])
	var base [256]string
	builtin, haveBuiltin := d.builtinEncoding(fd)
	switch e := enc.(type) {
	case Name:
		base = baseEncoding(e)
		f.known = true
	case Dict:
		if b, ok := d.Resolve(e["BaseEncoding"]).(Name); ok {
			base = baseEncoding(b)
		} else if haveBuiltin {
			base = builtin
		} else {
			base = baseEncoding("StandardEncoding")
		}
		f.known = true
		if diffs, ok := d.Resolve(e["Differences"]).(Array); ok {
			code := 0
			for _, item := range diffs {
				switch x := d.Resolve(item).(type) {
				case float64:
					code = int(x)
				case Name:
					if code >= 0 && code < 256 {
						if s, ok := glyphRune(string(x)); ok {
							base[code] = s
						} else {
							base[code] = "\x00" + string(x)
						}
					}
					code++
				}
			}
		}
	default:
		switch {
		case haveBuiltin:
			base = builtin
			f.known = true
		case sub == Name("TrueType") || sub == Name("Type1") || sub == Name("MMType1"):
			// No encoding and no program to read one from: a standard
			// font, or a TrueType font whose codes are its own. Latin
			// text is what both nearly always hold.
			base = baseEncoding("WinAnsiEncoding")
			f.known = true
		}
	}
	f.simple = base
}

var dupRe = regexp.MustCompile(`dup\s+(\d+)\s*/([^\s/\[\]{}()<>]+)\s+put`)

// builtinEncoding reads the encoding a Type 1 font program carries, which
// is what Computer Modern uses: pdfTeX names no encoding for it, so the
// letters are only in here.
func (d *Document) builtinEncoding(fd Dict) ([256]string, bool) {
	var out [256]string
	desc, _ := d.Resolve(fd["FontDescriptor"]).(Dict)
	if desc == nil {
		return out, false
	}
	s, ok := d.Resolve(desc["FontFile"]).(*Stream)
	if !ok {
		return out, false
	}
	body, err := d.Decoded(s)
	if err != nil {
		return out, false
	}
	if n, ok := asInt(d.Resolve(s.Dict["Length1"])); ok && n > 0 && n < len(body) {
		body = body[:n] // the part in the clear, before eexec
	} else if i := bytes.Index(body, []byte("eexec")); i > 0 {
		body = body[:i]
	}
	if bytes.Contains(body, []byte("/Encoding StandardEncoding def")) {
		return baseEncoding("StandardEncoding"), true
	}
	found := false
	for _, m := range dupRe.FindAllSubmatch(body, -1) {
		code, err := strconv.Atoi(string(m[1]))
		if err != nil || code < 0 || code > 255 {
			continue
		}
		if r, ok := glyphRune(string(m[2])); ok {
			out[code] = r
		} else {
			out[code] = "\x00" + string(m[2])
		}
		found = true
	}
	return out, found
}

// readCMap reads a ToUnicode table: the code space, single codes and
// ranges of them.
func (f *font) readCMap(body []byte) {
	lex := &lexer{data: body}
	var ops []any
	mode := ""
	for {
		lex.skipSpace()
		if lex.pos >= len(lex.data) {
			break
		}
		v, err := lex.readObject(nil)
		if err != nil {
			lex.pos++
			continue
		}
		if k, ok := v.(keyword); ok {
			switch string(k) {
			case "begincodespacerange", "beginbfchar", "beginbfrange":
				mode = string(k)
				ops = ops[:0]
				continue
			case "endcodespacerange":
				for i := 0; i+1 < len(ops); i += 2 {
					lo, _ := ops[i].([]byte)
					hi, _ := ops[i+1].([]byte)
					if len(lo) > 0 && len(lo) <= 4 {
						f.ranges = append(f.ranges, codeRange{code(lo), code(hi), len(lo)})
					}
				}
			case "endbfchar":
				for i := 0; i+1 < len(ops); i += 2 {
					src, _ := ops[i].([]byte)
					dst, _ := ops[i+1].([]byte)
					if len(src) > 0 {
						f.uni[code(src)] = utf16Text(dst, true)
						f.noteWidth(len(src))
					}
				}
			case "endbfrange":
				for i := 0; i+2 < len(ops); i += 3 {
					lo, _ := ops[i].([]byte)
					hi, _ := ops[i+1].([]byte)
					if len(lo) == 0 {
						continue
					}
					a, b := code(lo), code(hi)
					if b < a || b-a > 1<<16 {
						continue
					}
					f.noteWidth(len(lo))
					switch dst := ops[i+2].(type) {
					case []byte:
						start := []rune(utf16Text(dst, true))
						if len(start) == 0 {
							continue
						}
						for c := a; c <= b; c++ {
							r := append([]rune{}, start...)
							r[len(r)-1] += rune(c - a)
							f.uni[c] = string(r)
						}
					case Array:
						for k, item := range dst {
							if s, ok := item.([]byte); ok && a+uint32(k) <= b {
								f.uni[a+uint32(k)] = utf16Text(s, true)
							}
						}
					}
				}
			}
			mode = ""
			ops = ops[:0]
			continue
		}
		if mode != "" {
			ops = append(ops, v)
		}
	}
}

// noteWidth remembers the code length a table uses when the code space did
// not say, which is common in tables written by hand-rolled producers.
func (f *font) noteWidth(n int) {
	for _, r := range f.ranges {
		if r.n == n {
			return
		}
	}
	if n >= 1 && n <= 4 {
		f.ranges = append(f.ranges, codeRange{0, 1<<(8*uint(n)) - 1, n})
	}
}

func code(b []byte) uint32 {
	var v uint32
	for _, c := range b {
		v = v<<8 | uint32(c)
	}
	return v
}

// decode turns the codes of one string into text, and says how far it
// moves the pen in thousandths of the font size, and how many of its codes
// were a single-byte space, which word spacing applies to.
func (f *font) decode(s []byte) (string, float64, int) {
	var out strings.Builder
	advance := 0.0
	spaces := 0
	for i := 0; i < len(s); {
		n := f.codeLength(s[i:])
		c := code(s[i : i+n])
		i += n
		f.used++
		w, ok := f.widths[c]
		if !ok {
			w = f.dw
		}
		advance += w
		if n == 1 && c == 32 {
			spaces++
		}
		if t, ok := f.uni[c]; ok {
			out.WriteString(t)
			continue
		}
		if !f.cid && c < 256 {
			t := f.simple[c]
			if strings.HasPrefix(t, "\x00") || t == "" && c > 32 {
				f.unknown++
				continue
			}
			out.WriteString(t)
			continue
		}
		f.unknown++
	}
	return out.String(), advance, spaces
}

// codeLength picks how many bytes the next code takes from the code space.
func (f *font) codeLength(s []byte) int {
	if !f.cid {
		return 1 // a simple font's codes are one byte, whatever its table says
	}
	for n := 1; n <= 4 && n <= len(s); n++ {
		c := code(s[:n])
		for _, r := range f.ranges {
			if r.n == n && c >= r.lo && c <= r.hi {
				return n
			}
		}
	}
	if f.cid && len(s) >= 2 {
		return 2
	}
	return 1
}

// lost says whether a font turned out to draw letters nobody can read: most
// of its codes named nothing.
func (f *font) lost() bool {
	if f.unread != "" {
		return f.used > 0
	}
	return f.used >= 20 && f.unknown*2 > f.used && !mathFont.MatchString(f.name)
}
