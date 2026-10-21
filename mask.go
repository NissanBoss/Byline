package main

// Masking, the way Accent and Earmark do it: first and last letter of every word,
// punctuation kept, so a name still reads as the shape of a name.

import "strings"

func maskWord(s string) string {
	r := []rune(s)
	switch {
	case len(r) == 0:
		return ""
	case len(r) <= 2:
		return string(r[0]) + strings.Repeat("*", len(r)-1)
	}
	return string(r[0]) + strings.Repeat("*", len(r)-2) + string(r[len(r)-1])
}

func maskText(s string) string {
	return maskRuns(s, 1, isWordRune)
}

// maskLink masks an address the way maskText masks a name, but leaves the
// words that say what kind of place it is, github and drive and edu, so
// the owner of the link can still tell which one it is.
func maskLink(s string) string {
	return maskRuns(s, 2, func(r rune) bool { return r != '_' && isWordRune(r) })
}

// serviceWords are the parts of an address that say what kind of place it
// is and nothing about whose: they stay readable in a masked link.
var serviceWords = map[string]bool{
	"github": true, "gitlab": true, "gist": true, "bitbucket": true, "codeberg": true, "huggingface": true,
	"wandb": true, "kaggle": true, "twitter": true, "medium": true, "youtube": true, "replicate": true,
	"google": true, "drive": true, "docs": true, "colab": true, "research": true, "dropbox": true,
	"overleaf": true, "project": true, "read": true, "onedrive": true, "live": true, "sharepoint": true,
	"notion": true, "site": true, "sites": true, "view": true, "scholar": true, "citations": true,
	"orcid": true, "researchgate": true, "profile": true, "linkedin": true, "dblp": true, "pid": true,
	"semanticscholar": true, "author": true, "people": true, "person": true, "staff": true, "users": true,
	"user": true, "faculty": true, "home": true, "www": true, "https": true, "http": true, "com": true,
	"org": true, "net": true, "edu": true, "ac": true, "io": true, "co": true, "uk": true, "de": true,
	"datasets": true, "spaces": true, "file": true, "folders": true, "open": true, "osf": true,
	"git": true, "substack": true, "box": true, "pages": true, "blob": true, "tree": true, "main": true, "master": true,
}

// labelWords are this program's own, put around what a file says. Masking
// them along with the rest is what turned Earmark's labels into n**e.
var labelWords = map[string]bool{
	"name": true, "the": true, "of": true, "in": true, "a": true,
}

func maskRuns(s string, from int, inWord func(rune) bool) string {
	var b strings.Builder
	var word []rune
	flush := func() {
		if len(word) > 0 {
			lower := strings.ToLower(string(word))
			if len(word) >= from && !(from > 1 && serviceWords[lower]) && !labelWords[lower] {
				b.WriteString(maskWord(string(word)))
			} else {
				b.WriteString(string(word))
			}
			word = word[:0]
		}
	}
	for _, r := range s {
		if inWord(r) {
			word = append(word, r)
			continue
		}
		flush()
		b.WriteRune(r)
	}
	flush()
	return b.String()
}

func isWordRune(r rune) bool {
	return r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r > 0x7f
}
