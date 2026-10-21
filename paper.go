package main

// Every place in a PDF that keeps text, read into fields.
//
// The places that matter are the ones nobody sees. The page shows the words
// and the pictures; it does not show that pdfTeX copied each included
// figure's own document properties into the paper, with the Author that
// PowerPoint wrote into the figure when somebody saved it, or that an
// earlier save of the properties is still in the file after a tool was
// asked to clear them. Those are read first. The words on the page are read
// too, for the names given, because a name in the acknowledgements is as
// much a name as one in a figure.

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
)

type paperRead struct {
	c     *run
	doc   *Document
	where string
	depth int
	pages []Dict
	drawn map[Ref]int
	// seen are the objects already read for a particular reason, so the
	// sweep at the end only reads the rest.
	seen map[Ref]bool

	figures, pictures, comments, attachments, saves int
}

func (c *run) paper(where string, data []byte, depth int) {
	doc, err := Parse(data)
	if err != nil {
		c.gap(where, "could not be read as a PDF: "+err.Error())
		return
	}
	for _, t := range doc.trailersNewestFirst() {
		if t["Encrypt"] != nil {
			c.gap(where, "is encrypted, and so is everything this would read in it: its properties, its comments and its words. Save an unencrypted copy and give it that")
			return
		}
	}
	p := &paperRead{c: c, doc: doc, where: where, depth: depth, seen: map[Ref]bool{}}
	p.pages = doc.Pages()
	p.drawn = doc.drawnOn(p.pages)
	p.saves = saves(data, doc.Revisions)

	p.properties()
	p.objects()
	p.outline()
	p.words()
	p.rest()

	if p.figures > 0 {
		p.fixed(kindFigure, "", "figures with properties", fmt.Sprint(p.figures), tell)
	}
	for _, t := range doc.Trouble {
		c.gap(where, t)
	}
	c.papers = append(c.papers, paperSummary{
		where: where, pages: len(p.pages), figures: p.figures, pictures: p.pictures,
		comments: p.comments, attachments: p.attachments, saves: p.saves,
	})
}

type paperSummary struct {
	where                                                  string
	pages, figures, pictures, comments, attachments, saves int
}

func (p *paperRead) add(k kind, part, label, value string, author bool) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\x00", ""))
	if value == "" {
		return
	}
	p.c.fields = append(p.c.fields, field{where: p.where, part: part, kind: k, label: label, value: value, author: author})
}

func (p *paperRead) fixed(k kind, part, label, value string, lv level) {
	p.c.fields = append(p.c.fields, field{where: p.where, part: part, kind: k, label: label, value: value, fixed: true, level: lv})
}

// infoLabel says what a key of a properties dictionary is called in a
// properties dialog, and whether it holds a person.
func infoLabel(key Name) (string, bool) {
	switch key {
	case "Author":
		return "Author", true
	case "Company", "Manager", "AuthorEmail", "Authors", "LastModifiedBy", "lastModifiedBy":
		return string(key), true
	case "Creator":
		return "made by", false
	case "Producer":
		return "producer", false
	case "Trapped", "PTEX.Fullbanner", "SourceModified":
		return "", false
	}
	return string(key), false
}

// properties reads the paper's own document properties, the ones in force
// and the ones earlier saves left in the file, and its XMP the same way.
func (p *paperRead) properties() {
	doc := p.doc
	live := map[string]bool{}
	var infoRefs []Ref
	liveRef := Ref{-1, 0}
	for i, t := range doc.trailersNewestFirst() {
		ref, ok := t["Info"].(Ref)
		if !ok {
			continue
		}
		// Only the newest trailer's is in force. exiftool -all= writes a
		// trailer with no Info at all, and the first version of this took
		// the one before it for the document's, and so reported the
		// properties exiftool had just been asked to clear as current.
		if i == 0 {
			liveRef = ref
		}
		infoRefs = appendRef(infoRefs, ref)
	}
	if liveRef.Num >= 0 {
		if d, ok := doc.Objects[liveRef].(Dict); ok {
			p.seen[liveRef] = true
			p.dict(d, kindDocument, "the document properties", live, nil)
		}
	}
	// What earlier saves said is only worth telling when it differs from
	// what the file says now: the same Author twice is one Author.
	for _, ref := range infoRefs {
		var olds []Dict
		if ref != liveRef {
			if d, ok := doc.Objects[ref].(Dict); ok {
				olds = append(olds, d)
			}
			p.seen[ref] = true
		}
		for _, s := range doc.earlier(ref) {
			if d, ok := s.Value.(Dict); ok {
				olds = append(olds, d)
			}
		}
		for _, d := range olds {
			p.dict(d, kindEarlier, "the document properties of an earlier save", nil, live)
		}
	}

	root := doc.catalog()
	if root == nil {
		return
	}
	if ref, ok := root["Metadata"].(Ref); ok {
		p.seen[ref] = true
		liveXMP := map[string]bool{}
		if s, ok := doc.Objects[ref].(*Stream); ok {
			p.xmp(s, kindDocument, "the document's XMP metadata", liveXMP, nil)
		}
		for _, old := range doc.earlier(ref) {
			if s, ok := old.Value.(*Stream); ok {
				p.xmp(s, kindEarlier, "the XMP metadata of an earlier save", nil, liveXMP)
			}
		}
	} else if s, ok := doc.Resolve(root["Metadata"]).(*Stream); ok {
		p.xmp(s, kindDocument, "the document's XMP metadata", nil, nil)
	}
	if pi, ok := doc.Resolve(root["PieceInfo"]).(Dict); ok {
		p.private(pi, "the document")
	}
}

func appendRef(list []Ref, r Ref) []Ref {
	for _, x := range list {
		if x == r {
			return list
		}
	}
	return append(list, r)
}

// dict reads a properties dictionary. record, when given, collects what
// was read; skip, when given, is what not to report again.
func (p *paperRead) dict(d Dict, k kind, part string, record, skip map[string]bool) {
	keys := make([]string, 0, len(d))
	for key := range d {
		keys = append(keys, string(key))
	}
	sort.Strings(keys)
	for _, key := range keys {
		v := p.doc.Resolve(d[Name(key)])
		s := textOf(v)
		if _, isName := v.(Name); isName || s == "" {
			continue
		}
		id := key + "\x00" + s
		if record != nil {
			record[id] = true
		}
		if skip != nil && skip[id] {
			continue
		}
		if key == "CreationDate" || key == "ModDate" {
			if z := clockOf(s); z != "" {
				p.fixed(k, part, "clock", z, tell)
			}
			continue
		}
		label, author := infoLabel(Name(key))
		if label != "" {
			p.add(k, part, label, s, author)
		}
	}
}

func (p *paperRead) xmp(s *Stream, k kind, part string, record, skip map[string]bool) {
	body, err := p.doc.Decoded(s)
	if err != nil {
		p.c.gap(p.where, part+" could not be decoded: "+err.Error())
		return
	}
	for _, v := range xmpValues(string(body)) {
		id := v.label + "\x00" + v.value
		if record != nil {
			record[id] = true
		}
		if skip != nil && skip[id] {
			continue
		}
		p.add(k, part, v.label, v.value, v.author)
	}
	for _, z := range xmpClocks(string(body)) {
		p.fixed(k, part, "clock", z, tell)
	}
}

var xmpDateRe = regexp.MustCompile(`(?:xmp:CreateDate|xmp:ModifyDate|xmp:MetadataDate|photoshop:DateCreated|exif:DateTimeOriginal)(?:="|>)\d{4}-\d\d-\d\dT[\d:.]+([+-]\d\d:\d\d)`)

func xmpClocks(x string) []string {
	var out []string
	for _, m := range xmpDateRe.FindAllStringSubmatch(x, -1) {
		if m[1] != "+00:00" && m[1] != "-00:00" {
			out = append(out, "UTC"+m[1])
		}
	}
	return out
}

// figureName is how a figure is called in the report: the file it came
// from, as pdfTeX wrote it down, and the page it is on.
func (p *paperRead) figureName(ref Ref, s *Stream) string {
	name := "a figure"
	if f := textOf(p.doc.Resolve(s.Dict["PTEX.FileName"])); f != "" {
		name = "figure " + p.c.show.inner(f)
	}
	if pg, ok := p.drawn[ref]; ok {
		name += fmt.Sprintf(" (page %d)", pg)
	}
	return name
}

func (p *paperRead) pictureName(ref Ref, s *Stream) string {
	w, _ := asInt(p.doc.Resolve(s.Dict["Width"]))
	h, _ := asInt(p.doc.Resolve(s.Dict["Height"]))
	name := fmt.Sprintf("a picture, %d by %d", w, h)
	if pg, ok := p.drawn[ref]; ok {
		name += fmt.Sprintf(", on page %d", pg)
	}
	return name
}

var annotKinds = map[Name]bool{
	"Text": true, "Link": true, "FreeText": true, "Line": true, "Square": true, "Circle": true, "Polygon": true,
	"PolyLine": true, "Highlight": true, "Underline": true, "Squiggly": true, "StrikeOut": true, "Stamp": true,
	"Caret": true, "Ink": true, "Popup": true, "FileAttachment": true, "Sound": true, "Movie": true,
	"Widget": true, "Screen": true, "Redact": true, "Watermark": true, "3D": true, "RichMedia": true,
}

// objects reads every object for what it is: a figure, a picture, a
// comment, an attachment.
func (p *paperRead) objects() {
	doc := p.doc
	onPage := map[Ref]int{}
	for i, pg := range p.pages {
		if arr, ok := doc.Resolve(pg["Annots"]).(Array); ok {
			for _, a := range arr {
				if r, ok := a.(Ref); ok {
					onPage[r] = i + 1
				}
			}
		}
		if pi, ok := doc.Resolve(pg["PieceInfo"]).(Dict); ok {
			p.private(pi, fmt.Sprintf("page %d", i+1))
		}
	}
	refs := make([]Ref, 0, len(doc.Objects))
	for r := range doc.Objects {
		refs = append(refs, r)
	}
	sort.Slice(refs, func(i, j int) bool {
		return refs[i].Num < refs[j].Num || refs[i].Num == refs[j].Num && refs[i].Gen < refs[j].Gen
	})

	for _, ref := range refs {
		switch o := doc.Objects[ref].(type) {
		case *Stream:
			switch doc.Resolve(o.Dict["Subtype"]) {
			case Name("Form"):
				p.form(ref, o)
			case Name("Image"):
				p.picture(ref, o)
			}
		case Dict:
			sub, _ := doc.Resolve(o["Subtype"]).(Name)
			switch {
			case o["Type"] == Name("Annot") || annotKinds[sub] && (o["Rect"] != nil || o["P"] != nil):
				p.seen[ref] = true
				part := "a comment"
				if pg, ok := onPage[ref]; ok {
					part += fmt.Sprintf(" on page %d", pg)
				}
				p.annotation(o, sub, part)
			case o["Type"] == Name("Filespec") || o["EF"] != nil:
				p.seen[ref] = true
				p.attachment(o)
			}
		}
		// A comment deleted, or a link changed, in a later save is still
		// in the file, and the name on it with it.
		for _, old := range doc.earlier(ref) {
			if d, ok := old.Value.(Dict); ok && d["Subtype"] != nil && annotKinds[nameOf(doc.Resolve(d["Subtype"]))] {
				p.annotation(d, nameOf(doc.Resolve(d["Subtype"])), "a comment an earlier save had")
			}
		}
	}
}

func nameOf(v any) Name {
	n, _ := v.(Name)
	return n
}

// form reads an included figure: the properties pdfTeX copied from it,
// the metadata it brought, and an application's private copy of it.
func (p *paperRead) form(ref Ref, s *Stream) {
	doc := p.doc
	name := ""
	p.seen[ref] = true
	if info, ok := doc.Resolve(s.Dict["PTEX.InfoDict"]).(Dict); ok {
		name = p.figureName(ref, s)
		p.figures++
		p.dict(info, kindFigure, "the properties of "+name, nil, nil)
		if r, ok := s.Dict["PTEX.InfoDict"].(Ref); ok {
			p.seen[r] = true
		}
	}
	if f := textOf(doc.Resolve(s.Dict["PTEX.FileName"])); f != "" {
		if name == "" {
			name = p.figureName(ref, s)
		}
		p.add(kindFigure, "the file name pdfTeX kept for "+name, "file name", f, false)
	}
	if md, ok := doc.Resolve(s.Dict["Metadata"]).(*Stream); ok {
		if name == "" {
			name = p.figureName(ref, s)
		}
		if r, ok := s.Dict["Metadata"].(Ref); ok {
			p.seen[r] = true
		}
		p.xmp(md, kindFigureXMP, "the metadata of "+name, nil, nil)
	}
	if pi, ok := doc.Resolve(s.Dict["PieceInfo"]).(Dict); ok {
		if name == "" {
			name = p.figureName(ref, s)
		}
		p.private(pi, name)
	}
}

// private reads what an application keeps in PieceInfo. Illustrator keeps
// the whole of its own file there when told to preserve editing, which is
// what it does by default, and that file has its own metadata and the
// paths of every picture placed in it.
func (p *paperRead) private(pi Dict, owner string) {
	for app, v := range pi {
		entry, _ := p.doc.Resolve(v).(Dict)
		if entry == nil {
			continue
		}
		priv := p.doc.Resolve(entry["Private"])
		var streams []*Stream
		switch x := priv.(type) {
		case *Stream:
			streams = append(streams, x)
		case Dict:
			keys := make([]string, 0, len(x))
			for k := range x {
				keys = append(keys, string(k))
			}
			sort.Strings(keys)
			for _, k := range keys {
				if s, ok := p.doc.Resolve(x[Name(k)]).(*Stream); ok {
					streams = append(streams, s)
				}
			}
		}
		if len(streams) == 0 {
			continue
		}
		part := fmt.Sprintf("the %s file kept inside %s", app, owner)
		p.fixed(kindPrivate, part, "a copy of the file it was made from", string(app), tell)
		var all []byte
		for _, s := range streams {
			body, err := p.doc.Decoded(s)
			if err != nil {
				continue
			}
			all = append(all, body...)
			if len(all) > 32<<20 {
				break
			}
		}
		for _, x := range xmpPackets(all) {
			for _, v := range xmpValues(x) {
				p.add(kindPrivate, part, v.label, v.value, v.author)
			}
		}
		p.add(kindPrivate, part, "contents", printable(all), false)
	}
}

// picture reads what a camera or an editor wrote into a picture: only a
// JPEG or a JPEG 2000 still has it, because those are copied in as they
// are.
func (p *paperRead) picture(ref Ref, s *Stream) {
	filters := filterList(p.doc.Resolve(s.Dict["Filter"]))
	var values []pictureValue
	for _, f := range filters {
		switch f {
		case "DCTDecode", "DCT":
			values = jpegValues(s.Raw)
		case "JPXDecode":
			for _, x := range xmpPackets(s.Raw) {
				for _, v := range xmpValues(x) {
					values = append(values, pictureValue{label: v.label, value: v.value, author: v.author})
				}
			}
		}
	}
	p.seen[ref] = true
	p.pictures++
	name := ""
	if len(values) > 0 {
		name = p.pictureName(ref, s)
	}
	for _, v := range values {
		if v.fixed {
			p.fixed(kindPicture, "what was written into "+name, v.label, v.value, v.level)
			continue
		}
		p.add(kindPicture, "what was written into "+name, v.label, v.value, v.author)
	}
	if md, ok := p.doc.Resolve(s.Dict["Metadata"]).(*Stream); ok {
		if name == "" {
			name = p.pictureName(ref, s)
		}
		if r, ok := s.Dict["Metadata"].(Ref); ok {
			p.seen[r] = true
		}
		p.xmp(md, kindPicture, "the metadata of "+name, nil, nil)
	}
}

func (p *paperRead) annotation(a Dict, sub Name, part string) {
	doc := p.doc
	if sub == "Link" || sub == "Widget" || sub == "" {
		if act, ok := doc.Resolve(a["A"]).(Dict); ok {
			p.action(act, strings.Replace(part, "a comment", "a link", 1))
		}
		if sub == "Widget" {
			p.add(kindForm, strings.Replace(part, "a comment", "a form field", 1), "value", textOf(doc.Resolve(a["V"])), false)
			p.add(kindForm, strings.Replace(part, "a comment", "a form field", 1), "tip", textOf(doc.Resolve(a["TU"])), false)
		}
		return
	}
	p.comments++
	p.add(kindComment, part, "comment author", textOf(doc.Resolve(a["T"])), true)
	p.add(kindComment, part, "comment", textOf(doc.Resolve(a["Contents"])), false)
	p.add(kindComment, part, "comment subject", textOf(doc.Resolve(a["Subj"])), false)
	if rc := textOf(doc.Resolve(a["RC"])); rc != "" {
		p.add(kindComment, part, "comment", tagRe.ReplaceAllString(rc, " "), false)
	}
	if fs, ok := doc.Resolve(a["FS"]).(Dict); ok {
		p.attachment(fs)
	}
	if act, ok := doc.Resolve(a["A"]).(Dict); ok {
		p.action(act, part)
	}
}

// action reads where a link goes: an address, or a file on the machine
// the paper was made on, which pdfTeX writes down as it was typed.
func (p *paperRead) action(act Dict, part string) {
	doc := p.doc
	switch nameOf(doc.Resolve(act["S"])) {
	case "URI":
		p.add(kindLink, part, "link", textOf(doc.Resolve(act["URI"])), false)
	case "GoToR", "Launch", "GoToE":
		switch f := doc.Resolve(act["F"]).(type) {
		case Dict:
			p.add(kindLink, part, "link to a file", textOf(doc.Resolve(f["UF"])), false)
			p.add(kindLink, part, "link to a file", textOf(doc.Resolve(f["F"])), false)
		default:
			p.add(kindLink, part, "link to a file", textOf(f), false)
		}
	}
	if next, ok := doc.Resolve(act["Next"]).(Dict); ok && next["S"] != nil {
		p.action(next, part)
	}
}

// attachment reads a file attached to the PDF, and then the file itself,
// which is as much the paper's as its pages are.
func (p *paperRead) attachment(fs Dict) {
	doc := p.doc
	name := textOf(doc.Resolve(fs["UF"]))
	if name == "" {
		name = textOf(doc.Resolve(fs["F"]))
	}
	if name == "" {
		name = "an attached file"
	}
	part := "the attached file " + p.c.show.inner(name)
	p.add(kindAttachment, part, "file name", name, false)
	p.add(kindAttachment, part, "description", textOf(doc.Resolve(fs["Desc"])), false)
	ef, _ := doc.Resolve(fs["EF"]).(Dict)
	if ef == nil {
		return
	}
	s, ok := doc.Resolve(ef["F"]).(*Stream)
	if !ok {
		s, ok = doc.Resolve(ef["UF"]).(*Stream)
	}
	if !ok {
		return
	}
	if r, ok := ef["F"].(Ref); ok {
		if p.seen[r] {
			return // one file, named by an annotation and by the attachments list
		}
		p.seen[r] = true
	}
	p.attachments++
	body, err := doc.Decoded(s)
	if err != nil {
		p.c.gap(p.where, part+" could not be decoded: "+err.Error())
		return
	}
	p.c.inspect(p.where+" > "+path.Base(strings.ReplaceAll(name, `\`, "/")), name, body, p.depth+1)
}

// outline reads the bookmarks, whose titles are often the headings and
// sometimes what somebody called a section to find it again.
func (p *paperRead) outline() {
	root := p.doc.catalog()
	if root == nil {
		return
	}
	ol, _ := p.doc.Resolve(root["Outlines"]).(Dict)
	if ol == nil {
		return
	}
	count := 0
	var walk func(node any, depth int)
	walk = func(node any, depth int) {
		for i := 0; node != nil && i < 10000 && depth < 32; i++ {
			d, ok := p.doc.Resolve(node).(Dict)
			if !ok || count > 5000 {
				return
			}
			if r, ok := node.(Ref); ok {
				if p.seen[r] {
					return // a loop in the bookmarks
				}
				p.seen[r] = true
			}
			count++
			p.add(kindOutline, "the bookmarks", "bookmark", textOf(p.doc.Resolve(d["Title"])), false)
			if d["First"] != nil {
				walk(d["First"], depth+1)
			}
			node = d["Next"]
		}
	}
	walk(ol["First"], 0)
}

var appendixRe = regexp.MustCompile(`(?m)^\s*(?:[A-Z](?:\.\d+)*\.?\s+)?(?:Appendix|Appendices|APPENDIX|Supplementary Materials?|SUPPLEMENTARY MATERIALS?|NeurIPS Paper Checklist|Paper Checklist|Checklist)\b`)

// words reads the text of every page. What follows the References heading
// is kept apart, up to an appendix if there is one: a paper names other
// people's work there by its authors, its own authors' included, and that
// is how citing is done.
func (p *paperRead) words() {
	fonts := map[Ref]*font{}
	lost := map[string]bool{}
	inRefs := false
	for i, pg := range p.pages {
		pt, trouble := p.doc.readText(pg, fonts)
		for _, t := range trouble {
			p.c.gap(p.where, fmt.Sprintf("part of page %d could not be read: %s", i+1, t))
		}
		for f := range pt.lost {
			lost[f] = true
		}
		text := pt.text
		page := fmt.Sprintf("the text of page %d", i+1)
		refs := fmt.Sprintf("the references, page %d", i+1)
		if !inRefs {
			if at := referencesStart(text); at >= 0 {
				p.add(kindText, page, "text", text[:at], false)
				text = text[at:]
				inRefs = true
			}
		}
		if inRefs {
			loc := appendixRe.FindStringIndex(text)
			if loc == nil {
				p.add(kindReferences, refs, "text", text, false)
				continue
			}
			p.add(kindReferences, refs, "text", text[:loc[0]], false)
			text = text[loc[0]:]
			inRefs = false
		}
		p.add(kindText, page, "text", text, false)
	}
	if len(lost) > 0 {
		names := make([]string, 0, len(lost))
		for f := range lost {
			names = append(names, strings.TrimLeft(f[strings.IndexByte(f, '+')+1:], "+"))
		}
		sort.Strings(names)
		p.c.gap(p.where, fmt.Sprintf("the words set in %s could not be read, so nothing was looked for in them (%s)",
			plural(len(names), "font", "fonts"), strings.Join(atMost(names, 4), ", ")))
	}
}

// rest sweeps every string in every object not already read, for the
// names given and for addresses: producers keep text in places nobody
// thought to look, and this is the place for those.
func (p *paperRead) rest() {
	doc := p.doc
	refs := make([]Ref, 0, len(doc.Seen))
	for r := range doc.Seen {
		refs = append(refs, r)
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].Num < refs[j].Num })
	for _, ref := range refs {
		if p.seen[ref] {
			continue
		}
		// The catalogue and the bookmarks' root hold the names of what was
		// already read under its own name: the attached files, the
		// outline.
		if d, ok := doc.Objects[ref].(Dict); ok && (d["Type"] == Name("Catalog") || d["Type"] == Name("Outlines")) {
			continue
		}
		// Metadata nothing points to any more, or that hangs off an object
		// this did not read for its own sake.
		if s, ok := doc.Objects[ref].(*Stream); ok && s.Dict["Type"] == Name("Metadata") {
			if p.saves > 1 {
				// Saved more than once and pointed to by nothing: what an
				// earlier save had, which the one that cleared it left.
				p.xmp(s, kindEarlier, "the XMP metadata of an earlier save", nil, nil)
			} else {
				p.xmp(s, kindObject, fmt.Sprintf("XMP metadata kept elsewhere in the file (object %d)", ref.Num), nil, nil)
			}
			continue
		}
		for _, s := range doc.Seen[ref] {
			var strs []string
			collect(s.Value, &strs, 0)
			if len(strs) == 0 {
				continue
			}
			part := fmt.Sprintf("object %d", ref.Num)
			p.add(kindObject, part, "string", strings.Join(strs, "\n"), false)
		}
	}
}

// collect gathers the strings in a value, leaving out the keys that hold
// binary: the file identifiers and the encryption keys.
func collect(v any, out *[]string, depth int) {
	if depth > 16 {
		return
	}
	switch x := v.(type) {
	case []byte:
		if t := textOf(x); readable(t) {
			*out = append(*out, t)
		}
	case Array:
		for _, item := range x {
			collect(item, out, depth+1)
		}
	case Dict:
		for k, item := range x {
			switch k {
			case "ID", "O", "U", "OE", "UE", "Perms", "Contents", "ByteRange", "Cert":
				continue
			}
			collect(item, out, depth+1)
		}
	case *Stream:
		collect(x.Dict, out, depth+1)
	}
}

// readable is a string that is text rather than bytes that happen to sit in
// a string.
func readable(s string) bool {
	if len(s) < 3 {
		return false
	}
	good := 0
	total := 0
	for _, r := range s {
		total++
		if r >= 0x20 && r != 0xFFFD && r < 0xF000 || r == '\n' || r == '\t' {
			good++
		}
	}
	return good*10 >= total*9
}

// printable is the runs of text inside a binary, joined by newlines.
func printable(b []byte) string {
	var out strings.Builder
	start, run := 0, 0
	for i, c := range b {
		if c >= 0x20 && c < 0x7F {
			if run == 0 {
				start = i
			}
			run++
			continue
		}
		if run >= 6 {
			out.Write(b[start:i])
			out.WriteByte('\n')
		}
		run = 0
		if out.Len() > 4<<20 {
			break
		}
	}
	if run >= 6 {
		out.Write(b[start:])
	}
	return out.String()
}
