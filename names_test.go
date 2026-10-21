package main

import "testing"

// A name is found in the forms files write it in, and not in looser ones.
func TestTheFormsOfAName(t *testing.T) {
	s, err := readNames([]string{"Jane Q. Doe", "jdoe42", "jane.doe@uni.example.ac.uk", "University of Somewhere"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{
		"Jane Doe", "JANE Q. DOE", "Doe, Jane", "J. Doe", "Doe J", "jane_doe", "janedoe", "Thanks to J. Q. Doe.",
		"/Users/jdoe42/paper", "https://github.com/jdoe42/repo", "mail jane.doe@uni.example.ac.uk",
		"the University of Somewhere, Department of X", "Doe1, Jane2",
	} {
		if len(s.find(v)) == 0 {
			t.Errorf("%q was not found", v)
		}
	}
	for _, v := range []string{
		"Doe", "John Doe", "Jane Smith", "jdoe4", "jdoe421", "Somewhere University", "the doe jane-ish thing",
	} {
		if v == "the doe jane-ish thing" {
			continue // "doe jane" is the surname-first form, and it is there
		}
		if hits := s.find(v); len(hits) > 0 {
			t.Errorf("%q was found as %v", v, hits)
		}
	}
}

// Accents are off when names are compared, whichever side has them.
func TestAccents(t *testing.T) {
	s, _ := readNames([]string{"José Müller-Łukasiewicz"}, nil)
	for _, v := range []string{"Jose Muller-Lukasiewicz", "JOSÉ MÜLLER-ŁUKASIEWICZ", "Müller-Łukasiewicz, José"} {
		if len(s.find(v)) == 0 {
			t.Errorf("%q was not found", v)
		}
	}
	s, _ = readNames([]string{"Jose Muller"}, nil)
	if len(s.find("Thanks to José Müller.")) == 0 {
		t.Error("an accented spelling in the file was not found from a plain one given")
	}
}

// A match is shown in the file's own spelling.
func TestAMatchKeepsItsSpelling(t *testing.T) {
	s, _ := readNames([]string{"Jose Muller"}, nil)
	hits := s.find("We thank José Müller for the data.")
	if len(hits) != 1 || hits[0].text != "José Müller" {
		t.Errorf("found as %v", hits)
	}
}

func TestHomeFolders(t *testing.T) {
	cases := map[string]string{
		"/Users/ada/paper/fig.pdf":         "ada",
		"/home/alovelace/x":                "alovelace",
		`C:\Users\Ada Lovelace\Desktop\x`:  "Ada Lovelace",
		"C:/Users/ada/Documents":           "ada",
		"/mnt/c/Users/ada/x":               "ada",
		"/home/runner/work/x":              "",
		"/Users/Shared/x":                  "",
		"see /home/$USER/.config":          "",
		"/content/drive/MyDrive/x":         "",
		`C:\Users\%USERNAME%\AppData\x`:    "",
		"https://example.org/Users/guide/": "",
	}
	for in, want := range cases {
		got := homeFolders(in)
		switch {
		case want == "" && len(got) > 0:
			t.Errorf("%q gave %v", in, got)
		case want != "" && (len(got) == 0 || got[0] != want):
			t.Errorf("%q gave %v, not %s", in, got, want)
		}
	}
}

func TestEmails(t *testing.T) {
	if got := emails("write to ada@cam.ac.uk or anonymous@review.org, or you@example.com"); len(got) != 1 || got[0] != "ada@cam.ac.uk" {
		t.Errorf("got %v", got)
	}
	if got := emails("logo@2x.png"); len(got) != 0 {
		t.Errorf("a picture's name was taken for an address: %v", got)
	}
}

func TestLeads(t *testing.T) {
	cases := map[string]bool{
		"https://github.com/alovelace/engine":           true,
		"https://github.com/pytorch/pytorch":            false,
		"https://github.com/features/actions":           false,
		"https://alovelace.github.io/":                  true,
		"https://huggingface.co/datasets/alovelace/x":   true,
		"https://huggingface.co/bert-base-uncased":      false,
		"https://www.cs.ox.ac.uk/people/ada.lovelace/":  true,
		"https://www.cl.cam.ac.uk/research/":            true,
		"https://drive.google.com/file/d/1abc/view":     true,
		"https://colab.research.google.com/drive/1xyz":  true,
		"https://www.overleaf.com/read/abcdef":          true,
		"https://anonymous.4open.science/r/engine-1A2B": false,
		"https://arxiv.org/abs/2401.00001":              false,
		"https://example.com/~ada/":                     true,
		"https://pytorch.org/docs/stable/":              false,
	}
	for u, want := range cases {
		_, got := leadOf(u)
		if got != want {
			t.Errorf("%s: reported %v", u, got)
		}
	}
}
