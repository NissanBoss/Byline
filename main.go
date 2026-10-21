// Byline says whether a paper sent for anonymous review still names its
// authors: in the properties pdfTeX copies from every figure, in what a
// camera wrote into a photo, in comments, links, attachments and the
// properties earlier saves left behind, and in the supplementary material
// that goes with it.
package main

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime/debug"
	"strings"
)

var version = "unreleased"

const (
	exitClean   = 0
	exitFound   = 1 // something names somebody, or one of the names given was found
	exitBroken  = 2
	exitNothing = 3 // nothing could be read, which is not the same as nothing found
)

const tooBig = 1 << 30

func main() {
	var paths, given, nameFiles []string
	reveal := false
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		a := args[i]
		value := func() string {
			if k := strings.IndexByte(a, '='); k > 0 {
				return a[k+1:]
			}
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "byline: %s needs a value\n", a)
				os.Exit(exitBroken)
			}
			i++
			return args[i]
		}
		switch {
		case a == "--reveal" || a == "-reveal":
			reveal = true
		case a == "--version" || a == "-version":
			fmt.Println("Byline " + version)
			return
		case a == "-h" || a == "--help" || a == "-help":
			usage()
			return
		case a == "--name" || a == "-name" || strings.HasPrefix(a, "--name="):
			given = append(given, value())
		case a == "--names" || a == "-names" || strings.HasPrefix(a, "--names="):
			nameFiles = append(nameFiles, value())
		case strings.HasPrefix(a, "-") && len(a) > 1:
			fmt.Fprintf(os.Stderr, "byline: %s is not an option this has\n", a)
			usage()
			os.Exit(exitBroken)
		default:
			paths = append(paths, a)
		}
	}
	if len(paths) == 0 {
		usage()
		os.Exit(exitBroken)
	}
	mine, err := readNames(given, nameFiles)
	if err != nil {
		fmt.Fprintf(os.Stderr, "byline: the names could not be read: %v\n", err)
		os.Exit(exitBroken)
	}
	home, _ := os.UserHomeDir()
	show := shower{home: home, reveal: reveal}
	// A supplement can be a few hundred megabytes of zip; this keeps it
	// from ever being what slows a machine down.
	debug.SetMemoryLimit(1 << 30)

	c := newRun(show, mine)
	files, skipped, err := filesIn(paths, show)
	if err != nil {
		fmt.Fprintf(os.Stderr, "byline: %v\n", err)
		os.Exit(exitBroken)
	}
	c.skipped = skipped
	for _, f := range files {
		c.file(f)
	}
	r := c.report()
	r.render(os.Stdout)
	switch {
	case c.read == 0:
		os.Exit(exitNothing)
	case r.worst() >= Names:
		os.Exit(exitFound)
	}
	os.Exit(exitClean)
}

type run struct {
	show    shower
	mine    *nameSet
	fields  []field
	gaps    []string
	papers  []paperSummary
	read    int
	inside  int
	offices int
	skipped map[string]int
}

func newRun(show shower, mine *nameSet) *run {
	return &run{show: show, mine: mine, skipped: map[string]int{}}
}

func (c *run) gap(where, why string) {
	c.gaps = append(c.gaps, where+": "+why)
}

func (c *run) file(p string) {
	where := c.show.path(p)
	info, err := os.Stat(p)
	if err != nil {
		c.gap(where, "could not be opened")
		return
	}
	if info.Size() > tooBig {
		c.gap(where, fmt.Sprintf("is %s, more than this reads", size(info.Size())))
		return
	}
	data, err := os.ReadFile(p)
	if err != nil {
		c.gap(where, "could not be read")
		return
	}
	c.read++
	c.inspect(where, filepath.ToSlash(p), data, 0)
}

// inspect reads one file, or one file inside another. depth stops a PDF
// attached to a PDF inside a zip from going on for ever.
func (c *run) inspect(where, name string, data []byte, depth int) {
	if depth > 0 {
		c.inside++
	}
	lower := strings.ToLower(name)
	base := path.Base(strings.ReplaceAll(lower, `\`, "/"))
	head := data[:min(len(data), 1024)]
	switch {
	case bytes.Contains(head, []byte("%PDF-")):
		c.paper(where, data, depth)
	case strings.HasPrefix(base, "._") || strings.Contains(lower, "__macosx/"):
		c.appleDouble(where, name, data)
	case bytes.HasPrefix(data, []byte("PK\x03\x04")) || bytes.HasPrefix(data, []byte("PK\x05\x06")):
		if depth >= 4 {
			c.gap(where, "is a container inside a container inside a container, and was not opened")
			return
		}
		entries, comment, gaps, err := zipEntries(data)
		if err != nil {
			c.gap(where, "looks like a zip and could not be opened as one")
			return
		}
		for _, g := range gaps {
			c.gap(where, g)
		}
		if officeDoc(entries) {
			c.office(where, entries, depth)
			return
		}
		c.add(where, "the archive's comment", kindArchive, "comment", comment, false)
		c.entries(where, entries, depth)
	case bytes.HasPrefix(data, []byte{0x1f, 0x8b}) || looksTar(data):
		if depth >= 4 {
			c.gap(where, "is a container inside a container inside a container, and was not opened")
			return
		}
		entries, gaps, err := tarEntries(data)
		if err != nil {
			c.gap(where, "could not be unpacked")
			return
		}
		for _, g := range gaps {
			c.gap(where, g)
		}
		c.archiveOwners(where, entries)
		c.entries(where, entries, depth)
	case bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}):
		c.pictureFile(where, jpegValues(data))
	case bytes.HasPrefix(data, []byte("\x89PNG")):
		c.pictureFile(where, pngValues(data))
	case (strings.HasSuffix(base, ".tif") || strings.HasSuffix(base, ".tiff") || strings.HasSuffix(base, ".dng")) && len(data) > 8:
		c.pictureFile(where, tiffValues(data))
	case isText(data):
		c.textFile(where, name, string(data), depth)
	default:
		c.skipped["binary"]++
	}
}

func (c *run) entries(where string, entries []entry, depth int) {
	for _, e := range entries {
		for attr, v := range e.xattrs {
			c.xattr(where+" > "+e.name, "what the Mac kept about "+path.Base(e.name), attr, v)
		}
		if e.skip {
			continue
		}
		lower := strings.ToLower(e.name)
		if i := strings.Index(lower, ".git/"); i >= 0 && (i == 0 || lower[i-1] == '/') {
			inner := lower[i+5:]
			keep := false
			for _, g := range gitFiles {
				if inner == strings.ToLower(g) {
					keep = true
				}
			}
			if !keep {
				continue
			}
		}
		if skipped(lower) {
			continue
		}
		c.inspect(where+" > "+e.name, e.name, e.data, depth+1)
	}
}

// skipped is a path inside a container that would be skipped in a folder.
func skipped(lower string) bool {
	for _, part := range strings.Split(lower, "/") {
		if skipDirs[part] {
			return true
		}
	}
	return false
}

func (c *run) pictureFile(where string, values []pictureValue) {
	for _, v := range values {
		if v.fixed {
			c.addFixed(where, "what was written into the picture", kindPicture, v.label, v.value, v.level)
			continue
		}
		c.add(where, "what was written into the picture", kindPicture, v.label, v.value, v.author)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `Byline - whether a paper for anonymous review still names its authors

  byline paper.pdf                          what in it could name its authors
  byline paper.pdf supplement.zip code/     and in the supplementary material
  byline --name "Jane Doe" --name jdoe ...  look for these names everywhere
  byline --names authors.txt ...            one name, login or address a line
  byline --reveal ...                       show the names found in full
  byline --version

pdfTeX copies the document properties of every PDF figure into the
paper, and PowerPoint puts the name of whoever is signed in into those.
A photo goes in with what the camera wrote into it. Comments keep the
name of whoever left them, and clearing a PDF's properties with a tool
that appends to the file leaves the old ones in it. This reads every
one of those places, and the words of the paper, and with --name says
exactly where a name of yours is.

It only reads. Nothing is written, and nothing is looked up or sent
anywhere: every answer comes from the files themselves.`)
}
