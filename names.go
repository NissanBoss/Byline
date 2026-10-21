package main

// The names given on the command line, and finding them.
//
// Given names make the answer exact. Without them, all this program can say
// is that an author field holds somebody; with them, it says that a figure
// holds you. A name is looked for in the forms a file writes it in: as
// given, surname first as a bibliography does, with initials, and run
// together the way a login is. Nothing looser than that: a surname on its
// own would find every other author who shares it.

import (
	"bufio"
	"os"
	"strings"
	"unicode"
)

type wanted struct {
	given string // as typed
	forms []form
}

// form is one way a name is written, folded, with a space at both ends.
// loose forms are compared with digits taken for gaps, as a person's name
// is; the rest keep their digits, as a login does.
type form struct {
	text  string
	loose bool
}

type nameSet struct {
	list []wanted
}

type hit struct {
	name string // the name given
	text string // what the file has, in its own spelling
}

// Words that make a phrase an institution or a group rather than a person,
// whose name is only ever looked for as written.
var placeWords = map[string]bool{
	"university": true, "universit": true, "universidad": true, "universite": true, "universitat": true,
	"universita": true, "institute": true, "institut": true, "instituto": true, "college": true,
	"school": true, "department": true, "dept": true, "lab": true, "laboratory": true, "labs": true,
	"group": true, "center": true, "centre": true, "inc": true, "corp": true, "ltd": true, "gmbh": true,
	"academy": true, "hospital": true, "foundation": true, "research": true, "of": true, "for": true,
	"the": true, "and": true, "de": true, "der": true, "für": true, "du": true, "la": true, "team": true,
	"ai": true, "technologies": true, "technology": true, "company": true, "llc": true, "sa": true,
}

func readNames(given []string, files []string) (*nameSet, error) {
	all := append([]string{}, given...)
	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			all = append(all, line)
		}
		f.Close()
		if err := sc.Err(); err != nil {
			return nil, err
		}
	}
	if len(all) == 0 {
		return nil, nil
	}
	s := &nameSet{}
	for _, n := range all {
		if w, ok := formsOf(n); ok {
			s.list = append(s.list, w)
		}
	}
	return s, nil
}

func formsOf(n string) (wanted, bool) {
	w := wanted{given: strings.TrimSpace(n)}
	f := strings.TrimSpace(foldText(n).text)
	if len([]rune(f)) < 3 {
		return w, false
	}
	add := func(s string, loose bool) {
		s = " " + strings.TrimSpace(s) + " "
		for _, have := range w.forms {
			if have.text == s && have.loose == loose {
				return
			}
		}
		w.forms = append(w.forms, form{s, loose})
	}
	words := strings.Fields(f)
	if strings.Contains(n, "@") || len(words) < 2 || len(words) > 5 || placeLike(words) || !personLike(n) {
		// An address, a login, an institution: as written. A login written
		// with dots or underscores, jane.doe, folds to the same words as
		// the name does, and is found either way.
		add(f, false)
		if len(words) > 1 && !placeLike(words) && !strings.Contains(n, "@") {
			add(strings.Join(words, ""), false)
		}
		return w, true
	}
	words = strings.Fields(foldName(n).text)
	if len(words) < 2 {
		add(f, false)
		return w, true
	}
	add(strings.Join(words, " "), true)
	first, last := words[0], words[len(words)-1]
	middle := words[1 : len(words)-1]
	initials := ""
	for _, m := range middle {
		initials += " " + initial(m)
	}
	add(first+" "+last, true)
	add(first+initials+" "+last, true)
	add(initial(first)+" "+last, true)
	add(initial(first)+initials+" "+last, true)
	add(last+" "+first, true)
	add(last+" "+initial(first), true)
	add(last+" "+first+initials, true)
	add(first+last, false) // the login and the address made of the whole name
	add(initial(first)+last, false)
	add(last+first, false)
	// Two surnames, as in Spanish and Portuguese: the first one alone is how
	// most papers cite the person.
	if len(words) >= 3 {
		add(first+" "+words[1], true)
		add(initial(first)+" "+words[1], true)
		add(words[1]+" "+first, true)
	}
	return w, true
}

// placeLike says whether a phrase is an institution's name.
func placeLike(words []string) bool {
	for _, w := range words {
		if placeWords[w] {
			return true
		}
	}
	return false
}

// personLike says whether a phrase is written the way a name is: every
// word capitalised or an initial.
func personLike(n string) bool {
	for _, w := range strings.Fields(n) {
		r := []rune(strings.Trim(w, ".,"))
		if len(r) == 0 {
			continue
		}
		if !unicode.IsUpper(r[0]) && !(len(r) <= 3 && strings.Contains(w, ".")) {
			return false
		}
	}
	return true
}

// find returns the names found in a value. Shorter forms are only tried
// when the longer ones failed, so a full name is reported as the full name.
func (s *nameSet) find(v string) []hit {
	if s == nil || len(s.list) == 0 {
		return nil
	}
	strict, loose := foldText(v), foldName(v)
	var out []hit
	for _, w := range s.list {
		for _, fm := range w.forms {
			f := strict
			if fm.loose {
				f = loose
			}
			i := strings.Index(f.text, fm.text)
			if i < 0 {
				continue
			}
			text := f.original(i+1, i+len(fm.text)-1)
			if text == "" {
				text = strings.TrimSpace(fm.text)
			}
			out = append(out, hit{name: w.given, text: text})
			break
		}
	}
	return out
}

func (s *nameSet) empty() bool { return s == nil || len(s.list) == 0 }

func initial(w string) string {
	r := []rune(w)
	if len(r) == 0 {
		return ""
	}
	return string(r[0])
}
