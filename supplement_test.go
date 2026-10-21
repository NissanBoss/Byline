package main

import (
	"os"
	"path/filepath"
	"testing"
)

// The supplementary material names its authors in the ordinary course of
// things: the package, the licence, the repository's settings and reflog.
func TestASupplementNamesItsAuthors(t *testing.T) {
	z := zipOf(t, map[string]string{
		"code/pyproject.toml":          "[project]\nname = \"engine\"\nauthors = [{name = \"Ada Lovelace\", email = \"ada@cam.ac.uk\"}]\n",
		"code/LICENSE":                 "MIT License\n\nCopyright (c) 2026 Ada Lovelace\n\nPermission is hereby granted...",
		"code/package.json":            `{"name": "engine", "author": "Charles Babbage <cb@cam.ac.uk>"}`,
		"code/.git/config":             "[remote \"origin\"]\n\turl = git@github.com:alovelace/engine.git\n[user]\n\tname = Ada Lovelace\n",
		"code/.git/logs/HEAD":          "0000000000000000000000000000000000000000 1111111111111111111111111111111111111111 Ada Lovelace <ada@cam.ac.uk> 1760000000 +0100\tcommit (initial): first\n",
		"code/.git/objects/ab/c":       "not read",
		"code/train.py":                "DATA = '/home/alovelace/data/engine'\n",
		"code/third_party/lib/LICENSE": "Copyright (c) 2019 Somebody Else",
		"README.md":                    "git clone https://github.com/alovelace/engine\n",
	})
	_, clues := check(t, nil, writeFile(t, "supplement.zip", z))
	for _, want := range []struct {
		k    kind
		lv   level
		text string
	}{
		{kindManifest, names, "Ada Lovelace"},
		{kindManifest, names, "Charles Babbage"},
		{kindGit, names, "Ada Lovelace|committer"},
		{kindGit, names, "Ada Lovelace|user name"},
		{kindLink, points, "github.com/alovelace"},
		{kindFile, names, "alovelace"},
		{kindFile, points, "github.com/alovelace"},
	} {
		if !has(found(clues, want.k, want.lv), want.text) {
			t.Errorf("no %s at %d with %q: %v", want.k, want.lv, want.text, found(clues, want.k, want.lv))
		}
	}
	for _, cl := range clues {
		if cl.what == "Somebody Else" {
			t.Errorf("a vendored library's author was reported: %v", cl)
		}
	}
}

// tar writes the login of whoever packed it against every file.
func TestTarOwners(t *testing.T) {
	tb := tarOf(t, "alovelace", map[string]string{"a.txt": "x", "b.txt": "y"})
	_, clues := check(t, nil, writeFile(t, "s.tar", tb))
	if !has(found(clues, kindArchive, names), "alovelace|owner") {
		t.Errorf("the tar's owner was not found: %v", clues)
	}
	tb = tarOf(t, "runner", map[string]string{"a.txt": "x"})
	_, clues = check(t, nil, writeFile(t, "s.tar", tb))
	if len(found(clues, kindArchive, names)) > 0 {
		t.Error("a build machine's login was reported as somebody")
	}
}

// A zip from a Mac's Finder keeps where each file was downloaded from.
func TestMacArchives(t *testing.T) {
	z := zipOf(t, map[string]string{
		"data/results.csv":            "a,b\n1,2\n",
		"__MACOSX/data/._results.csv": string(appleDouble("https://mail.example-university.edu/attachment?id=12345")),
	})
	_, clues := check(t, nil, writeFile(t, "s.zip", z))
	if !has(found(clues, kindLink, points), "example-university.edu") {
		t.Errorf("the download address was not found: %v", clues)
	}
}

// Office files keep who made and saved them, and who commented.
func TestOfficeFiles(t *testing.T) {
	d := zipOf(t, map[string]string{
		"[Content_Types].xml": "<Types/>",
		"docProps/core.xml":   "<cp:coreProperties><dc:creator>Ada Lovelace</dc:creator><cp:lastModifiedBy>Charles Babbage</cp:lastModifiedBy></cp:coreProperties>",
		"word/comments.xml":   `<w:comments><w:comment w:id="0" w:author="Mary Somerville" w:initials="MS"><w:p><w:r><w:t>check</w:t></w:r></w:p></w:comment></w:comments>`,
		"word/document.xml":   "<w:document><w:body><w:p><w:r><w:t>Hello</w:t></w:r></w:p></w:body></w:document>",
	})
	_, clues := check(t, nil, writeFile(t, "notes.docx", d))
	got := found(clues, kindOffice, names)
	for _, want := range []string{"Ada Lovelace|created by", "Charles Babbage|last saved by", "Mary Somerville"} {
		if !has(got, want) {
			t.Errorf("%s was not found: %v", want, got)
		}
	}
}

// A folder is walked, its .git read for its settings only, and the
// libraries a virtual environment holds are passed over.
func TestFolders(t *testing.T) {
	dir := t.TempDir()
	must := func(p, body string) {
		full := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	must("README.md", "by ada@cam.ac.uk")
	must(".venv/lib/site-packages/x/METADATA", "Author-email: someone@else.org")
	must(".git/config", "[user]\n\tname = Ada Lovelace\n")
	must("weights.pt", "\x00\x01\x02")
	c, clues := check(t, nil, dir)
	if !has(found(clues, kindFile, names), "ada@cam.ac.uk") {
		t.Errorf("the README's address was not found: %v", clues)
	}
	if !has(found(clues, kindGit, names), "Ada Lovelace") {
		t.Errorf("the .git settings were not read: %v", clues)
	}
	for _, cl := range clues {
		if cl.what == "someone@else.org" {
			t.Error("a library in .venv was read")
		}
	}
	if c.skipped["data"] != 1 {
		t.Errorf("the weights were not counted as passed over: %v", c.skipped)
	}
}
