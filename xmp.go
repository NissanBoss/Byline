package main

// XMP, the metadata packet a PDF, a picture or a figure carries beside its
// older properties.
//
// XMP is RDF written as XML, and the same field turns up as an attribute in
// one program's files and as an element in another's. A real XML parser
// would need the namespaces declared right, and programs do not always
// declare them; so this looks for the prefixed name in both shapes, as
// Pentimento does.

import (
	"regexp"
	"strings"
)

type xmpValue struct {
	label  string
	value  string
	author bool
}

// xmpPeople are the fields that hold a person, or the company one works
// for. The labels are said the way a program's properties dialog says them.
var xmpPeople = []struct {
	name, label string
}{
	{"dc:creator", "creator"},
	{"dc:contributor", "contributor"},
	{"xmpRights:Owner", "rights owner"},
	{"photoshop:Credit", "credit"},
	{"pdf:Author", "author"},
	{"xmp:Author", "author"},
	{"pdfx:Author", "author"},
	{"pdfx:Company", "company"},
	{"Iptc4xmpCore:CreatorContactInfo", ""},
	{"crs:Author", "author"},
	{"exif:Artist", "artist"},
	{"tiff:Artist", "artist"},
	{"aux:OwnerName", "camera owner"},
	{"exifEX:CameraOwnerName", "camera owner"},
	{"MicrosoftPhoto:Author", "author"},
}

// xmpOther are fields worth naming when one of the names given is in
// them, or an address or a home folder is.
var xmpOther = []struct {
	name, label string
}{
	{"dc:title", "title"},
	{"dc:description", "description"},
	{"dc:subject", "keywords"},
	{"dc:rights", "copyright"},
	{"pdf:Keywords", "keywords"},
	{"xmp:CreatorTool", "made by"},
	{"pdf:Producer", "producer"},
	{"stRef:filePath", "file it came from"},
	{"xmpMM:DerivedFrom", "file it came from"},
	{"xmpMM:Ingredients", "files placed in it"},
	{"photoshop:AuthorsPosition", "author's position"},
	{"photoshop:City", "city"},
	{"photoshop:Country", "country"},
	{"Iptc4xmpCore:Location", "location"},
}

var tagRe = regexp.MustCompile(`<[^>]*>`)

func xmpValues(x string) []xmpValue {
	var out []xmpValue
	for _, p := range xmpPeople {
		for _, v := range xmpField(x, p.name) {
			label := p.label
			if label == "" {
				// The contact block of an IPTC creator: its parts are
				// addresses and numbers, read for what they hold.
				out = append(out, xmpValue{"creator's contact details", v, false})
				continue
			}
			out = append(out, xmpValue{label, v, true})
		}
	}
	for _, p := range xmpOther {
		for _, v := range xmpField(x, p.name) {
			out = append(out, xmpValue{p.label, v, false})
		}
	}
	// Everything else in the packet, read for the names given, addresses
	// and home folders: programs keep private fields here too.
	rest := xmlUnescape(tagRe.ReplaceAllString(x, " "))
	rest = strings.Join(strings.Fields(rest), " ")
	if rest != "" {
		out = append(out, xmpValue{"metadata", rest, false})
	}
	for _, a := range attrRe.FindAllStringSubmatch(x, -1) {
		if strings.HasPrefix(a[1], "xmlns") || strings.HasPrefix(a[1], "rdf:") || strings.HasPrefix(a[1], "x:") {
			continue
		}
		out = append(out, xmpValue{"metadata", xmlUnescape(a[2]), false})
	}
	return out
}

var attrRe = regexp.MustCompile(`\s([A-Za-z][\w.-]*:[\w.-]+)="([^"]*)"`)

// xmpField returns every value of a field: the items of a list, the text
// of an element, or an attribute.
func xmpField(x, name string) []string {
	var out []string
	for at := 0; ; {
		i := strings.Index(x[at:], name+`="`)
		if i < 0 {
			break
		}
		rest := x[at+i+len(name)+2:]
		j := strings.IndexByte(rest, '"')
		if j < 0 {
			break
		}
		if v := strings.TrimSpace(xmlUnescape(rest[:j])); v != "" {
			out = append(out, v)
		}
		at += i + len(name) + 2 + j
	}
	for at := 0; ; {
		i := strings.Index(x[at:], "<"+name)
		if i < 0 {
			break
		}
		start := at + i + len(name) + 1
		if start >= len(x) || x[start] != '>' && x[start] != ' ' && x[start] != '\n' && x[start] != '\t' && x[start] != '\r' {
			at = start
			continue
		}
		gt := strings.IndexByte(x[start:], '>')
		if gt < 0 {
			break
		}
		if x[start+gt-1] == '/' { // <name .../> holds only attributes
			for _, a := range attrRe.FindAllStringSubmatch(x[start:start+gt], -1) {
				if !strings.HasPrefix(a[1], "rdf:") {
					out = append(out, xmlUnescape(a[2]))
				}
			}
			at = start + gt
			continue
		}
		body := x[start+gt+1:]
		end := strings.Index(body, "</"+name+">")
		if end < 0 {
			break
		}
		inner := body[:end]
		if strings.Contains(inner, "<rdf:li") {
			for _, li := range strings.Split(inner, "<rdf:li")[1:] {
				if k := strings.IndexByte(li, '>'); k >= 0 {
					li = li[k+1:]
				}
				if k := strings.Index(li, "</rdf:li>"); k >= 0 {
					li = li[:k]
				}
				if v := strings.TrimSpace(xmlUnescape(tagRe.ReplaceAllString(li, " "))); v != "" {
					out = append(out, v)
				}
			}
		} else {
			for _, a := range attrRe.FindAllStringSubmatch(x[start:start+gt], -1) {
				if !strings.HasPrefix(a[1], "rdf:") && !strings.HasPrefix(a[1], "xml:") {
					out = append(out, xmlUnescape(a[2]))
				}
			}
			if v := strings.TrimSpace(xmlUnescape(tagRe.ReplaceAllString(inner, " "))); v != "" {
				out = append(out, strings.Join(strings.Fields(v), " "))
			}
		}
		at = start + gt + 1 + end
	}
	return out
}

func xmlUnescape(s string) string {
	if !strings.Contains(s, "&") {
		return s
	}
	return strings.NewReplacer("&lt;", "<", "&gt;", ">", "&quot;", `"`, "&apos;", "'", "&#xA;", "\n", "&#10;", "\n", "&#xD;", "", "&#13;", "", "&amp;", "&").Replace(s)
}

// xmpPackets finds the XMP inside a larger run of bytes: a picture, an
// application's private copy of a document.
func xmpPackets(b []byte) []string {
	var out []string
	s := string(b)
	for at := 0; len(out) < 16; {
		i := strings.Index(s[at:], "<x:xmpmeta")
		if i < 0 {
			i = strings.Index(s[at:], "<rdf:RDF")
			if i < 0 {
				break
			}
		}
		start := at + i
		end := strings.Index(s[start:], "</x:xmpmeta>")
		if end < 0 {
			end = strings.Index(s[start:], "</rdf:RDF>")
		}
		if end < 0 {
			out = append(out, s[start:min(len(s), start+1<<20)])
			break
		}
		out = append(out, s[start:start+end])
		at = start + end + 1
	}
	return out
}
