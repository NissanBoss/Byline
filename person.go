package main

// What a program puts in an author field when nobody told it a name.
//
// An author field holds a person by definition, so whatever is in one is
// reported, with one exception: the values that are a program's and not a
// person's. Reporting "Microsoft Office User" as somebody's name would be
// telling people off for having done it right, and a tool that does that is
// a tool people stop reading.

import "strings"

// placeholders are whole values.
var placeholders = map[string]bool{
	"unknown": true, "anonymous": true, "anonymous author": true, "anonymous authors": true,
	"anonymous author(s)": true, "anonymous submission": true, "anonymized": true, "anonymised": true,
	"author": true, "authors": true, "author name": true, "author names": true, "your name": true,
	"name": true, "user": true, "admin": true, "administrator": true, "owner": true, "default": true,
	"windows user": true, "microsoft office user": true, "office user": true, "microsoft office": true,
	"mac": true, "root": true, "n/a": true, "na": true, "none": true, "null": true, "nil": true,
	"-": true, "--": true, "?": true, ".": true, "x": true, "test": true, "guest": true, "pc": true,
	"ubuntu": true, "runner": true, "home": true, "student": true, "the author": true, "the authors": true,
	"blind review": true, "blinded": true, "for review": true, "submission": true, "paper id": true,
	"confidential": true, "draft": true, "untitled": true, "lenovo": true, "dell": true, "hp": true,
	"asus": true, "acer": true, "toshiba": true, "msi": true, "microsoft": true, "valued customer": true,
	"registered user": true, "preferred customer": true, "authorized user": true,
	// Found by the calibration in the figures of many papers by different
	// people: what some exporter or generator writes as the author.
	"walnut exporter": true, "openai": true, "chatgpt": true,
}

// programWords are names of the programs that make figures and of the
// companies behind them. A value made only of these is a program signing
// its work.
var programWords = map[string]bool{
	"matplotlib": true, "python": true, "r": true, "matlab": true, "mathworks": true, "mathematica": true,
	"wolfram": true, "gnuplot": true, "inkscape": true, "adobe": true, "illustrator": true, "photoshop": true,
	"acrobat": true, "tex": true, "latex": true, "pdftex": true, "luatex": true, "xetex": true, "tikz": true,
	"pgf": true, "pgfplots": true, "origin": true, "originlab": true, "graphpad": true, "prism": true,
	"excel": true, "powerpoint": true, "word": true, "keynote": true, "pages": true, "numbers": true,
	"draw.io": true, "drawio": true, "diagrams.net": true, "visio": true, "google": true, "chrome": true,
	"skia": true, "cairo": true, "plotly": true, "bokeh": true, "seaborn": true, "ggplot2": true, "qt": true,
	"ghostscript": true, "preview": true, "quartz": true, "apple": true, "microsoft": true, "office": true,
	"libreoffice": true, "openoffice": true, "corel": true, "coreldraw": true, "affinity": true,
	"designer": true, "figma": true, "canva": true, "omnigraffle": true, "omni": true, "group": true,
	"sketch": true, "lucidchart": true, "biorender": true, "chemdraw": true, "pymol": true, "chimera": true,
	"paraview": true, "blender": true, "veusz": true, "xfig": true, "dia": true, "graphviz": true,
	"dot": true, "mermaid": true, "jupyter": true, "notebook": true, "tableau": true, "spss": true,
	"stata": true, "sas": true, "minitab": true, "jmp": true, "igor": true, "kaleidagraph": true,
	"sigmaplot": true, "statistica": true, "maple": true, "octave": true, "scilab": true, "julia": true,
	"makie": true, "plots": true, "vega": true, "altair": true, "d3": true, "echarts": true, "foxit": true,
	"pdf": true, "pdfcreator": true, "nitro": true, "smallpdf": true, "ilovepdf": true, "cutepdf": true,
	"primopdf": true, "printer": true, "print": true, "to": true, "created": true, "by": true, "with": true,
	"version": true, "software": true, "inc": true, "inc.": true, "corporation": true, "corp": true,
	"systems": true, "incorporated": true, "ltd": true, "gmbh": true, "pro": true, "for": true, "365": true,
	"mac os x": true, "macos": true, "windows": true, "linux": true, "xpdf": true, "poppler": true,
	"mupdf": true, "pypdf": true, "reportlab": true, "itext": true, "pdfbox": true, "wkhtmltopdf": true,
}

// nobody says whether an author field's value is a placeholder or a
// program's own name.
func nobody(v string) bool {
	l := strings.ToLower(strings.Join(strings.Fields(v), " "))
	if placeholders[l] || strings.Contains(l, "anonym") {
		return true
	}
	words := strings.FieldsFunc(l, func(r rune) bool {
		return r == ' ' || r == ',' || r == '(' || r == ')' || r == '/' || r == ';' || r == '|' || r == '_'
	})
	if len(words) == 0 {
		return true
	}
	for _, w := range words {
		w = strings.Trim(w, ".:-")
		if w == "" || programWords[w] || isNumber(w) || strings.Trim(w, "®™©") != w && programWords[strings.Trim(w, "®™©")] {
			continue
		}
		return false
	}
	return true
}

func isNumber(w string) bool {
	if w == "" {
		return false
	}
	for _, r := range w {
		if (r < '0' || r > '9') && r != '.' && r != 'v' {
			return false
		}
	}
	return true
}
