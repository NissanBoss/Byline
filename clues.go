package main

// What is read out of the files, and what it says about who made them.
//
// Reading and judging are kept apart. The readers turn every place a file
// keeps text into a field, saying where it sits and whether that place holds
// a person by definition, the way an Author entry does. Judging is done here,
// once, the same way for a figure in a paper, a picture in a zip and a line
// of a README.

import (
	"fmt"
	"regexp"
	"strings"
)

// kind is which place in a file a field comes from. Each has its own advice,
// because the way to clear the properties of a figure is not the way to
// clear a comment.
type kind string

const (
	kindDocument   kind = "document"   // the paper's own properties
	kindEarlier    kind = "earlier"    // properties an earlier save left behind
	kindFigure     kind = "figure"     // the properties pdfTeX copied from a figure
	kindFigureXMP  kind = "figure-xmp" // the metadata a figure brought along
	kindPrivate    kind = "private"    // an application's own copy of a figure
	kindPicture    kind = "picture"    // what a camera or an editor wrote into a picture
	kindComment    kind = "comment"    // comments and highlights, and who left them
	kindLink       kind = "link"       // where a link goes
	kindAttachment kind = "attachment" // files attached to the PDF
	kindOutline    kind = "outline"    // the bookmarks
	kindForm       kind = "form"       // form fields
	kindObject     kind = "object"     // any other string in the file
	kindText       kind = "text"       // the words on the page
	kindReferences kind = "references" // the words after the References heading
	kindManifest   kind = "manifest"   // a package's author fields
	kindGit        kind = "git"        // a repository's own records
	kindOffice     kind = "office"     // a document's properties and comments
	kindArchive    kind = "archive"    // what an archive keeps about who packed it
	kindFile       kind = "file"       // the text of a file in the supplement
)

// field is one piece of text a file carries.
type field struct {
	where string // the file, as shown in the report
	part  string // where in the file, said so somebody can go and look
	kind  kind
	label string // what the place is called: Author, comment author, link
	value string
	// author is set when the place holds a person whatever is in it: an
	// Author entry, the name on a comment, a camera's Artist.
	author bool
	// fixed is for what is not text to judge, a position or a serial
	// number: it is reported at level, as it stands.
	fixed bool
	level level
}

type level int

const (
	// tell says something about the machine or the people, and names
	// nobody: the clock a figure was made by, a camera's serial number.
	tell level = iota
	// points leads to somebody without naming them: a link to an account,
	// an address at a university, a document shared from a drive.
	points
	// names is somebody's name, address or login, in a place that holds
	// one. From here up the exit code says something was found.
	names
	// you is one of the names given on the command line, found.
	you
)

// clue is a field judged. Its level is the field's, set here.
type clue struct {
	field
	what  string // the part of the value that names or points, when only part does
	why   string // what it is: an email address, a home folder, the account
	match string // the name it matched, for the level you
}

var (
	emailRe = regexp.MustCompile(`[A-Za-z0-9][A-Za-z0-9._%+-]{0,63}@[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*\.[A-Za-z]{2,24}`)
	// A home folder in any of the shapes a path takes: Unix, macOS,
	// Windows with either slash, and Windows seen from WSL or Git Bash.
	homeRe = regexp.MustCompile(`(?i)(?:^|[^A-Za-z0-9])(?:/home/|/Users/|/mnt/[a-z]/Users/|/[a-z]/Users/|[A-Z]:[\\/]{1,2}Users[\\/]{1,2}|[A-Z]:[\\/]{1,2}Documents and Settings[\\/]{1,2})([^\\/\s:*?"<>|]{1,64})`)
	// Windows lets a user's folder have spaces in it, C:\Users\Ada Lovelace,
	// and it is only taken whole when a separator follows: in running text
	// the words after a path are not part of it.
	winHomeRe = regexp.MustCompile(`(?i)(?:^|[^A-Za-z0-9])[A-Z]:[\\/]{1,2}Users[\\/]{1,2}([^\\/\s:*?"<>|]+(?: [^\\/\s:*?"<>|]+){1,3})[\\/]`)
	// Not anchored on a word boundary: a property list puts a length byte
	// in front of each string, and it is often a digit.
	urlRe = regexp.MustCompile(`(?i)(?:https?://|ftp://|\bwww\.)[^\s<>"'(){}\\^` + "`" + `]+`)
)

// Logins that are a machine's rather than anybody's: what a container, a
// build server or a fresh install calls its user.
var machineUsers = map[string]bool{
	"runner": true, "runneradmin": true, "user": true, "ubuntu": true, "root": true, "admin": true,
	"administrator": true, "shared": true, "public": true, "default": true, "default user": true,
	"all users": true, "vagrant": true, "jovyan": true, "jenkins": true, "travis": true, "circleci": true,
	"buildbot": true, "builder": true, "build": true, "docker": true, "guest": true, "pi": true,
	"ec2-user": true, "centos": true, "debian": true, "fedora": true, "arch": true, "codespace": true,
	"vscode": true, "gitpod": true, "node": true, "app": true, "test": true, "username": true,
	"yourname": true, "your_name": true, "name": true, "me": true, "you": true, "anonymous": true,
	"<user>": true, "$user": true, "%username%": true, "...": true, "~": true, "conda": true,
}

// examine judges every field. A name given on the command line is looked
// for everywhere; the rest is judged by the place it sits in.
func examine(fields []field, mine *nameSet) []clue {
	var out []clue
	said := map[string]bool{}
	// The same name turns up under two labels when a packet keeps it as a
	// field and again in its body; the first, more particular one is kept.
	add := func(c clue) {
		k := fmt.Sprint(c.level) + "\x00" + string(c.kind) + "\x00" + c.where + "\x00" + c.part + "\x00" + c.what + "\x00" + c.why
		if said[k] {
			return
		}
		said[k] = true
		out = append(out, c)
	}
	at := func(f field, lv level) field {
		f.level = lv
		return f
	}
	for _, f := range fields {
		v := strings.TrimSpace(f.value)
		if v == "" {
			continue
		}
		if f.fixed {
			add(clue{field: f, what: v})
			continue
		}
		if mine != nil {
			if hits := mine.find(v); len(hits) > 0 {
				for _, h := range hits {
					lv := you
					if f.kind == kindReferences {
						// A paper may cite its authors' earlier work, in
						// the third person; that is the rule in every
						// venue, not a slip, and saying otherwise would
						// be a warning people learn to ignore.
						lv = tell
					}
					add(clue{field: at(f, lv), what: h.text, match: h.name})
				}
				continue
			}
		}
		authors := map[string]bool{}
		if f.author {
			for _, who := range people(v) {
				if !nobody(who) {
					authors[who] = true
					add(clue{field: at(f, names), what: who})
				}
			}
		}
		if f.kind != kindReferences {
			for _, e := range emails(v) {
				if !authors[e] {
					add(clue{field: at(f, names), what: e, why: "an email address"})
				}
			}
		}
		for _, h := range homeFolders(v) {
			add(clue{field: at(f, names), what: h, why: "a home folder"})
		}
		if f.kind == kindReferences {
			continue // a cited paper's address is the cited authors'
		}
		for _, l := range links(f) {
			add(clue{field: at(f, points), what: l.url, why: l.why})
		}
	}
	return withoutEchoes(out)
}

// withoutEchoes drops what the whole text of a file, or the sweep of a
// PDF's leftover strings, finds again after a particular place already did:
// the address in pyproject.toml's author fields is not news a second time
// as an address somewhere in pyproject.toml.
func withoutEchoes(clues []clue) []clue {
	particular := map[string]bool{}
	key := func(cl clue) string {
		return fmt.Sprint(cl.level) + "\x00" + cl.where + "\x00" + strings.ToLower(cl.what)
	}
	for _, cl := range clues {
		if cl.kind != kindFile && cl.kind != kindObject {
			particular[key(cl)] = true
		}
	}
	out := clues[:0]
	for _, cl := range clues {
		if (cl.kind == kindFile || cl.kind == kindObject) && particular[key(cl)] {
			continue
		}
		out = append(out, cl)
	}
	return out
}

// people splits an author field that holds several, the way hyperref
// writes \author{A \and B} and Word joins its authors: by semicolons, new
// lines, and "and" between names.
func people(v string) []string {
	var out []string
	for _, part := range strings.FieldsFunc(v, func(r rune) bool { return r == ';' || r == '\n' || r == '\r' }) {
		for _, p := range strings.Split(part, " and ") {
			if p = strings.TrimSpace(strings.Trim(strings.TrimSpace(p), ",")); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

func emails(s string) []string {
	if !strings.Contains(s, "@") {
		return nil
	}
	var out []string
	for _, e := range emailRe.FindAllString(s, -1) {
		if !placeholderEmail(e) {
			out = append(out, e)
		}
	}
	return out
}

// placeholderEmail is an address nobody receives mail at: the examples in
// templates, and the ones that ask to stay anonymous.
func placeholderEmail(e string) bool {
	l := strings.ToLower(e)
	at := strings.LastIndexByte(l, '@')
	local, host := l[:at], l[at+1:]
	switch {
	case host == "example.com", host == "example.org", host == "example.net", strings.HasSuffix(host, ".example"),
		strings.HasSuffix(host, "example.edu"), host == "domain.com", host == "email.com", host == "mail.com" && local == "name",
		strings.Contains(host, "anonymous"), strings.Contains(local, "anonymous"), strings.HasPrefix(local, "anon"),
		strings.HasSuffix(host, "users.noreply.github.com"), host == "localhost":
		return true
	case local == "noreply" || local == "no-reply":
		return true
	}
	// An image name with a scale in it, logo@2x.png, is not an address.
	return strings.HasSuffix(host, ".png") || strings.HasSuffix(host, ".jpg") || strings.HasSuffix(host, ".svg")
}

func homeFolders(s string) []string {
	if !strings.Contains(s, "/") && !strings.Contains(s, `\`) {
		return nil
	}
	var out []string
	matches := winHomeRe.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		matches = homeRe.FindAllStringSubmatch(s, -1)
	}
	for _, m := range matches {
		u := strings.TrimSpace(m[1])
		if u == "" || machineUsers[strings.ToLower(u)] || strings.HasPrefix(u, "$") || strings.HasPrefix(u, "%") || strings.HasPrefix(u, "{") || strings.HasPrefix(u, "<") {
			continue
		}
		out = append(out, u)
	}
	return out
}
