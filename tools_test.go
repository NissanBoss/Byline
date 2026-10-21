package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Papers made by the tools people use, built by testdata/tools.sh, each
// with what must be found in it and what must not. The facts the README
// states about pdfTeX, LuaTeX, XeTeX, exiftool and qpdf are these lines.
func TestMadeByTools(t *testing.T) {
	dir := os.Getenv("BYLINE_TOOLS")
	if dir == "" {
		t.Skip("BYLINE_TOOLS is not set; see testdata/tools.sh")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "expect.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	logged := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) != 6 {
			t.Fatalf("a line of expect.tsv is not six columns: %q", line)
		}
		file, given, k, lvText, want, yes := parts[0], parts[1], kind(parts[2]), parts[3], parts[4], parts[5] == "yes"
		lv, _ := strconv.Atoi(lvText)
		t.Run(file+" "+string(k)+" "+want, func(t *testing.T) {
			var names []string
			if given != "-" {
				names = strings.Split(given, ",")
			}
			_, clues := check(t, names, filepath.Join(dir, file))
			if !logged[file] && given == "-" {
				// What was found, for the log: the README's table of what
				// each engine copies comes from here.
				logged[file] = true
				for _, cl := range clues {
					t.Logf("%s level %d %s: %s (%s)", cl.kind, cl.level, cl.part, cl.what, cl.label)
				}
			}
			seen := false
			for _, cl := range clues {
				if cl.kind == k && int(cl.level) == lv && (strings.Contains(cl.what, want) || strings.Contains(cl.label, want) || strings.Contains(cl.value, want)) {
					seen = true
				}
			}
			if seen != yes {
				t.Errorf("%s: %s at level %d with %q found: %v, wanted %v", file, k, lv, want, seen, yes)
			}
		})
	}
}
