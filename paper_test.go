package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// The fact the program stands on: a figure's own Author, copied into the
// paper by pdfTeX, is found and said to name somebody, and with the name
// given, to name you.
func TestAFigureNamesItsAuthor(t *testing.T) {
	p := newPDF()
	fig := p.figure("figs/arch.pdf", "<< /Author (Ada Lovelace) /Creator (Microsoft\xae PowerPoint\xae for Microsoft 365) /CreationDate (D:20261003101500+05'30') >>")
	p.page(fmt.Sprintf("<< /XObject << /Fm1 %d 0 R >> >>", fig), "q 1 0 0 1 0 0 cm /Fm1 Do Q")
	path := writeFile(t, "paper.pdf", p.bytes(""))

	_, clues := check(t, nil, path)
	got := found(clues, kindFigure, names)
	if !has(got, "Ada Lovelace|Author") {
		t.Fatalf("the figure's Author was not reported: %v", got)
	}
	if !has(found(clues, kindFigure, tell), "UTC+05:30") {
		t.Error("the figure's clock was not read")
	}
	for _, cl := range clues {
		if cl.kind == kindFigure && cl.level == names && !strings.Contains(cl.part, "figs/arch.pdf (page 1)") {
			t.Errorf("the figure was called %q, not by its file and page", cl.part)
		}
	}

	_, clues = check(t, []string{"Ada Lovelace"}, path)
	if !has(found(clues, kindFigure, you), "Ada Lovelace") {
		t.Errorf("with the name given, the figure was not said to hold it: %v", clues)
	}
}

// What a program puts in an author field when nobody gave it a name is not
// a name, and saying it is would be scolding somebody for doing it right.
func TestPlaceholdersNameNobody(t *testing.T) {
	for _, v := range []string{"Microsoft Office User", "Anonymous Author(s)", "matplotlib", "Python", "user", "Unknown", "Adobe Illustrator 28.0", "", "Microsoft® PowerPoint®"} {
		if !nobody(v) {
			t.Errorf("%q was taken for a person", v)
		}
	}
	for _, v := range []string{"Ada Lovelace", "alovelace", "Lovelace, A.", "Ada", "李华"} {
		if nobody(v) {
			t.Errorf("%q was taken for a placeholder", v)
		}
	}
}

// Clearing the properties with a tool that appends to the file leaves the
// earlier ones in it. exiftool works this way and says so; the old Author
// has to come out as an earlier save, and not as the current one.
func TestAnEarlierSaveStillNames(t *testing.T) {
	p := newPDF()
	info := p.add("<< /Author (Ada Lovelace) /Title (Notes) >>")
	p.page("<< >>", "")
	first := p.bytes(fmt.Sprintf("/Info %d 0 R", info))
	second := update(first, map[int]string{info: "<< /Author () /Title () >>"}, len(p.objs)+1, fmt.Sprintf("/Info %d 0 R", info))
	_, clues := check(t, nil, writeFile(t, "paper.pdf", second))
	if !has(found(clues, kindEarlier, names), "Ada Lovelace") {
		t.Fatalf("the Author an earlier save left was not found: %v", clues)
	}
	if has(found(clues, kindDocument, names), "Ada Lovelace") {
		t.Error("the earlier Author was reported as the current one")
	}
}

// exiftool -all= appends a save whose trailer has no properties at all.
// The ones before are then an earlier save's, not the document's.
func TestATrailerWithoutPropertiesClearsThem(t *testing.T) {
	p := newPDF()
	info := p.add("<< /Author (Ada Lovelace) >>")
	p.page("<< >>", "")
	first := p.bytes(fmt.Sprintf("/Info %d 0 R", info))
	second := update(first, map[int]string{1: "<< /Type /Catalog /Pages 2 0 R >>"}, len(p.objs)+1, "")
	_, clues := check(t, nil, writeFile(t, "cleared.pdf", second))
	if has(found(clues, kindDocument, names), "Ada Lovelace") {
		t.Error("properties no trailer points to any more were reported as the document's")
	}
	if !has(found(clues, kindEarlier, names), "Ada Lovelace") {
		t.Errorf("they were not reported as an earlier save's: %v", clues)
	}
}

// A PDF written this century packs its dictionaries inside object streams
// and indexes them with a stream. The figure has to be found there too.
func TestObjectStreams(t *testing.T) {
	p := newPDF()
	info := p.add("<< /Author (Grace Hopper) >>")
	fig := p.figure("plot.pdf", "<< /Author (Mary Somerville) >>")
	p.page(fmt.Sprintf("<< /XObject << /Fm1 %d 0 R >> >>", fig), "/Fm1 Do")
	_, clues := check(t, nil, writeFile(t, "packed.pdf", p.packed(fmt.Sprintf("/Info %d 0 R", info))))
	if !has(found(clues, kindDocument, names), "Grace Hopper") {
		t.Errorf("the properties inside an object stream were not read: %v", clues)
	}
	if !has(found(clues, kindFigure, names), "Mary Somerville") {
		t.Errorf("the figure's properties inside an object stream were not read: %v", clues)
	}
}

// pdfTeX puts a JPEG in as it is, EXIF and all.
func TestAPhotoKeepsItsCamera(t *testing.T) {
	jpg := exifJPEG("Caroline Herschel", 51.4769, 0.0005)
	p := newPDF()
	img := p.add(stream("/Type /XObject /Subtype /Image /Width 640 /Height 480 /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /DCTDecode", string(jpg)))
	p.page(fmt.Sprintf("<< /XObject << /Im1 %d 0 R >> >>", img), "q 100 0 0 100 0 0 cm /Im1 Do Q")
	_, clues := check(t, nil, writeFile(t, "photo.pdf", p.bytes("")))
	if !has(found(clues, kindPicture, names), "Caroline Herschel|artist") {
		t.Errorf("the camera's Artist was not found: %v", clues)
	}
	if !has(found(clues, kindPicture, points), "where it was taken") {
		t.Errorf("the position was not found: %v", clues)
	}
	if !has(found(clues, kindPicture, tell), "camera serial number") {
		t.Errorf("the serial number was not found: %v", clues)
	}
}

func TestCommentsAndLinks(t *testing.T) {
	p := newPDF()
	note := p.add("<< /Type /Annot /Subtype /Text /Rect [0 0 10 10] /T (Emmy Noether) /Contents (fix this before Friday) >>")
	l1 := p.add("<< /Type /Annot /Subtype /Link /Rect [0 0 10 10] /A << /S /URI /URI (https://github.com/enoether/rings) >> >>")
	l2 := p.add("<< /Type /Annot /Subtype /Link /Rect [0 0 10 10] /A << /S /URI /URI (https://github.com/pytorch/pytorch) >> >>")
	l3 := p.add("<< /Type /Annot /Subtype /Link /Rect [0 0 10 10] /A << /S /URI /URI (https://anonymous.4open.science/r/rings-1234) >> >>")
	l4 := p.add("<< /Type /Annot /Subtype /Link /Rect [0 0 10 10] /A << /S /GoToR /F (/Users/enoether/papers/rings/appendix.pdf) >> >>")
	pg := p.page("<< >>", "")
	body := p.objs[pg-1]
	p.set(pg, strings.Replace(body, "/Contents", fmt.Sprintf("/Annots [%d 0 R %d 0 R %d 0 R %d 0 R %d 0 R] /Contents", note, l1, l2, l3, l4), 1))
	_, clues := check(t, nil, writeFile(t, "c.pdf", p.bytes("")))
	if !has(found(clues, kindComment, names), "Emmy Noether|comment author") {
		t.Errorf("the comment's author was not found: %v", clues)
	}
	links := found(clues, kindLink, points)
	if !has(links, "github.com/enoether") {
		t.Errorf("the link to an account was not reported: %v", links)
	}
	if has(links, "pytorch") || has(links, "anonymous.4open") {
		t.Errorf("a library's or an anonymising service's link was reported: %v", links)
	}
	if !has(found(clues, kindLink, names), "enoether") {
		t.Errorf("the home folder in a link to a file was not found: %v", clues)
	}
}

// The words of the page, set three ways: a standard font, a font whose
// codes are named by Differences with TeX's glyph names, and a composite
// font that only a ToUnicode table can read.
func TestTheWordsAreRead(t *testing.T) {
	p := newPDF()
	f1 := p.add("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>")
	f2 := p.add("<< /Type /Font /Subtype /Type1 /BaseFont /ABCDEF+CMR10 /Encoding << /Type /Encoding /Differences [65 /J /o /s /eacute /space /M /udieresis /l /e /r] >> >>")
	cmap := "/CIDInit /ProcSet findresource begin 12 dict begin begincmap\n1 begincodespacerange <0000> <FFFF> endcodespacerange\n" +
		"2 beginbfchar <0001> <0053> <0002> <00F8> endbfchar\n1 beginbfrange <0003> <0005> <006E> endbfrange\nendcmap end end"
	tu := p.add(stream("", cmap))
	desc := p.add("<< /Type /Font /Subtype /CIDFontType2 /BaseFont /Noto /CIDSystemInfo << /Registry (Adobe) /Ordering (Identity) /Supplement 0 >> /DW 500 >>")
	f3 := p.add(fmt.Sprintf("<< /Type /Font /Subtype /Type0 /BaseFont /Noto /Encoding /Identity-H /DescendantFonts [%d 0 R] /ToUnicode %d 0 R >>", desc, tu))
	content := "BT /F1 12 Tf 72 700 Td (We thank Ada Lovelace.) Tj ET\n" +
		"BT /F2 12 Tf 72 680 Td [(ABCD) -333 (EFGHHIJ)] TJ ET\n" +
		"BT /F3 12 Tf 72 660 Td <0001000200030005> Tj ET\n"
	p.page(fmt.Sprintf("<< /Font << /F1 %d 0 R /F2 %d 0 R /F3 %d 0 R >> >>", f1, f2, f3), content)
	_, clues := check(t, []string{"Ada Lovelace", "José Müller", "Sønp"}, writeFile(t, "w.pdf", p.bytes("")))
	text := found(clues, kindText, you)
	for _, want := range []string{"Ada Lovelace", "José Müller", "Sønp"} {
		if !has(text, want) {
			t.Errorf("%s was not found in the words of the page: %v", want, text)
		}
	}
}

// A name after the References heading is a citation, said as a note and
// not as a slip. One in an appendix after them is not a citation.
func TestReferencesAreKeptApart(t *testing.T) {
	p := newPDF()
	f1 := p.add("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>")
	content := "BT /F1 12 Tf 72 700 Td (Our method is new.) Tj 0 -20 Td (References) Tj 0 -20 Td ([1] A. Lovelace. Notes on the engine. 1843.) Tj " +
		"0 -40 Td (A Appendix) Tj 0 -20 Td (Thanks to Ada Lovelace for the engine.) Tj ET"
	p.page(fmt.Sprintf("<< /Font << /F1 %d 0 R >> >>", f1), content)
	_, clues := check(t, []string{"Ada Lovelace"}, writeFile(t, "r.pdf", p.bytes("")))
	if !has(found(clues, kindReferences, tell), "A. Lovelace") {
		t.Errorf("the citation was not kept apart: %v", clues)
	}
	if !has(found(clues, kindText, you), "Ada Lovelace") {
		t.Errorf("the name in the appendix was not reported: %v", clues)
	}
}

// pdfTeX writes every parenthesis in a string as an octal escape.
func TestOctalEscapes(t *testing.T) {
	l := &lexer{data: []byte(`(a\050b\051c\0123\7)`)}
	v, err := l.readObject(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(v.([]byte)); got != "a(b)c\n3\x07" {
		t.Errorf("read as %q", got)
	}
}

func TestEncryptedIsAGap(t *testing.T) {
	p := newPDF()
	enc := p.add("<< /Filter /Standard /V 2 /R 3 /O <00> /U <00> /P -4 >>")
	p.page("<< >>", "")
	c, _ := check(t, nil, writeFile(t, "e.pdf", p.bytes(fmt.Sprintf("/Encrypt %d 0 R", enc))))
	if len(c.gaps) == 0 {
		t.Error("an encrypted PDF was read as if it said nothing")
	}
}

// A file attached to the PDF is read as a file of its own.
func TestAttachments(t *testing.T) {
	inner := newPDF()
	inner.page("<< >>", "")
	innerBytes := inner.bytes(fmt.Sprintf("/Info %d 0 R", inner.add("<< /Author (Hedy Lamarr) >>")))
	p := newPDF()
	ef := p.add(stream("/Type /EmbeddedFile", string(innerBytes)))
	fs := p.add(fmt.Sprintf("<< /Type /Filespec /F (proof.pdf) /UF (proof.pdf) /EF << /F %d 0 R >> >>", ef))
	p.set(1, fmt.Sprintf("<< /Type /Catalog /Pages 2 0 R /Names << /EmbeddedFiles << /Names [(proof.pdf) %d 0 R] >> >> >>", fs))
	p.page("<< >>", "")
	_, clues := check(t, nil, writeFile(t, "a.pdf", p.bytes("")))
	got := found(clues, kindDocument, names)
	if !has(got, "Hedy Lamarr") {
		t.Errorf("the attached PDF's Author was not found: %v", clues)
	}
}

// What a report shows is masked unless --reveal is given.
func TestTheReportMasks(t *testing.T) {
	p := newPDF()
	p.page("<< >>", "")
	path := writeFile(t, "m.pdf", p.bytes(fmt.Sprintf("/Info %d 0 R", p.add("<< /Author (Ada Lovelace) >>"))))
	c, _ := check(t, nil, path)
	var out bytes.Buffer
	c.report().render(&out)
	if strings.Contains(out.String(), "Lovelace") {
		t.Errorf("the name was printed in full:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "L******e") {
		t.Errorf("the masked name is not in the report:\n%s", out.String())
	}
	c.show.reveal = true
	out.Reset()
	c.report().render(&out)
	if !strings.Contains(out.String(), "Ada Lovelace") {
		t.Errorf("--reveal did not show the name:\n%s", out.String())
	}
}

// The XMP of the paper names its creators too, as a list.
func TestXMPCreators(t *testing.T) {
	x := `<x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF><rdf:Description xmp:CreatorTool="Word"><dc:creator><rdf:Seq><rdf:li>Ada Lovelace</rdf:li><rdf:li>Charles Babbage</rdf:li></rdf:Seq></dc:creator></rdf:Description></rdf:RDF></x:xmpmeta>`
	var people []string
	for _, v := range xmpValues(x) {
		if v.author {
			people = append(people, v.value)
		}
	}
	if strings.Join(people, ",") != "Ada Lovelace,Charles Babbage" {
		t.Errorf("creators read as %v", people)
	}
}
