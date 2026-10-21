package main

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The README makes promises. Promises in a README rot; promises with a test
// behind them break the build first.

// It only reads. The files it is given are somebody's paper, and there is
// no way for it to change one, because there is no way for it to write
// anything.
func TestNeverWrites(t *testing.T) {
	banned := []string{
		"os.WriteFile", "os.Create", "os.OpenFile", "os.Remove", "os.RemoveAll",
		"os.Rename", "os.Truncate", "os.Chmod", "os.Chown", "os.Mkdir",
		"os.MkdirAll", "os.Symlink", "os.Link", "os.Chtimes", "os.CreateTemp",
		"ioutil.WriteFile",
	}
	scanSource(t, banned, "this tool must not write anything")
}

// No network code at all. Every answer comes from the files: no name is
// looked up anywhere, no link is followed, and nothing is sent.
func TestNeverOpensTheNetwork(t *testing.T) {
	// As imports, written the two ways gofmt leaves them: "net" itself is
	// a word this source uses, the top-level domain.
	banned := []string{"\t\"net\"\n", "\t\"net/", "import \"net", "net.Dial", "http.Get", "http.Post", "syscall.Socket"}
	scanSource(t, banned, "this tool must have no way to send anything anywhere")
}

func TestNeverRunsAnotherProgram(t *testing.T) {
	scanSource(t, []string{`"os/exec"`, "syscall.Exec", "syscall.ForkExec", "StartProcess"},
		"this tool must not run another program")
}

func scanSource(t *testing.T, banned []string, why string) {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	looked := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		looked++
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, bad := range banned {
			if bytes.Contains(src, []byte(bad)) {
				t.Errorf("%s contains %q: %s", name, bad, why)
			}
		}
	}
	if looked < 10 {
		t.Fatalf("only %d source files were read, so this test proved nothing", looked)
	}
}

// The long dash is not used anywhere in this project, in code, comments,
// documentation or anything the program prints. Written as a number here
// so this file passes its own test.
func TestNoLongDash(t *testing.T) {
	dash := []byte(string(rune(0x2014)))
	looked := 0
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != "." && (strings.HasPrefix(d.Name(), ".") && d.Name() != ".github" || d.Name() == "dist") {
				return fs.SkipDir
			}
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.IndexByte(body, 0) >= 0 {
			return nil
		}
		looked++
		if bytes.Contains(body, dash) {
			t.Errorf("%s has a long dash in it", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if looked < 12 {
		t.Fatalf("only %d files were looked at", looked)
	}
}

// The home folder is written as ~ unless --reveal is given.
func TestTheHomeFolderIsNotShown(t *testing.T) {
	home := filepath.Join(t.TempDir(), "marisol")
	p := filepath.Join(home, "Papers", "icml", "paper.pdf")
	if got := (shower{home: home}).path(p); got != "~/Papers/icml/paper.pdf" {
		t.Errorf("shown as %s", got)
	}
	if got := (shower{home: home, reveal: true}).path(p); got != p {
		t.Errorf("--reveal showed %s", got)
	}
	if got := (shower{}).inner("/Users/marisol/figs/a.pdf"); got != "/Users/m*****l/figs/a.pdf" {
		t.Errorf("a home folder inside a file came out as %s", got)
	}
}

// What it could not read is said, never counted as clean.
func TestAFileItCannotReadIsSaid(t *testing.T) {
	c, _ := check(t, nil, writeFile(t, "paper.pdf", []byte("%PDF-1.7\ncut short")))
	if len(c.report().Gaps) == 0 {
		t.Error("a PDF that could not be read produced no gap")
	}
}
