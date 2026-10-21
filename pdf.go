package main

// Enough of a PDF reader to answer one question: what in this file could
// say who wrote it? It came from Pentimento, which came from Baggage.
//
// This is deliberately not a general PDF library. It is lenient where a
// renderer has to be strict, because a file that a renderer would reject
// can still be a file somebody is about to email, and refusing to look at
// it would be the least useful thing this program could do.
//
// The one rule it does not bend: when it cannot read something, it says so.
// Reporting a document as clean because the parser gave up is how a tool
// like this gets somebody hurt.

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"strconv"
)

// The values a PDF object can hold. Kept as plain Go types so the rest of
// the program never has to know about any of this.
type (
	Name  string
	Dict  map[Name]any
	Array []any
	Ref   struct{ Num, Gen int }
)

type Stream struct {
	Dict Dict
	Raw  []byte // still encoded; call Decoded
}

// sighting is one definition of one object, and where it was found. A file
// that has been saved more than once holds several for the same number.
type sighting struct {
	Value any
	// Offset is where in the file the definition sits, or -1 when it came
	// out of an object stream, in which case Stm is that stream's number.
	Offset int
	Stm    int
}

type Document struct {
	// Objects holds the live definition of each object: the one a reader
	// following the index would arrive at.
	Objects map[Ref]any
	// Seen holds every definition found, in the order they sit in the
	// file. Anything past the first is a version of that object from an
	// earlier save, still present because appending to a PDF does not
	// remove what was there before.
	Seen map[Ref][]sighting
	// Packed says which object stream an object was unpacked from.
	Packed map[Ref]int
	// Live is the index into Seen of the definition that is in force. It
	// is kept as a position rather than by comparing values, because a
	// string in a PDF arrives as a byte slice and Go will not compare two
	// of those at all.
	Live map[Ref]int
	// Revisions are the saves the file has been through, oldest first.
	Revisions []Revision
	// Trailers are the trailer dictionaries found by scanning, in the
	// order they sit in the file. A file whose index chain cannot be
	// followed usually still has one of these, and it is what names the
	// document information dictionary.
	Trailers []Dict

	Unpacked int // objects recovered from object streams
	Carriers int // object streams opened

	// Trouble collects everything that could not be read. It is not an
	// error, because a document that is half readable is still worth
	// reporting on, but it must reach the report so nobody mistakes a
	// short answer for a clean one.
	Trouble []string
}

func (d *Document) note(format string, args ...any) {
	d.Trouble = append(d.Trouble, fmt.Sprintf(format, args...))
}

// Resolve follows an indirect reference. Anything else comes back as is.
func (d *Document) Resolve(v any) any {
	for range 32 { // a reference loop is malformed, not worth chasing
		r, ok := v.(Ref)
		if !ok {
			return v
		}
		v = d.Objects[r]
	}
	return nil
}

// Decoded gives the stream contents with the filters undone. Flate covers
// essentially every stream in the wild; the two text encodings are there
// because PostScript-minded tools still wrap metadata in them. The rest are
// reported rather than silently skipped.
func (d *Document) Decoded(s *Stream) ([]byte, error) {
	filters := filterList(d.Resolve(s.Dict["Filter"]))
	data := s.Raw
	for _, f := range filters {
		switch f {
		case "FlateDecode", "Fl":
			out, err := inflate(data)
			if err != nil {
				return nil, fmt.Errorf("FlateDecode failed: %w", err)
			}
			data = out
		case "ASCIIHexDecode", "AHx":
			data = unhex(data)
		case "ASCII85Decode", "A85":
			out, err := un85(data)
			if err != nil {
				return nil, fmt.Errorf("ASCII85Decode failed: %w", err)
			}
			data = out
		default:
			return nil, fmt.Errorf("filter %s is not handled", f)
		}
	}
	// The parameters come after all the filters have run. Every
	// cross-reference stream sets one, and a reader that inflates and stops
	// gets numbers that look like offsets and point nowhere.
	parms := s.Dict["DecodeParms"]
	if parms == nil {
		parms = s.Dict["DP"]
	}
	if p, ok := predictorFrom(d, parms); ok && p.Kind > 1 {
		out, err := p.undo(data)
		if err != nil {
			return nil, fmt.Errorf("undoing predictor %d: %w", p.Kind, err)
		}
		data = out
	}
	return data, nil
}

func filterList(v any) []Name {
	switch f := v.(type) {
	case Name:
		return []Name{f}
	case Array:
		var out []Name
		for _, x := range f {
			if n, ok := x.(Name); ok {
				out = append(out, n)
			}
		}
		return out
	}
	return nil
}

// inflate undoes zlib, and falls back to raw deflate. Plenty of PDFs in the
// wild are written without the zlib header, and a renderer that only tried
// one of the two would reject files that open fine everywhere else.
func inflate(data []byte) ([]byte, error) {
	if r, err := zlib.NewReader(bytes.NewReader(data)); err == nil {
		if out, err := io.ReadAll(r); err == nil {
			return out, nil
		} else if len(out) > 0 {
			// Truncated streams are common and the readable part is
			// still worth having.
			return out, nil
		}
	}
	out, err := rawInflate(data)
	if err != nil && len(out) == 0 {
		return nil, err
	}
	return out, nil
}

func unhex(data []byte) []byte {
	var out []byte
	var hi byte
	half := false
	for _, c := range data {
		var v byte
		switch {
		case c >= '0' && c <= '9':
			v = c - '0'
		case c >= 'a' && c <= 'f':
			v = c - 'a' + 10
		case c >= 'A' && c <= 'F':
			v = c - 'A' + 10
		case c == '>':
			if half {
				out = append(out, hi<<4)
			}
			return out
		default:
			continue
		}
		if half {
			out = append(out, hi<<4|v)
		} else {
			hi = v
		}
		half = !half
	}
	if half {
		out = append(out, hi<<4)
	}
	return out
}

// un85 is ASCII85 the way PostScript writes it: z for four zeros, and ~>
// at the end, which encoding/ascii85 does not know about.
func un85(data []byte) ([]byte, error) {
	if i := bytes.Index(data, []byte("~>")); i >= 0 {
		data = data[:i]
	}
	data = bytes.TrimPrefix(bytes.TrimSpace(data), []byte("<~"))
	out := make([]byte, 0, len(data))
	var group [5]byte
	n := 0
	for _, c := range data {
		switch {
		case isSpace(c):
			continue
		case c == 'z' && n == 0:
			out = append(out, 0, 0, 0, 0)
			continue
		case c < '!' || c > 'u':
			return out, fmt.Errorf("%q is not ASCII85", c)
		}
		group[n] = c - '!'
		n++
		if n == 5 {
			v := uint32(0)
			for _, g := range group {
				v = v*85 + uint32(g)
			}
			out = append(out, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
			n = 0
		}
	}
	if n > 1 {
		for i := n; i < 5; i++ {
			group[i] = 84
		}
		v := uint32(0)
		for _, g := range group {
			v = v*85 + uint32(g)
		}
		tail := []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
		out = append(out, tail[:n-1]...)
	}
	return out, nil
}

// Reading the file

// Parse walks the whole file looking for objects rather than trusting the
// cross-reference table. Broken xrefs are common, particularly in files that
// have been edited, and those are exactly the files worth checking.
func Parse(data []byte) (*Document, error) {
	if !bytes.HasPrefix(bytes.TrimLeft(data, " \r\n\t"), []byte("%PDF-")) {
		return nil, errors.New("this does not start like a PDF")
	}
	doc := &Document{
		Objects: map[Ref]any{},
		Seen:    map[Ref][]sighting{},
		Packed:  map[Ref]int{},
		Live:    map[Ref]int{},
	}

	for i := 0; i+3 < len(data); {
		j := bytes.Index(data[i:], []byte("obj"))
		if j < 0 {
			break
		}
		at := i + j
		num, gen, start, ok := objectHeaderBefore(data, at)
		if !ok {
			i = at + 3
			continue
		}
		lex := &lexer{data: data, pos: at + 3}
		value, err := lex.readObject(doc)
		if err != nil {
			doc.note("object %d %d could not be read: %v", num, gen, err)
			i = at + 3
			continue
		}
		ref := Ref{num, gen}
		// Every definition is kept, not only the last. The ones a later
		// save replaced are what this file used to say, and throwing them
		// away here is what stopped anybody being told about them.
		doc.Seen[ref] = append(doc.Seen[ref], sighting{Value: value, Offset: start, Stm: -1})
		doc.Live[ref] = len(doc.Seen[ref]) - 1
		doc.Objects[ref] = value
		i = lex.pos
	}

	if len(doc.Objects) == 0 {
		return nil, errors.New("no objects found; the file may be encrypted or damaged")
	}

	doc.readTrailers(data)
	doc.unpack()

	revisions, trouble := history(data, doc)
	doc.Revisions = revisions
	for _, t := range trouble {
		doc.note("%s", t)
	}
	doc.settle()
	return doc, nil
}

// readTrailers picks up every trailer dictionary in the file. They are
// cheap to find and they are the only thing that says which object holds the
// document properties, so a file with a damaged index still has an answer.
func (d *Document) readTrailers(data []byte) {
	for at := 0; ; {
		j := bytes.Index(data[at:], []byte("trailer"))
		if j < 0 {
			return
		}
		lex := &lexer{data: data, pos: at + j + len("trailer")}
		at += j + len("trailer")
		value, err := lex.readObject(d)
		if err != nil {
			continue
		}
		if dict, ok := value.(Dict); ok {
			d.Trailers = append(d.Trailers, dict)
		}
	}
}

// settle picks the live definition of every object that has more than one.
//
// Scanning the file takes whichever definition sits last, which is right for
// a file saved by appending and wrong when a later save packed the object
// into a stream instead of writing it out. The index of the newest revision
// is what actually decides, so it is used where it can be read, and the
// order in the file is the fallback for when it cannot.
func (d *Document) settle() {
	if len(d.Revisions) == 0 {
		return
	}
	newest := d.Revisions[len(d.Revisions)-1]
	for ref, seen := range d.Seen {
		if len(seen) < 2 {
			continue
		}
		s, ok := newest.Slots[ref.Num]
		if !ok {
			continue
		}
		for i, candidate := range seen {
			switch {
			case s.Kind == 'n' && candidate.Offset == s.Offset,
				s.Kind == 'c' && candidate.Stm == s.Stm:
				d.Live[ref] = i
				d.Objects[ref] = candidate.Value
			}
		}
	}
}

// earlier returns the definitions of an object that a later save replaced,
// oldest first. These are the versions still sitting in the file that
// nothing reading it will ever arrive at.
func (d *Document) earlier(ref Ref) []sighting {
	seen := d.Seen[ref]
	if len(seen) < 2 {
		return nil
	}
	live := d.Live[ref]
	out := make([]sighting, 0, len(seen)-1)
	for i, s := range seen {
		if i != live {
			out = append(out, s)
		}
	}
	return out
}

// objectHeaderBefore reads the "12 0 " that precedes an obj keyword.
func objectHeaderBefore(data []byte, at int) (num, gen, start int, ok bool) {
	i := at - 1
	for i >= 0 && isSpace(data[i]) {
		i--
	}
	genEnd := i + 1
	for i >= 0 && isDigit(data[i]) {
		i--
	}
	genStart := i + 1
	if genStart == genEnd {
		return 0, 0, 0, false
	}
	for i >= 0 && isSpace(data[i]) {
		i--
	}
	numEnd := i + 1
	for i >= 0 && isDigit(data[i]) {
		i--
	}
	numStart := i + 1
	if numStart == numEnd {
		return 0, 0, 0, false
	}
	num, err1 := strconv.Atoi(string(data[numStart:numEnd]))
	gen, err2 := strconv.Atoi(string(data[genStart:genEnd]))
	if err1 != nil || err2 != nil {
		return 0, 0, 0, false
	}
	return num, gen, numStart, true
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\n' || c == '\r' || c == '\t' || c == '\f' || c == 0
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isDelim(c byte) bool {
	switch c {
	case '(', ')', '<', '>', '[', ']', '{', '}', '/', '%':
		return true
	}
	return false
}
