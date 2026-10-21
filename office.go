package main

// Word, PowerPoint and Excel files, and their LibreOffice cousins, in a
// supplement or attached to a paper. Each keeps who created it and who last
// saved it, and the name on every comment and every tracked change; Excel's
// threaded comments keep the commenter's address as well.

import (
	"path"
	"regexp"
	"strings"
)

var officeFields = []struct {
	re     *regexp.Regexp
	label  string
	author bool
}{
	{regexp.MustCompile(`<dc:creator>([^<]+)</dc:creator>`), "created by", true},
	{regexp.MustCompile(`<cp:lastModifiedBy>([^<]+)</cp:lastModifiedBy>`), "last saved by", true},
	{regexp.MustCompile(`<Company>([^<]+)</Company>`), "company", true},
	{regexp.MustCompile(`<Manager>([^<]+)</Manager>`), "manager", true},
	{regexp.MustCompile(`<meta:initial-creator>([^<]+)</meta:initial-creator>`), "created by", true},
	{regexp.MustCompile(`\bw:author="([^"]+)"`), "author of a comment or a change", true},
	{regexp.MustCompile(`\bw:initials="([^"]+)"`), "initials on a comment", false},
	{regexp.MustCompile(`<p:cmAuthor\b[^>]*\bname="([^"]+)"`), "comment author", true},
	{regexp.MustCompile(`<p188:author\b[^>]*\bname="([^"]+)"`), "comment author", true},
	{regexp.MustCompile(`<author>([^<]+)</author>`), "comment author", true},
	{regexp.MustCompile(`<person\b[^>]*\bdisplayName="([^"]+)"`), "comment author", true},
	{regexp.MustCompile(`<person\b[^>]*\buserId="([^"]+)"`), "commenter's account", true},
	{regexp.MustCompile(`<cp:keywords>([^<]+)</cp:keywords>`), "keywords", false},
	{regexp.MustCompile(`<dc:title>([^<]+)</dc:title>`), "title", false},
	{regexp.MustCompile(`<dc:description>([^<]+)</dc:description>`), "comments", false},
	{regexp.MustCompile(`_SetBy">\s*<vt:lpwstr>([^<]+)</vt:lpwstr>`), "the sensitivity label was set by", true},
	{regexp.MustCompile(`_Owner">\s*<vt:lpwstr>([^<]+)</vt:lpwstr>`), "the sensitivity label's owner", true},
}

// officeDoc says whether a zip is a document rather than an archive.
func officeDoc(entries []entry) bool {
	for _, e := range entries {
		if e.name == "[Content_Types].xml" || e.name == "mimetype" && strings.HasPrefix(string(e.data), "application/vnd.oasis.opendocument") {
			return true
		}
	}
	return false
}

func (c *run) office(where string, entries []entry, depth int) {
	c.offices++
	for _, e := range entries {
		lower := strings.ToLower(e.name)
		part := officePart(lower)
		switch {
		case strings.HasSuffix(lower, ".xml") || strings.HasSuffix(lower, ".rels"):
			x := string(e.data)
			for _, f := range officeFields {
				for _, m := range f.re.FindAllStringSubmatch(x, 64) {
					c.add(where, part, kindOffice, f.label, xmlUnescape(m[1]), f.author)
				}
			}
			// The words of the document, for the names given and for
			// addresses. Tags are dropped, which runs some words
			// together; a name is still a run of letters.
			text := xmlUnescape(tagRe.ReplaceAllString(strings.ReplaceAll(x, "</w:p>", "\n"), " "))
			c.add(where, part, kindOffice, "text", strings.Join(strings.Fields(text), " "), false)
		case strings.Contains(lower, "media/") || strings.Contains(lower, "pictures/"):
			c.inspect(where+" > "+e.name, e.name, e.data, depth+1)
		case strings.HasSuffix(lower, ".bin") && strings.Contains(lower, "embeddings/"),
			strings.HasSuffix(lower, ".xlsx") || strings.HasSuffix(lower, ".docx") || strings.HasSuffix(lower, ".pdf"):
			// A chart's workbook, or a document pasted in whole: its own
			// properties and comments are its own, and still in here.
			c.inspect(where+" > "+e.name, e.name, e.data, depth+1)
		}
	}
}

func officePart(name string) string {
	switch {
	case strings.HasPrefix(name, "docprops/"), name == "meta.xml":
		return "the document properties"
	case strings.Contains(name, "comment"), strings.Contains(name, "persons/"):
		return "the comments"
	case strings.HasPrefix(name, "word/document"), name == "content.xml":
		return "the document"
	case strings.HasPrefix(name, "ppt/slides/"):
		return "slide " + strings.TrimSuffix(strings.TrimPrefix(path.Base(name), "slide"), ".xml")
	case strings.HasPrefix(name, "xl/worksheets/"):
		return "sheet " + strings.TrimSuffix(strings.TrimPrefix(path.Base(name), "sheet"), ".xml")
	}
	return "its " + name
}
