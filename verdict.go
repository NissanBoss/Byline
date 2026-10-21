package main

// From clues to the report: one finding for each kind of place at each
// level, each with the advice for that place.

import (
	"fmt"
	"sort"
	"strings"
)

// placeName is how a kind of place is called in a finding's title.
var placeName = map[kind]string{
	kindDocument:   "the document properties",
	kindEarlier:    "the properties an earlier save left in the file",
	kindFigure:     "the properties of a figure",
	kindFigureXMP:  "the metadata a figure brought with it",
	kindPrivate:    "a copy of a figure's own file kept inside it",
	kindPicture:    "what was written into a picture",
	kindComment:    "a comment",
	kindLink:       "a link",
	kindAttachment: "a file attached to the PDF",
	kindOutline:    "the bookmarks",
	kindForm:       "a form field",
	kindObject:     "the file, somewhere no reader shows",
	kindText:       "the words of the paper",
	kindReferences: "the references",
	kindManifest:   "a package's author fields",
	kindGit:        "a repository's own records",
	kindOffice:     "a document's properties or comments",
	kindArchive:    "what an archive keeps about who packed it",
	kindFile:       "a file of the supplement",
}

var advice = map[kind]string{
	kindFigure:     "pdfTeX copies the document properties of every PDF figure into the paper, and PowerPoint, Word and Keynote write the name of whoever is signed in into those. Put \\pdfsuppressptexinfo=-1 at the top of the main .tex file and compile again; with LuaLaTeX it is \\pdfvariable suppressoptionalinfo 15. Or clear the properties of the figure files themselves.",
	kindFigureXMP:  "This metadata came in with the figure file. Clear it in the figure, or export the figure again with its metadata left out.",
	kindDocument:   "These are the paper's own properties. With hyperref, \\hypersetup{pdfauthor={},pdftitle={}} sets them; in Word, File, Info, Inspect Document clears them. Then check the result again: some tools leave the old ones in the file.",
	kindEarlier:    "Clearing a PDF's properties with a tool that appends to the file, as exiftool does, leaves the earlier ones in it; exiftool's own documentation says so. Rewriting the whole file drops them: qpdf --linearize in.pdf out.pdf, then check out.pdf again.",
	kindPrivate:    "Illustrator keeps a whole copy of its own file inside every PDF it saves while Preserve Illustrator Editing Capabilities is ticked, which it is by default, and that copy has its own metadata and the paths of every picture placed in it. Save the figure again with the box unticked.",
	kindPicture:    "pdfTeX puts a JPEG into the paper as it is, with everything the camera and every editor since wrote into it. Clear the picture before including it, with exiftool -all= picture.jpg, or save it as a PNG, whose text pdfTeX leaves behind.",
	kindComment:    "A comment keeps the name of whoever left it, which a PDF reader takes from its settings or from the account it runs under. Delete the comments before sending the file.",
	kindLink:       "If one of these is yours or your group's, the reader is a click away from your name. anonymous.4open.science serves a repository for review without its owner, and venues accept it.",
	kindAttachment: "A file attached to a PDF goes everywhere the PDF goes. What is in it is listed under its own name.",
	kindOutline:    "The bookmarks are made from the headings, and from whatever a tool was told to call them.",
	kindForm:       "Form fields keep what was typed into them.",
	kindObject:     "This is text kept in the file where no reader shows it. Rewriting the file with qpdf --linearize drops what nothing refers to; what is still in use has to be cleared in the program that wrote it.",
	kindText:       "These are in the words of the paper itself: an acknowledgement, a thank-you, an address, or a sentence left over from a version with names on it.",
	kindReferences: "Citing your own earlier work in the third person is how double-blind venues ask for it to be done, and it is not a slip. It gives you away when the text treats it as yours: \"in our previous work [12]\".",
	kindManifest:   "Package files name their authors on purpose. Blank these in the copy that goes to review.",
	kindGit:        "A .git folder carries the repository's settings and its history, with every commit's author and address. Send the files without it: git archive makes such a copy.",
	kindOffice:     "Word, PowerPoint and Excel keep who created and who last saved a file, and the name on every comment and tracked change. File, Info, Check for Issues, Inspect Document clears them.",
	kindArchive:    "tar writes the login name of whoever packed it against every file, and a zip made by a Mac's Finder keeps each file's extended attributes in __MACOSX, the address it was downloaded from among them. tar --owner=0 --group=0 --numeric-owner, or zip from the command line, leaves both out.",
	kindFile:       "This is in a file of the supplementary material.",
}

func (c *run) report() *Report {
	r := &Report{Title: "Byline: whether this still names its authors"}
	r.Summary = c.summary()
	r.Gaps = append(r.Gaps, c.gaps...)
	clues := examine(c.fields, c.mine)

	type group struct {
		lv    level
		k     kind
		clues []clue
	}
	groups := map[string]*group{}
	var order []string
	figuresWithProperties := 0
	for _, cl := range clues {
		if cl.fixed && cl.label == "figures with properties" {
			n := 0
			fmt.Sscan(cl.value, &n)
			figuresWithProperties += n
			continue
		}
		k := fmt.Sprintf("%d\x00%s", 9-int(cl.level), cl.kind)
		if cl.label == "clock" {
			k = fmt.Sprintf("%d\x00clock", 9-int(cl.level))
		}
		g, ok := groups[k]
		if !ok {
			g = &group{lv: cl.level, k: cl.kind}
			groups[k] = g
			order = append(order, k)
		}
		g.clues = append(g.clues, cl)
	}
	sort.SliceStable(order, func(a, b int) bool { return order[a] < order[b] })
	figureFound := false
	for _, k := range order {
		g := groups[k]
		if strings.HasSuffix(k, "\x00clock") {
			r.add(c.clockFinding(g.clues))
			continue
		}
		if g.k == kindFigure && g.lv >= names {
			figureFound = true
		}
		r.add(c.finding(g.lv, g.k, g.clues))
	}
	if figuresWithProperties > 0 && !figureFound {
		r.add(Finding{Tag: "NOTE    ", Weight: Note,
			Title:  fmt.Sprintf("%s keep the properties of the files they came from", capital(plural(figuresWithProperties, "figure", "figures"))),
			Lines:  []string{"None of them names anybody now. A figure exported again later may."},
			Advice: advice[kindFigure]})
	}
	if len(r.Findings) == 0 && c.read > 0 && len(r.Gaps) == 0 {
		line := "That covers every place listed in the README. What a paper says, how it is written and what it cites can give its authors away too, and none of that is judged here."
		if c.mine.empty() {
			line += " Give your names with --name to have them looked for everywhere, the words of the paper included."
		}
		r.add(Finding{Tag: "CLEAR   ", Weight: Note, Title: "Nothing here names anybody", Advice: line})
	}
	return r
}

var tags = map[level]string{you: "YOU     ", names: "NAMES   ", points: "POINTS  ", tell: "NOTE    "}

func (c *run) finding(lv level, k kind, clues []clue) Finding {
	// Counted by who, not by line: one Author in the properties and the
	// same name in the XMP beside them are one person.
	heads := map[string]bool{}
	for _, cl := range clues {
		heads[foldText(cl.what).text] = true
	}
	lines := c.layout(clues, c.headOf)
	return Finding{Tag: tags[lv], Weight: Severity(lv), Title: c.title(lv, k, placeName[k], len(heads)), Lines: lines, Advice: advice[k]}
}

// layout lists what was found and where, whichever way round is shorter:
// a name in twelve figures is one name and twelve figures, and eight
// authors in one packet are one packet and eight names.
func (c *run) layout(clues []clue, head func(clue) string) []string {
	type group struct {
		key  string
		list []string
	}
	files := map[string]bool{}
	for _, cl := range clues {
		files[cl.where] = true
	}
	many := len(files) > 1 || c.read+c.inside > 1
	gather := func(key, item func(clue) string) []*group {
		var out []*group
		at := map[string]*group{}
		for _, cl := range clues {
			k := key(cl)
			g, ok := at[k]
			if !ok {
				g = &group{key: k}
				at[k] = g
				out = append(out, g)
			}
			g.list = append(g.list, item(cl))
		}
		for _, g := range out {
			g.list = dedupe(g.list)
		}
		return out
	}
	placeOf := func(cl clue) string {
		if many {
			return cl.where + ": " + cl.part
		}
		return cl.part
	}
	byHead := gather(head, placeOf)
	byPlace := gather(placeOf, head)

	var lines []string
	indent := func(prefix, s string) {
		for i, l := range wrapText(s, wrap-4) {
			if i == 0 {
				lines = append(lines, prefix+l)
			} else {
				lines = append(lines, strings.Repeat(" ", runes(prefix)+2)+l)
			}
		}
	}
	if len(byPlace) < len(byHead) {
		for n, g := range byPlace {
			if n > 0 {
				lines = append(lines, "")
			}
			indent("in ", g.key)
			for _, h := range atMost(g.list, 8) {
				indent("  ", h)
			}
		}
		return lines
	}
	for n, g := range byHead {
		if n == 8 {
			lines = append(lines, "", fmt.Sprintf("... and %d more", len(byHead)-8))
			break
		}
		if n > 0 {
			lines = append(lines, "")
		}
		indent("", g.key)
		for _, p := range atMost(g.list, 4) {
			indent("  in ", p)
		}
	}
	return lines
}

func dedupe(list []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range list {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// headOf is the line that says what was found: the name, masked, and what
// it is.
func (c *run) headOf(cl clue) string {
	switch {
	case cl.level == points:
		return c.show.link(cl.what) + ", " + cl.why
	case cl.match != "":
		s := c.show.text(cl.what)
		if !strings.EqualFold(foldText(cl.what).text, foldText(cl.match).text) {
			s += ", as " + c.show.text(cl.match) + " is written there"
		}
		if cl.label != "text" {
			s += " (" + cl.label + ")"
		}
		return s
	case cl.why == "a home folder":
		return "the home folder of " + c.show.text(cl.what)
	case cl.why != "":
		return c.show.text(cl.what) + ", " + cl.why
	case cl.fixed:
		return cl.label + ": " + c.show.fixed(cl)
	}
	return c.show.text(cl.what) + " (" + cl.label + ")"
}

func (c *run) title(lv level, k kind, place string, n int) string {
	switch lv {
	case you:
		if n == 1 {
			return "A name of yours is in " + place
		}
		return "Names of yours are in " + place
	case names:
		if n == 1 {
			return "Somebody is named in " + place
		}
		return fmt.Sprintf("%d people or accounts are named in %s", n, place)
	case points:
		if n == 1 {
			return "It links to a place that leads to somebody"
		}
		return fmt.Sprintf("It links to %d places that lead to somebody", n)
	}
	switch k {
	case kindReferences:
		return "A name given is in the references"
	case kindPrivate:
		return "A figure carries a whole copy of the file it was made from"
	case kindPicture:
		return "A picture carries what can tie it to a camera"
	case kindArchive:
		return "An archive says what downloaded its files"
	}
	return capital(place)
}

// clockFinding gathers the time zones the files give, which say roughly
// where in the world the machines that made them were.
func (c *run) clockFinding(clues []clue) Finding {
	var order []string
	seen := map[string]bool{}
	for _, cl := range clues {
		if !seen[cl.value] {
			seen[cl.value] = true
			order = append(order, cl.value)
		}
	}
	sort.Strings(order)
	lines := c.layout(clues, func(cl clue) string { return cl.value })
	title := "The clock of the machine that made it says " + order[0]
	if len(order) > 1 {
		title = fmt.Sprintf("The clocks of the machines that made it say %d time zones", len(order))
	}
	return Finding{Tag: "NOTE    ", Weight: Note, Title: title, Lines: lines,
		Advice: "A date in a file is written with the time zone of the machine that wrote it, which says roughly where in the world that machine was. Overleaf's servers write UTC, which says nothing."}
}

func capital(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// link is an address, masked but for the words that say what kind of
// place it is.
func (s shower) link(u string) string {
	if runes(u) > 100 {
		u = string([]rune(u)[:97]) + "..."
	}
	if s.reveal {
		return u
	}
	return maskLink(u)
}

// fixed shows what is not a name: a time zone or a program as it is, a
// position and a serial number masked, since either finds the camera.
func (s shower) fixed(cl clue) string {
	switch cl.label {
	case "where it was taken":
		if s.reveal {
			return cl.value
		}
		return "a position, shown with --reveal"
	case "camera serial number", "lens serial number":
		if s.reveal {
			return cl.value
		}
		return maskText(cl.value)
	}
	return cl.value
}

func (c *run) summary() []string {
	var out []string
	for _, p := range c.papers {
		line := fmt.Sprintf("%s: %s", p.where, plural(p.pages, "page", "pages"))
		var bits []string
		if p.figures > 0 {
			bits = append(bits, plural(p.figures, "figure that keeps its own properties", "figures that keep their own properties"))
		}
		if p.pictures > 0 {
			bits = append(bits, plural(p.pictures, "picture", "pictures"))
		}
		if p.comments > 0 {
			bits = append(bits, plural(p.comments, "comment", "comments"))
		}
		if p.attachments > 0 {
			bits = append(bits, plural(p.attachments, "attached file", "attached files"))
		}
		if len(bits) > 0 {
			line += ", " + strings.Join(bits, ", ")
		}
		if p.saves > 1 {
			line += fmt.Sprintf(", saved %d times", p.saves)
		}
		out = append(out, line+".")
	}
	others := c.read + c.inside - len(c.papers)
	if others > 0 {
		out = append(out, fmt.Sprintf("And %s besides.", plural(others, "other file", "other files")))
	}
	if !c.mine.empty() {
		var shown []string
		for _, w := range c.mine.list {
			shown = append(shown, c.show.text(w.given))
		}
		out = append(out, "Looked everywhere for "+strings.Join(atMost(shown, 6), ", ")+".")
	}
	if c.skipped[".git"] > 0 {
		out = append(out, "Of a .git folder, only its settings and its reflog were read. What the history says is a question of its own.")
	}
	if c.skipped["data"] > 0 || c.skipped["binary"] > 0 {
		out = append(out, fmt.Sprintf("%s were not read: weights, arrays, recordings and programs, which keep no names where this could look.", capital(plural(c.skipped["data"]+c.skipped["binary"], "data file", "data and binary files"))))
	}
	return out
}
