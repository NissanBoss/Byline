package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The calibration: recent papers from arXiv, downloaded by
// testdata/arxiv.py, read twice each, with their authors given and
// without. arXiv's papers carry their authors' names openly, so what this
// measures is what the same file would have said if it had been sent to an
// anonymous review: in how many a figure, a picture or a comment names one
// of the authors. Only counts are printed.
func TestCalibration(t *testing.T) {
	dir := os.Getenv("BYLINE_CALIBRATION")
	if dir == "" {
		t.Skip("BYLINE_CALIBRATION is not set; see testdata/arxiv.py")
	}
	f, err := os.Open(filepath.Join(dir, "papers.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	type paper struct {
		File    string   `json:"file"`
		Authors []string `json:"authors"`
	}
	// The places no reader shows. A link to the authors' own repository is
	// left out: in a paper on arXiv it is meant to be there, and the
	// anonymous version would have had another address.
	hidden := map[kind]bool{
		kindFigure: true, kindFigureXMP: true, kindPicture: true, kindPrivate: true, kindComment: true,
		kindEarlier: true, kindAttachment: true, kindObject: true, kindOutline: true, kindForm: true,
	}
	n := 0
	count := map[string]int{}
	tools := map[string]int{}
	strangers := map[string]map[string]bool{} // a figure Author not among the authors, by paper
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<22)
	for sc.Scan() {
		var p paper
		if err := json.Unmarshal(sc.Bytes(), &p); err != nil || p.File == "" {
			continue
		}
		path := filepath.Join(dir, p.File)
		c, mine := check(t, p.Authors, path)
		if len(c.papers) == 0 {
			count["could not be read"]++
			continue
		}
		n++
		if c.papers[0].figures > 0 {
			count["carry figure properties"]++
			count["figures with properties, in all"] += c.papers[0].figures
		}
		for _, g := range c.gaps {
			if strings.Contains(g, "could not be read, so nothing was looked for") {
				count["have words in a font that could not be read"]++
				break
			}
		}
		kinds := map[string]bool{}
		for _, cl := range mine {
			if cl.level != you {
				continue
			}
			kinds[string(cl.kind)] = true
			if cl.kind == kindFigure {
				tools[madeBy(c.fields, cl)]++
			}
		}
		any := false
		for k := range kinds {
			count["name an author in "+k]++
			if hidden[kind(k)] {
				any = true
			}
		}
		if any {
			count["name an author where no reader shows it"]++
		}

		_, plain := check(t, nil, path)
		named, gps, serial, links, clock := false, false, false, false, false
		for _, cl := range plain {
			switch {
			case cl.kind == kindFigure && cl.level == names:
				named = true
				if !isAuthor(cl.what, p.Authors) {
					if strangers[cl.what] == nil {
						strangers[cl.what] = map[string]bool{}
					}
					strangers[cl.what][p.File] = true
				}
			case cl.kind == kindPicture && cl.label == "where it was taken":
				gps = true
			case cl.kind == kindPicture && cl.label == "camera serial number":
				serial = true
			case cl.level == points:
				links = true
			case cl.label == "clock" && cl.kind == kindFigure:
				clock = true
			}
		}
		for k, v := range map[string]bool{"have a figure that names somebody": named, "have a photo with its position": gps,
			"have a photo with its camera's serial number": serial, "link to a place that leads to somebody": links,
			"have a figure made on a clock not set to UTC": clock} {
			if v {
				count[k]++
			}
		}
	}
	if n == 0 {
		t.Fatal("no papers were read")
	}
	keys := make([]string, 0, len(count))
	for k := range count {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Printf("\n%d papers read\n", n)
	for _, k := range keys {
		fmt.Printf("  %5d  %-55s %s\n", count[k], k, percent(float64(count[k]), float64(n)))
	}
	fmt.Println("\nwhat made the figures that named an author:")
	for _, k := range sortedByCount(tools) {
		fmt.Printf("  %5d  %s\n", tools[k], k)
	}
	// A figure Author that is in three papers or more of different authors
	// is a program's or a template's, not a person's: printing those is how
	// the placeholder list grows. Anything rarer is somebody, and is only
	// counted.
	common := map[string]int{}
	rare := 0
	for who, papers := range strangers {
		if len(papers) >= 3 {
			common[who] = len(papers)
		} else {
			rare++
		}
	}
	fmt.Printf("\nfigure Authors that are none of the paper's authors: %d seen in fewer than three papers, and these in more:\n", rare)
	for _, k := range sortedByCount(common) {
		fmt.Printf("  %5d  %s\n", common[k], maskText(k))
	}
}

func isAuthor(who string, authors []string) bool {
	s, _ := readNames(authors, nil)
	return len(s.find(who)) > 0
}

// madeBy is the program a figure says made it, said as one of a few
// names.
func madeBy(fields []field, cl clue) string {
	var made []string
	for _, f := range fields {
		if f.where == cl.where && f.part == cl.part && (f.label == "made by" || f.label == "producer") {
			made = append(made, f.value)
		}
	}
	all := strings.ToLower(strings.Join(made, " "))
	for _, p := range []string{"powerpoint", "word", "excel", "keynote", "pages", "illustrator", "inkscape", "visio", "libreoffice", "openoffice", "draw.io", "omnigraffle", "affinity", "canva", "figma", "google", "matplotlib", "origin", "matlab", "quartz", "acrobat", "foxit", "pdfcreator", "ghostscript", "cairo", "skia"} {
		if strings.Contains(all, p) {
			return p
		}
	}
	if all == "" {
		return "nothing says"
	}
	return "something else"
}

func sortedByCount(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return m[keys[i]] > m[keys[j]] || m[keys[i]] == m[keys[j]] && keys[i] < keys[j] })
	return keys
}
