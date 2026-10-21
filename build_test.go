package main

// Files built by hand for the tests: PDFs object by object, a JPEG with
// EXIF in it, an AppleDouble file. Nothing here uses a library that writes
// these formats, because the point is to control every byte the reader
// sees, including the ones a library would never write.

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pdfFile builds a PDF one object at a time. Object 1 is the catalog and
// 2 the page tree; the first page is added by page.
type pdfFile struct {
	objs []string
}

func newPDF() *pdfFile {
	p := &pdfFile{}
	p.add("<< /Type /Catalog /Pages 2 0 R >>")
	p.add("") // the page tree, written once the pages are known
	return p
}

func (p *pdfFile) add(body string) int {
	p.objs = append(p.objs, body)
	return len(p.objs)
}

func (p *pdfFile) set(n int, body string) { p.objs[n-1] = body }

func stream(dict, data string) string {
	return fmt.Sprintf("<< %s /Length %d >>\nstream\n%s\nendstream", dict, len(data), data)
}

// page adds a page drawing content, with the resources given, and makes
// it the only page.
func (p *pdfFile) page(resources, content string) int {
	c := p.add(stream("", content))
	pg := p.add(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources %s /Contents %d 0 R >>", resources, c))
	p.set(2, fmt.Sprintf("<< /Type /Pages /Kids [%d 0 R] /Count 1 >>", pg))
	return pg
}

// bytes writes the file with a plain cross-reference table. extra goes in
// the trailer.
func (p *pdfFile) bytes(extra string) []byte {
	if p.objs[1] == "" {
		p.set(2, "<< /Type /Pages /Kids [] /Count 0 >>")
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.7\n%\xe2\xe3\xcf\xd3\n")
	offsets := make([]int, len(p.objs))
	for i, body := range p.objs {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, body)
	}
	start := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(p.objs)+1)
	for _, o := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R %s >>\nstartxref\n%d\n%%%%EOF\n", len(p.objs)+1, extra, start)
	return b.Bytes()
}

// update appends a save to a file: new definitions of some objects, and an
// index for just those that points back at the one before, the way every
// incremental save does.
func update(file []byte, objs map[int]string, size int, extra string) []byte {
	var b bytes.Buffer
	b.Write(file)
	prev := bytes.LastIndex(file, []byte("startxref"))
	var prevAt int
	fmt.Sscan(string(file[prev+len("startxref"):]), &prevAt)
	offsets := map[int]int{}
	nums := []int{}
	for n := range objs {
		nums = append(nums, n)
	}
	for i := 1; i < len(nums); i++ {
		for j := i; j > 0 && nums[j] < nums[j-1]; j-- {
			nums[j], nums[j-1] = nums[j-1], nums[j]
		}
	}
	for _, n := range nums {
		offsets[n] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", n, objs[n])
	}
	start := b.Len()
	b.WriteString("xref\n")
	for _, n := range nums {
		fmt.Fprintf(&b, "%d 1\n%010d 00000 n \n", n, offsets[n])
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R /Prev %d %s >>\nstartxref\n%d\n%%%%EOF\n", size, prevAt, extra, start)
	return b.Bytes()
}

// packed writes the file the way a PDF 1.5 writer does: every object that
// is not a stream inside one object stream, and the index a stream too.
func (p *pdfFile) packed(extra string) []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.7\n%\xe2\xe3\xcf\xd3\n")
	type slot struct{ kind, a, b int }
	slots := make([]slot, len(p.objs)+3)
	var head, body strings.Builder
	n := 0
	stm := len(p.objs) + 1
	for i, o := range p.objs {
		if strings.Contains(o, "stream\n") {
			continue
		}
		fmt.Fprintf(&head, "%d %d ", i+1, body.Len())
		body.WriteString(o + "\n")
		slots[i+1] = slot{2, stm, n}
		n++
	}
	for i, o := range p.objs {
		if !strings.Contains(o, "stream\n") {
			continue
		}
		slots[i+1] = slot{1, b.Len(), 0}
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	inner := head.String() + body.String()
	slots[stm] = slot{1, b.Len(), 0}
	fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", stm, stream(fmt.Sprintf("/Type /ObjStm /N %d /First %d", n, head.Len()), inner))
	xr := stm + 1
	slots[xr] = slot{1, b.Len(), 0}
	var rows bytes.Buffer
	for i := 0; i <= xr; i++ {
		s := slots[i]
		if i == 0 {
			rows.Write([]byte{0, 0, 0, 0, 0, 0xFF, 0xFF})
			continue
		}
		rows.WriteByte(byte(s.kind))
		binary.Write(&rows, binary.BigEndian, uint32(s.a))
		binary.Write(&rows, binary.BigEndian, uint16(s.b))
	}
	fmt.Fprintf(&b, "%d 0 obj\n<< /Type /XRef /Size %d /W [1 4 2] /Root 1 0 R %s /Length %d >>\nstream\n", xr, xr+1, extra, rows.Len())
	b.Write(rows.Bytes())
	fmt.Fprintf(&b, "\nendstream\nendobj\nstartxref\n%d\n%%%%EOF\n", slots[xr].a)
	return b.Bytes()
}

// pdfString writes text as a PDF literal string, escaping what has to be.
func pdfString(s string) string {
	r := strings.NewReplacer(`\`, `\\`, "(", `\(`, ")", `\)`)
	return "(" + r.Replace(s) + ")"
}

// utf16String writes text the way producers write anything outside
// Latin-1: UTF-16 with its byte order mark, in hex.
func utf16String(s string) string {
	var b strings.Builder
	b.WriteString("<FEFF")
	for _, r := range s {
		fmt.Fprintf(&b, "%04X", r)
	}
	b.WriteString(">")
	return b.String()
}

// figure adds a form XObject the way pdfTeX includes a PDF figure, with
// the figure's own properties copied beside it.
func (p *pdfFile) figure(file string, info string) int {
	i := p.add(info)
	return p.add(stream(fmt.Sprintf("/Type /XObject /Subtype /Form /BBox [0 0 100 100] /PTEX.FileName %s /PTEX.PageNumber 1 /PTEX.InfoDict %d 0 R", pdfString(file), i), "0 0 m 100 100 l S"))
}

// exifJPEG makes the start of a JPEG with an EXIF block: an Artist, a
// camera serial number and a position, which is what a phone or a camera
// set up by its owner writes.
func exifJPEG(artist string, lat, lon float64) []byte {
	le := binary.LittleEndian
	var t bytes.Buffer
	t.WriteString("II*\x00")
	binary.Write(&t, le, uint32(8))
	// IFD0: Artist, ExifIFD pointer, GPS pointer
	type tag struct {
		id, kind uint16
		count    uint32
		value    []byte
	}
	writeIFD := func(at int, tags []tag) []byte {
		var ifd, extra bytes.Buffer
		binary.Write(&ifd, le, uint16(len(tags)))
		dataAt := at + 2 + 12*len(tags) + 4
		for _, tg := range tags {
			binary.Write(&ifd, le, tg.id)
			binary.Write(&ifd, le, tg.kind)
			binary.Write(&ifd, le, tg.count)
			if len(tg.value) <= 4 {
				v := append(append([]byte{}, tg.value...), make([]byte, 4-len(tg.value))...)
				ifd.Write(v)
			} else {
				binary.Write(&ifd, le, uint32(dataAt+extra.Len()))
				extra.Write(tg.value)
				if extra.Len()%2 == 1 {
					extra.WriteByte(0)
				}
			}
		}
		binary.Write(&ifd, le, uint32(0))
		return append(ifd.Bytes(), extra.Bytes()...)
	}
	rational := func(v float64) []byte {
		deg := uint32(v)
		minF := (v - float64(deg)) * 60
		min := uint32(minF)
		sec := uint32((minF - float64(min)) * 60 * 100)
		var b bytes.Buffer
		for _, x := range [][2]uint32{{deg, 1}, {min, 1}, {sec, 100}} {
			binary.Write(&b, le, x[0])
			binary.Write(&b, le, x[1])
		}
		return b.Bytes()
	}
	u32 := func(v uint32) []byte { b := make([]byte, 4); le.PutUint32(b, v); return b }
	art := append([]byte(artist), 0)
	// The layout: IFD0 at 8, then the EXIF IFD, then the GPS IFD. Their
	// offsets depend on the sizes before them, so IFD0 is built twice.
	ifd0 := writeIFD(8, []tag{{0x013B, 2, uint32(len(art)), art}, {0x8769, 4, 1, u32(0)}, {0x8825, 4, 1, u32(0)}})
	exifAt := 8 + len(ifd0)
	serial := []byte("0451234567\x00")
	exif := writeIFD(exifAt, []tag{{0xA431, 2, uint32(len(serial)), serial}})
	gpsAt := exifAt + len(exif)
	ifd0 = writeIFD(8, []tag{{0x013B, 2, uint32(len(art)), art}, {0x8769, 4, 1, u32(uint32(exifAt))}, {0x8825, 4, 1, u32(uint32(gpsAt))}})
	gps := writeIFD(gpsAt, []tag{{1, 2, 2, []byte("N\x00")}, {2, 5, 3, rational(lat)}, {3, 2, 2, []byte("W\x00")}, {4, 5, 3, rational(lon)}})
	t.Write(ifd0)
	t.Write(exif)
	t.Write(gps)
	var j bytes.Buffer
	j.Write([]byte{0xFF, 0xD8, 0xFF, 0xE1})
	binary.Write(&j, binary.BigEndian, uint16(2+6+t.Len()))
	j.WriteString("Exif\x00\x00")
	j.Write(t.Bytes())
	j.Write([]byte{0xFF, 0xDA, 0x00, 0x02, 0xFF, 0xD9}) // the picture would start here
	return j.Bytes()
}

// appleDouble makes the ._ file a Mac's Finder puts in __MACOSX, with one
// extended attribute: the address a file was downloaded from, as Safari
// writes it, a binary property list.
func appleDouble(url string) []byte {
	be := binary.BigEndian
	// The value: a bplist00 array of one string. Only the string matters to
	// the reader, which looks for addresses in it, so the rest is minimal.
	value := []byte("bplist00\xa1\x01\x5f\x10" + string(rune(len(url))) + url)
	name := []byte("com.apple.metadata:kMDItemWhereFroms\x00")
	var attr bytes.Buffer
	// 26 bytes of header, two entries, the Finder information at 50.
	const finderAt = 50
	entryLen := 36 + 11 + len(name)
	entryLen = (entryLen + 3) &^ 3
	valueAt := finderAt + 34 + entryLen
	attr.WriteString("ATTR")
	binary.Write(&attr, be, uint32(0))
	binary.Write(&attr, be, uint32(34+entryLen+len(value)))
	binary.Write(&attr, be, uint32(valueAt))
	binary.Write(&attr, be, uint32(len(value)))
	attr.Write(make([]byte, 12))
	binary.Write(&attr, be, uint16(0))
	binary.Write(&attr, be, uint16(1))
	binary.Write(&attr, be, uint32(valueAt))
	binary.Write(&attr, be, uint32(len(value)))
	binary.Write(&attr, be, uint16(0))
	attr.WriteByte(byte(len(name)))
	attr.Write(name)
	for attr.Len() < entryLen {
		attr.WriteByte(0)
	}
	var b bytes.Buffer
	binary.Write(&b, be, uint32(0x00051607))
	binary.Write(&b, be, uint32(0x00020000))
	b.Write(make([]byte, 16))
	binary.Write(&b, be, uint16(2))
	finderLen := 34 + attr.Len() + len(value)
	binary.Write(&b, be, uint32(9))
	binary.Write(&b, be, uint32(finderAt))
	binary.Write(&b, be, uint32(finderLen))
	binary.Write(&b, be, uint32(2))
	binary.Write(&b, be, uint32(finderAt+finderLen))
	binary.Write(&b, be, uint32(0))
	b.Write(make([]byte, 32)) // the Finder information itself
	b.Write([]byte{0, 0})
	b.Write(attr.Bytes())
	b.Write(value)
	return b.Bytes()
}

func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for name, body := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		f.Write([]byte(body))
	}
	w.Close()
	return b.Bytes()
}

func tarOf(t *testing.T, owner string, files map[string]string) []byte {
	t.Helper()
	var b bytes.Buffer
	w := tar.NewWriter(&b)
	for name, body := range files {
		w.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(body)), Uname: owner, Gname: "staff", Typeflag: tar.TypeReg})
		w.Write([]byte(body))
	}
	w.Close()
	return b.Bytes()
}

func writeFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// check runs the program on files, as the command line would, with the
// names given.
func check(t *testing.T, names []string, paths ...string) (*run, []clue) {
	t.Helper()
	mine, err := readNames(names, nil)
	if err != nil {
		t.Fatal(err)
	}
	c := newRun(shower{}, mine)
	files, skipped, err := filesIn(paths, c.show)
	if err != nil {
		t.Fatal(err)
	}
	c.skipped = skipped
	for _, f := range files {
		c.file(f)
	}
	return c, examine(c.fields, c.mine)
}

// found returns the clues of a kind at a level, as "what|label".
func found(clues []clue, k kind, lv level) []string {
	var out []string
	for _, cl := range clues {
		if cl.kind == k && cl.level == lv {
			out = append(out, cl.what+"|"+cl.label)
		}
	}
	return out
}

func has(list []string, want string) bool {
	for _, s := range list {
		if strings.Contains(s, want) {
			return true
		}
	}
	return false
}
