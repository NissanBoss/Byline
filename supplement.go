package main

// The supplementary material: the code, the data and the extra PDF that go
// with a paper, and that name their authors in the ordinary course of
// things. A package says who wrote it, a licence says whose it is, a
// repository remembers every commit's author, and an archive keeps the
// login of whoever packed it.

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"path"
	"regexp"
	"strings"
)

func (c *run) add(where, part string, k kind, label, value string, author bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	c.fields = append(c.fields, field{where: where, part: part, kind: k, label: label, value: value, author: author})
}

func (c *run) addFixed(where, part string, k kind, label, value string, lv level) {
	c.fields = append(c.fields, field{where: where, part: part, kind: k, label: label, value: value, fixed: true, level: lv})
}

var (
	// key = value and key: value lines that name an author, in Python,
	// TOML, setup.cfg, YAML, R's DESCRIPTION and Sphinx's conf.py.
	authorLineRe = regexp.MustCompile(`(?im)^[ \t]*(?:__author__|__maintainer__|__credits__|author|authors|maintainer|maintainers|author_email|maintainer_email|__email__|copyright|Authors@R|creator)[ \t]*[:=][ \t]*(.+)$`)
	// The same keys as JSON, the way package.json and .zenodo.json write
	// them.
	authorJSONRe = regexp.MustCompile(`(?is)"(?:author|authors|contributors|maintainers|creators)"\s*:\s*("(?:[^"\\]|\\.)*"|\{[^{}]*\}|\[(?:[^\[\]]|\[[^\[\]]*\])*\])`)
	nameKeyRe    = regexp.MustCompile(`(?i)"?(?:name|given-names|family-names|givenName|familyName|email)"?\s*[:=]\s*"?([^",}\n]+)"?`)
	copyrightRe  = regexp.MustCompile(`(?im)^[ \t#/*;-]*copyright\s*(?:\(c\)|©)?\s*(?:(?:\d{4}\s*[-,]?\s*)+)?(?:by\s+)?(.+)$`)
)

// A copyright line that names nobody: a template's blanks, or the words
// every licence uses for whoever holds it.
var copyrightNobody = regexp.MustCompile(`(?i)^(?:\[?yyyy\]?|<year>|\[year\]|\[fullname\]|\[name of copyright owner\]|<copyright holders?>|the authors?|the contributors|contributors|holders?|notice|and permission notice|owner|the copyright holders?|its contributors|all rights reserved|\(c\)|[-.,;:]*)\.?$`)

func (c *run) textFile(where, name, text string, depth int) {
	base := strings.ToLower(path.Base(strings.ReplaceAll(name, `\`, "/")))
	lowerName := strings.ToLower(strings.ReplaceAll(name, `\`, "/"))
	if strings.Contains(lowerName, ".git/") || strings.HasPrefix(lowerName, ".git/") || strings.Contains(where, ".git/") || strings.Contains(where, `.git\`) {
		c.git(where, base, text)
		return
	}
	borrowed := borrowedDir.MatchString(lowerName)
	if borrowed {
		// Only the names given are looked for in somebody else's code.
		if !c.mine.empty() {
			c.add(where, "the text of the file", kindFile, "text", text, false)
		}
		return
	}
	part := "the text of the file"
	switch {
	case base == "authors" || base == "authors.md" || base == "authors.txt" || base == "contributors" ||
		base == "contributors.md" || base == "contributors.txt" || base == "maintainers" || base == "credits":
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(strings.TrimLeft(line, "*-# "))
			if line != "" && !strings.HasSuffix(line, ":") && len(line) < 120 {
				c.add(where, "the list of authors", kindManifest, "author", line, true)
			}
		}
	case strings.HasPrefix(base, "license") || strings.HasPrefix(base, "licence") || strings.HasPrefix(base, "copying") || strings.HasPrefix(base, "notice"):
		for _, m := range copyrightRe.FindAllStringSubmatch(text, 8) {
			holder := strings.TrimSpace(strings.Trim(m[1], ".,"))
			if holder != "" && !copyrightNobody.MatchString(holder) && len(holder) < 160 {
				c.add(where, "the licence", kindManifest, "copyright holder", holder, true)
			}
		}
	case base == "package.json" || base == ".zenodo.json" || base == "codemeta.json" || base == "composer.json" || strings.HasSuffix(base, ".ipynb"):
		for _, m := range authorJSONRe.FindAllStringSubmatch(text, 16) {
			c.jsonPeople(where, m[1])
		}
	case base == "citation.cff":
		for _, m := range nameKeyRe.FindAllStringSubmatch(text, 64) {
			c.add(where, "the citation file", kindManifest, "author", m[1], true)
		}
	}
	if isManifest(base) {
		for _, m := range authorLineRe.FindAllStringSubmatch(text, 32) {
			v := strings.Trim(strings.TrimSpace(m[1]), `"',[]()`)
			if v == "" || strings.HasPrefix(v, "{") && !strings.Contains(v, "name") {
				continue
			}
			for _, n := range nameKeyRe.FindAllStringSubmatch(v, 8) {
				c.add(where, "the package's author fields", kindManifest, "author", n[1], true)
			}
			if !strings.Contains(v, "name") {
				c.add(where, "the package's author fields", kindManifest, "author", v, true)
			}
		}
	}
	c.add(where, part, kindFile, "text", text, false)
}

// isManifest is a file whose author lines are the package's, not a line of
// code that happens to use the word.
func isManifest(base string) bool {
	switch base {
	case "setup.py", "setup.cfg", "pyproject.toml", "cargo.toml", "description", "conf.py", "__init__.py",
		"_version.py", "__about__.py", "meta.yaml", "environment.yml", "pubspec.yaml", "gemspec", "build.sbt",
		"project.clj", "mix.exs", "dune-project", "package.yaml", "stack.yaml", "flake.nix", "default.nix",
		"citation.cff", "codemeta.json":
		// Not a README: its prose begins lines with "authors:" as often as
		// it lists any, and this one's own first paragraph did.
		return true
	}
	return strings.HasSuffix(base, ".gemspec") || strings.HasSuffix(base, ".cabal") || strings.HasSuffix(base, ".nuspec")
}

func (c *run) jsonPeople(where, v string) {
	if strings.HasPrefix(v, `"`) {
		s := strings.Trim(v, `"`)
		// "Jane Doe <jane@uni.edu> (https://jane.dev)", npm's short form
		if i := strings.IndexAny(s, "<("); i > 0 {
			c.add(where, "the package's author fields", kindManifest, "author", s[:i], true)
		}
		c.add(where, "the package's author fields", kindManifest, "author", s, true)
		return
	}
	for _, m := range nameKeyRe.FindAllStringSubmatch(v, 32) {
		c.add(where, "the package's author fields", kindManifest, "author", m[1], true)
	}
	if strings.HasPrefix(v, "[") && !strings.Contains(v, "{") {
		for _, item := range strings.Split(strings.Trim(v, "[]"), ",") {
			if s := strings.Trim(strings.TrimSpace(item), `"`); s != "" {
				c.add(where, "the package's author fields", kindManifest, "author", s, true)
			}
		}
	}
}

var (
	gitUserRe   = regexp.MustCompile(`(?im)^\s*(name|email)\s*=\s*(.+)$`)
	gitRemoteRe = regexp.MustCompile(`(?im)^\s*url\s*=\s*(.+)$`)
	// a line of a reflog: two hashes, the author and their address, the
	// time and its zone, then what was done.
	gitLogRe = regexp.MustCompile(`(?m)^[0-9a-f]{40,64} [0-9a-f]{40,64} (.*?) <([^>]*)> \d+ ([+-]\d{4})`)
)

// git reads the records a repository keeps about who it is: the remotes,
// the name and address set for it, and the reflog, which writes the
// committer down at every step.
func (c *run) git(where, base, text string) {
	switch base {
	case "config":
		for _, m := range gitUserRe.FindAllStringSubmatch(text, -1) {
			c.add(where, "the repository's settings", kindGit, "user "+strings.ToLower(m[1]), m[2], true)
		}
		for _, m := range gitRemoteRe.FindAllStringSubmatch(text, -1) {
			u := strings.TrimSpace(m[1])
			// git@github.com:owner/repo is the same as an address.
			if rest, ok := strings.CutPrefix(u, "git@"); ok {
				u = "https://" + strings.Replace(rest, ":", "/", 1)
			}
			c.fields = append(c.fields, field{where: where, part: "the repository's settings", kind: kindLink, label: "remote", value: u})
		}
	case "head", "fetch_head", "description":
		zones := map[string]bool{}
		for _, m := range gitLogRe.FindAllStringSubmatch(text, -1) {
			c.add(where, "the repository's reflog", kindGit, "committer", m[1], true)
			c.add(where, "the repository's reflog", kindGit, "committer's address", m[2], false)
			if m[3] != "+0000" && m[3] != "-0000" && !zones[m[3]] {
				zones[m[3]] = true
				c.addFixed(where, "the repository's reflog", kindGit, "clock", "UTC"+m[3][:3]+":"+m[3][3:], tell)
			}
		}
		if base != "head" {
			c.add(where, "the repository's records", kindGit, "text", text, false)
		}
	}
}

// archive reads what a container keeps about who packed it. A tar writes
// the login and the group of whoever made it against every file; a zip
// made by a Mac's Finder keeps each file's extended attributes beside it,
// in __MACOSX, and one of those is the address it was downloaded from.
func (c *run) archiveOwners(where string, entries []entry) {
	owners := map[string]int{}
	for _, e := range entries {
		for _, who := range []string{e.owner, e.group} {
			if who != "" && !machineUsers[strings.ToLower(who)] && who != "wheel" && who != "staff" && who != "users" && who != "root" {
				owners[who]++
			}
		}
	}
	for who, n := range owners {
		c.add(where, fmt.Sprintf("the owner tar wrote against %s", plural(n, "file", "files")), kindArchive, "owner", who, true)
	}
}

// appleDouble reads the extended attributes a Mac keeps in ._ files.
func (c *run) appleDouble(where, name string, data []byte) {
	if len(data) < 26 || binary.BigEndian.Uint32(data) != 0x00051607 {
		return
	}
	of := strings.TrimPrefix(path.Base(name), "._")
	part := "what the Mac kept about " + of
	n := int(binary.BigEndian.Uint16(data[24:]))
	for i := 0; i < n && 26+12*(i+1) <= len(data); i++ {
		e := data[26+12*i:]
		id := binary.BigEndian.Uint32(e)
		off := int(binary.BigEndian.Uint32(e[4:]))
		size := int(binary.BigEndian.Uint32(e[8:]))
		if id != 9 || off < 0 || off+size > len(data) {
			continue // 9 is the Finder information, where the attributes are
		}
		finder := data[off : off+size]
		at := bytes.Index(finder, []byte("ATTR"))
		if at < 0 || at+36 > len(finder) {
			continue
		}
		hdr := finder[at:]
		count := int(binary.BigEndian.Uint16(hdr[34:]))
		p := 36
		for k := 0; k < count && p+11 <= len(hdr); k++ {
			aoff := int(binary.BigEndian.Uint32(hdr[p:]))
			alen := int(binary.BigEndian.Uint32(hdr[p+4:]))
			nlen := int(hdr[p+10])
			if p+11+nlen > len(hdr) {
				break
			}
			attr := strings.TrimRight(string(hdr[p+11:p+11+nlen]), "\x00")
			p += 11 + nlen
			p = (p + 3) &^ 3
			// The offsets count from the start of the ._ file, not from
			// the entry they sit in.
			vstart := aoff - off
			if vstart < 0 || vstart+alen > len(finder) {
				continue
			}
			c.xattr(where, part, attr, finder[vstart:vstart+alen])
		}
	}
}

// plistStrings reads the strings of a binary property list, which is how
// a Mac stores the addresses a file came from. Reading the bytes as text
// instead glued the next object's marker onto the end of each address.
func plistStrings(b []byte) []string {
	if len(b) < 40 || !bytes.HasPrefix(b, []byte("bplist00")) {
		return nil
	}
	t := b[len(b)-32:]
	offSize, refSize := int(t[6]), int(t[7])
	count := int(binary.BigEndian.Uint64(t[8:]))
	table := int(binary.BigEndian.Uint64(t[24:]))
	if offSize < 1 || offSize > 8 || refSize < 1 || count > 1<<16 || table < 8 || table+count*offSize > len(b)-32 {
		return nil
	}
	uint := func(p []byte) int {
		v := 0
		for _, c := range p {
			v = v<<8 | int(c)
		}
		return v
	}
	var out []string
	for i := 0; i < count; i++ {
		at := uint(b[table+i*offSize : table+(i+1)*offSize])
		if at <= 0 || at >= len(b)-32 {
			continue
		}
		marker := b[at]
		kind, n := marker>>4, int(marker&0x0F)
		p := at + 1
		if (kind == 5 || kind == 6) && n == 0x0F {
			// the length is an integer object of its own
			if p >= len(b) || b[p]>>4 != 1 {
				continue
			}
			w := 1 << (b[p] & 0x0F)
			if p+1+w > len(b) {
				continue
			}
			n = uint(b[p+1 : p+1+w])
			p += 1 + w
		}
		switch {
		case kind == 5 && p+n <= len(b):
			out = append(out, string(b[p:p+n]))
		case kind == 6 && p+2*n <= len(b):
			out = append(out, utf16Text(b[p:p+2*n], true))
		}
	}
	return out
}

// xattr reads one extended attribute of a file, from a ._ file or from the
// header a Mac's tar writes.
func (c *run) xattr(where, part, attr string, value []byte) {
	switch {
	case strings.Contains(attr, "kMDItemWhereFroms"):
		text := strings.Join(plistStrings(value), "\n")
		if text == "" {
			text = printable(value)
		}
		for _, u := range urlRe.FindAllString(text, -1) {
			c.addFixed(where, part, kindArchive, "where it was downloaded from", u, points)
		}
	case attr == "com.apple.quarantine":
		q := strings.Split(string(value), ";")
		if len(q) >= 3 && q[2] != "" {
			c.addFixed(where, part, kindArchive, "the program that downloaded it", q[2], tell)
		}
	case strings.Contains(attr, "kMDItemUserTags") || strings.Contains(attr, "FinderComment") || strings.Contains(attr, "kMDItemCreator"):
		c.add(where, part, kindArchive, "Finder's notes", printable(value), false)
	}
}
