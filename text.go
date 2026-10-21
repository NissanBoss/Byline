package main

// The words on a page, in the order they were drawn.
//
// A PDF has no spaces to speak of. pdfTeX never draws one: it moves the pen
// and draws the next word further along. So a space here is a gap between
// where one run of letters ended and the next began, worked out from the
// widths the font gives, and a new line is the pen moving down. That is
// close enough to find a name, which is all the text is read for.

import (
	"math"
	"strings"
)

type matrix [6]float64

var identity = matrix{1, 0, 0, 1, 0, 0}

// mul returns m applied first, then n.
func (m matrix) mul(n matrix) matrix {
	return matrix{
		m[0]*n[0] + m[1]*n[2],
		m[0]*n[1] + m[1]*n[3],
		m[2]*n[0] + m[3]*n[2],
		m[2]*n[1] + m[3]*n[3],
		m[4]*n[0] + m[5]*n[2] + n[4],
		m[4]*n[1] + m[5]*n[3] + n[5],
	}
}

func scaleOf(m matrix) float64 {
	s := math.Sqrt(math.Abs(m[0]*m[3] - m[1]*m[2]))
	if s == 0 || math.IsNaN(s) || math.IsInf(s, 0) {
		return 1
	}
	return s
}

// pageText is what one page says, and the fonts on it whose letters could
// not be read.
type pageText struct {
	text string
	lost map[string]bool
}

type textReader struct {
	doc   *Document
	fonts map[Ref]*font
	out   strings.Builder
	busy  map[Ref]bool
	// Where the last letters drawn ended, in page space, and how big they
	// were, to decide whether the next ones start a new word or a line.
	endX, endY, size float64
	started          bool
	lost             map[string]bool
	trouble          []string
}

func (d *Document) readText(pg Dict, fonts map[Ref]*font) (pageText, []string) {
	r := &textReader{doc: d, fonts: fonts, busy: map[Ref]bool{}, lost: map[string]bool{}}
	content, err := d.content(pg)
	if err != nil {
		r.trouble = append(r.trouble, err.Error())
	}
	r.walk(content, d.resources(pg), identity, 0)
	return pageText{text: r.out.String(), lost: r.lost}, r.trouble
}

type textState struct {
	ctm                         matrix
	font                        *font
	size, charSp, wordSp, scale float64
	leading                     float64
}

func (r *textReader) fontNamed(res Dict, name Name) *font {
	fs, _ := r.doc.Resolve(res["Font"]).(Dict)
	if fs == nil {
		return nil
	}
	v := fs[name]
	ref, isRef := v.(Ref)
	if isRef {
		if f, ok := r.fonts[ref]; ok {
			return f
		}
	}
	f := r.doc.loadFont(v)
	if isRef {
		r.fonts[ref] = f
	}
	return f
}

func (r *textReader) walk(content []byte, res Dict, ctm matrix, depth int) {
	if depth > 12 {
		return
	}
	lex := &lexer{data: content}
	gs := textState{ctm: ctm, scale: 1}
	var stack []textState
	var operands []any
	var tm, tlm matrix

	num := func(i int) float64 {
		if i >= len(operands) {
			return 0
		}
		v, _ := operands[len(operands)-1-i].(float64)
		return v
	}
	show := func(s []byte) {
		if gs.font == nil {
			return
		}
		text, adv, spaces := gs.font.decode(s)
		if gs.font.lost() {
			r.lost[gs.font.name] = true
		}
		full := tm.mul(gs.ctm)
		r.place(full[4], full[5], gs.size*scaleOf(full))
		r.out.WriteString(text)
		chars := float64(len([]rune(text)))
		tx := (adv/1000*gs.size + gs.charSp*chars + gs.wordSp*float64(spaces)) * gs.scale
		tm = matrix{1, 0, 0, 1, tx, 0}.mul(tm)
		end := tm.mul(gs.ctm)
		r.endX, r.endY = end[4], end[5]
	}

	for {
		lex.skipSpace()
		if lex.pos >= len(lex.data) {
			break
		}
		v, err := lex.readObject(nil)
		if err != nil {
			// A broken stretch of a content stream does not end the page:
			// step over it, the way a viewer would, and go on reading.
			lex.pos++
			operands = operands[:0]
			continue
		}
		op, isOp := v.(keyword)
		if !isOp {
			operands = append(operands, v)
			if len(operands) > 64 {
				operands = operands[len(operands)-64:]
			}
			continue
		}
		switch op {
		case "q":
			stack = append(stack, gs)
		case "Q":
			if n := len(stack); n > 0 {
				gs = stack[n-1]
				stack = stack[:n-1]
			}
		case "cm":
			m := matrix{num(5), num(4), num(3), num(2), num(1), num(0)}
			gs.ctm = m.mul(gs.ctm)
		case "BT":
			tm, tlm = identity, identity
		case "Tf":
			if len(operands) >= 2 {
				if name, ok := operands[len(operands)-2].(Name); ok {
					gs.font = r.fontNamed(res, name)
				}
			}
			gs.size = num(0)
		case "Tc":
			gs.charSp = num(0)
		case "Tw":
			gs.wordSp = num(0)
		case "Tz":
			gs.scale = num(0) / 100
		case "TL":
			gs.leading = num(0)
		case "Td":
			tlm = matrix{1, 0, 0, 1, num(1), num(0)}.mul(tlm)
			tm = tlm
		case "TD":
			gs.leading = -num(0)
			tlm = matrix{1, 0, 0, 1, num(1), num(0)}.mul(tlm)
			tm = tlm
		case "Tm":
			tlm = matrix{num(5), num(4), num(3), num(2), num(1), num(0)}
			tm = tlm
		case "T*":
			tlm = matrix{1, 0, 0, 1, 0, -gs.leading}.mul(tlm)
			tm = tlm
		case "Tj":
			if s, ok := last(operands).([]byte); ok {
				show(s)
			}
		case "'", "\"":
			if op == "\"" && len(operands) >= 3 {
				gs.wordSp, gs.charSp = num(2), num(1)
			}
			tlm = matrix{1, 0, 0, 1, 0, -gs.leading}.mul(tlm)
			tm = tlm
			if s, ok := last(operands).([]byte); ok {
				show(s)
			}
		case "TJ":
			arr, _ := last(operands).(Array)
			for _, item := range arr {
				switch x := item.(type) {
				case []byte:
					show(x)
				case float64:
					tx := -x / 1000 * gs.size * gs.scale
					tm = matrix{1, 0, 0, 1, tx, 0}.mul(tm)
				}
			}
		case "Do":
			if name, ok := last(operands).(Name); ok {
				r.form(name, res, gs.ctm, depth)
			}
		}
		operands = operands[:0]
	}
}

func last(ops []any) any {
	if len(ops) == 0 {
		return nil
	}
	return ops[len(ops)-1]
}

// place decides what goes between the letters drawn last and the ones
// about to be: nothing, a space, or a new line.
func (r *textReader) place(x, y, size float64) {
	if size <= 0 {
		size = 1
	}
	if !r.started {
		r.started = true
		r.endX, r.endY, r.size = x, y, size
		return
	}
	unit := math.Max(size, r.size)
	r.size = size
	switch dy, dx := math.Abs(y-r.endY), x-r.endX; {
	case dy > 0.5*unit:
		r.out.WriteByte('\n')
	case dx > 0.15*unit || dx < -0.5*unit:
		r.out.WriteByte(' ')
	}
}

// form follows a form XObject, which is how an included PDF figure is
// drawn: the words in a figure are words on the page.
func (r *textReader) form(name Name, res Dict, ctm matrix, depth int) {
	xs, _ := r.doc.Resolve(res["XObject"]).(Dict)
	entry := xs[name]
	s, ok := r.doc.Resolve(entry).(*Stream)
	if !ok || s.Dict["Subtype"] != Name("Form") {
		return
	}
	if ref, isRef := entry.(Ref); isRef {
		if r.busy[ref] {
			return
		}
		r.busy[ref] = true
		defer delete(r.busy, ref)
	}
	body, err := r.doc.Decoded(s)
	if err != nil {
		r.trouble = append(r.trouble, "a figure's drawing could not be decoded: "+err.Error())
		return
	}
	inner := ctm
	if arr, ok := r.doc.Resolve(s.Dict["Matrix"]).(Array); ok && len(arr) == 6 {
		var m matrix
		for i, item := range arr {
			m[i], _ = asFloat(r.doc.Resolve(item))
		}
		inner = m.mul(ctm)
	}
	formRes, _ := r.doc.Resolve(s.Dict["Resources"]).(Dict)
	if formRes == nil {
		formRes = res
	}
	r.walk(body, formRes, inner, depth+1)
	r.out.WriteByte('\n')
}

// referencesStart finds where the list of references begins in a page's
// text: a line that is only the heading, numbered or not.
func referencesStart(text string) int {
	at := 0
	for _, line := range strings.SplitAfter(text, "\n") {
		t := strings.TrimSpace(line)
		t = strings.TrimLeft(t, "0123456789.IVX ")
		switch strings.ToLower(t) {
		case "references", "bibliography", "references and notes", "literature cited", "works cited", "reference", "literatur", "referencias", "bibliographie":
			return at
		}
		at += len(line)
	}
	return -1
}
