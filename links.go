package main

// Where a link goes, and whether that place is somebody's.
//
// A paper links to a great deal that is not its authors': the libraries it
// used, the datasets, the papers it cites. Only some addresses lead to a
// person: an account on a code host, a personal page, a page at a
// university, and the documents shared from a drive, which show their
// owner to whoever opens them. Those are what is reported, and as leads
// rather than names, because an account linked to may be anybody's.

import (
	"regexp"
	"strings"
)

type lead struct {
	url string
	why string
}

// accountHosts are the sites whose first path part is an account.
var accountHosts = map[string]string{
	"github.com":         "a GitHub account",
	"gitlab.com":         "a GitLab account",
	"bitbucket.org":      "a Bitbucket account",
	"codeberg.org":       "a Codeberg account",
	"huggingface.co":     "a Hugging Face account",
	"wandb.ai":           "a Weights & Biases account",
	"kaggle.com":         "a Kaggle account",
	"x.com":              "an X account",
	"twitter.com":        "an X account",
	"medium.com":         "a Medium account",
	"youtube.com":        "a YouTube channel",
	"gist.github.com":    "a GitHub account",
	"replicate.com":      "a Replicate account",
	"paperswithcode.com": "",
}

// notAccounts are the first path parts of those sites that are pages of
// the site itself.
var notAccounts = map[string]bool{
	"about": true, "features": true, "topics": true, "explore": true, "marketplace": true, "sponsors": true,
	"orgs": true, "settings": true, "login": true, "join": true, "search": true, "trending": true,
	"collections": true, "pricing": true, "docs": true, "blog": true, "datasets": true, "spaces": true,
	"models": true, "papers": true, "tasks": true, "learn": true, "competitions": true, "code": true,
	"discussions": true, "watch": true, "results": true, "playlist": true, "c": true, "channel": true,
	"intent": true, "share": true, "home": true, "i": true, "hashtag": true, "en": true, "site": true,
	"apps": true, "enterprise": true, "security": true, "customer-stories": true, "readme": true,
	"resources": true, "solutions": true, "team": true, "contact": true, "new": true, "notifications": true,
	"api": true, "raw": true, "assets": true, "static": true, "dashboard": true, "organizations": true,
}

// libraries are the accounts every paper in machine learning links to:
// the frameworks and the big labs. Listing them would bury the one link
// that matters under thirty that do not.
var libraries = map[string]bool{
	"pytorch": true, "tensorflow": true, "huggingface": true, "google": true, "google-research": true,
	"google-deepmind": true, "deepmind": true, "facebookresearch": true, "openai": true, "microsoft": true,
	"nvidia": true, "nvlabs": true, "keras-team": true, "scikit-learn": true, "numpy": true, "scipy": true,
	"pandas-dev": true, "matplotlib": true, "jax-ml": true, "google-deepmind-research": true,
	"apple": true, "meta-llama": true, "mistralai": true, "anthropics": true, "allenai": true,
	"eleutherai": true, "bigscience-workshop": true, "lm-sys": true, "vllm-project": true, "ggml-org": true,
	"ggerganov": true, "langchain-ai": true, "ray-project": true, "lightning-ai": true, "pytorchlightning": true,
	"open-mmlab": true, "ultralytics": true, "openmmlab": true, "salesforce": true, "ibm": true,
	"amazon-science": true, "aws": true, "awslabs": true, "intel": true, "qualcomm": true, "baidu": true,
	"alibaba": true, "tencent": true, "thudm": true, "qwenlm": true, "deepseek-ai": true, "stanfordnlp": true,
	"stanford-crfm": true, "explosion": true, "nltk": true, "opencv": true, "dmlc": true, "rapidsai": true,
	"datasets": true, "openreview": true, "arxiv": true, "paperswithcode": true, "github": true, "actions": true,
	"jupyter": true, "python": true, "rust-lang": true, "golang": true, "nodejs": true, "kubernetes": true,
	"docker": true, "apache": true, "mozilla": true, "chromium": true, "torch": true, "gpt2": true,
	"bert-base-uncased": true, "sentence-transformers": true, "stabilityai": true, "runwayml": true,
	"compvis": true, "laion": true, "mlfoundations": true, "meta": true, "facebook": true, "nlp": true,
}

// Shared documents show their owner's name and picture to whoever opens
// them, so a link to one is a link to its owner.
var sharedHosts = []struct{ host, path, why string }{
	{"drive.google.com", "", "a file shared from Google Drive, which shows its owner"},
	{"docs.google.com", "", "a Google document, which shows its owner"},
	{"colab.research.google.com", "/drive/", "a Colab notebook, which shows its owner"},
	{"colab.research.google.com", "/github/", ""},
	{"dropbox.com", "", "a Dropbox link, which shows whose it is"},
	{"www.dropbox.com", "", "a Dropbox link, which shows whose it is"},
	{"overleaf.com", "/project/", "an Overleaf project, which shows its owner"},
	{"www.overleaf.com", "/project/", "an Overleaf project, which shows its owner"},
	{"www.overleaf.com", "/read/", "an Overleaf project, which shows its owner"},
	{"1drv.ms", "", "a OneDrive link, which shows its owner"},
	{"onedrive.live.com", "", "a OneDrive link, which shows its owner"},
	{"sharepoint.com", "", "a SharePoint link, which names the organisation"},
	{"osf.io", "", "an OSF project, which lists its contributors"},
	{"notion.site", "", "a Notion page, which shows its owner"},
	{"box.com", "", "a Box link, which shows whose it is"},
	{"wetransfer.com", "", ""},
	{"sites.google.com", "/view/", "a personal Google site"},
	{"scholar.google.com", "/citations", "a Google Scholar profile"},
	{"orcid.org", "", "an ORCID profile"},
	{"researchgate.net", "/profile/", "a ResearchGate profile"},
	{"www.researchgate.net", "/profile/", "a ResearchGate profile"},
	{"linkedin.com", "/in/", "a LinkedIn profile"},
	{"www.linkedin.com", "/in/", "a LinkedIn profile"},
	{"dblp.org", "/pid/", "a dblp profile"},
	{"semanticscholar.org", "/author/", "a Semantic Scholar profile"},
	{"www.semanticscholar.org", "/author/", "a Semantic Scholar profile"},
}

// Addresses that exist to keep a review anonymous, and the venues'
// own sites. A paper that links to these is doing what it was asked.
var anonymousHosts = map[string]bool{
	"anonymous.4open.science": true, "anonymous.4open.sciences": true, "anonymous-github.xyz": true,
	"openreview.net": true, "arxiv.org": true, "doi.org": true, "dx.doi.org": true, "aclanthology.org": true,
	"proceedings.neurips.cc": true, "papers.nips.cc": true, "proceedings.mlr.press": true,
	"dl.acm.org": true, "ieeexplore.ieee.org": true, "link.springer.com": true, "creativecommons.org": true,
}

var institutionRe = regexp.MustCompile(`(?i)(?:\.edu|\.edu\.[a-z]{2}|\.ac\.[a-z]{2}|\.uni-[a-z-]+\.de|\.tu-[a-z]+\.de|\.univ-[a-z-]+\.fr|\.ethz\.ch|\.epfl\.ch|\.mpg\.de|\.inria\.fr|\.cnrs\.fr|\.kit\.edu|\.tum\.de|\.uva\.nl|\.tudelft\.nl|\.kth\.se|\.ox\.ac\.uk|\.cam\.ac\.uk|\.utoronto\.ca|\.ubc\.ca|\.mcgill\.ca)$|^(?:[a-z0-9-]+\.)*(?:uni-[a-z-]+|univ-[a-z-]+|tu-[a-z]+)\.[a-z]{2,3}$`)

// personalPath is a page of a person on any site: ~name, /people/name,
// /~name.
var personalPath = regexp.MustCompile(`(?i)^/(?:~[^/]+|people/[^/]+|person/[^/]+|staff/[^/]+|members?/[^/]+|homes?/[^/]+|users?/[^/]+|faculty/[^/]+)`)

func links(f field) []lead {
	var out []lead
	var candidates []string
	if f.kind == kindLink {
		candidates = append(candidates, f.value)
	} else if strings.Contains(f.value, "http") || strings.Contains(f.value, "www.") {
		candidates = urlRe.FindAllString(f.value, -1)
	}
	for _, c := range candidates {
		if l, ok := leadOf(strings.TrimRight(c, ".,;:)]}>")); ok {
			out = append(out, l)
		}
	}
	return out
}

// address is the two parts of a link this needs. net/url would do it, and
// importing anything under net is what the promise tests forbid, so that
// no part of this program could ever open a connection.
type address struct {
	full, path string
}

func parseAddress(s string) (address, bool) {
	i := strings.Index(s, "://")
	if i <= 0 {
		return address{}, false
	}
	rest := s[i+3:]
	end := strings.IndexAny(rest, "/?#")
	host, path := rest, "/"
	if end >= 0 {
		host = rest[:end]
		path = rest[end:]
		if q := strings.IndexAny(path, "?#"); q >= 0 {
			path = path[:q]
			if path == "" {
				path = "/"
			}
		}
	}
	if at := strings.LastIndexByte(host, '@'); at >= 0 {
		host = host[at+1:]
	}
	if c := strings.LastIndexByte(host, ':'); c >= 0 && !strings.Contains(host[c:], "]") {
		host = host[:c]
	}
	if host == "" || !strings.Contains(host, ".") {
		return address{}, false
	}
	return address{full: strings.ToLower(host), path: path}, true
}

func leadOf(raw string) (lead, bool) {
	s := raw
	if strings.HasPrefix(strings.ToLower(s), "www.") {
		s = "https://" + s
	}
	u, ok := parseAddress(s)
	if !ok {
		return lead{}, false
	}
	full := u.full
	host := strings.TrimPrefix(full, "www.")
	if anonymousHosts[host] || anonymousHosts[full] {
		return lead{}, false
	}
	parts := strings.Split(strings.Trim(u.path, "/"), "/")
	for _, sh := range sharedHosts {
		if (full == sh.host || host == sh.host || strings.HasSuffix(full, "."+sh.host)) && strings.HasPrefix(u.path, sh.path) {
			if sh.why == "" {
				return lead{}, false
			}
			return lead{raw, sh.why}, true
		}
	}
	if why, ok := accountHosts[host]; ok {
		if why == "" || len(parts) == 0 || parts[0] == "" {
			return lead{}, false
		}
		account := strings.ToLower(strings.TrimPrefix(parts[0], "@"))
		if host == "huggingface.co" && (account == "datasets" || account == "spaces") && len(parts) > 1 {
			account = strings.ToLower(parts[1])
		}
		if notAccounts[account] || libraries[account] {
			return lead{}, false
		}
		return lead{raw, why}, true
	}
	switch {
	case strings.HasSuffix(host, ".github.io"):
		account := strings.TrimSuffix(host, ".github.io")
		if libraries[account] {
			return lead{}, false
		}
		return lead{raw, "a GitHub Pages site, which is a GitHub account's"}, true
	case strings.HasSuffix(host, ".gitlab.io"):
		return lead{raw, "a GitLab Pages site, which is a GitLab account's"}, true
	case strings.HasSuffix(host, ".substack.com") || strings.HasSuffix(host, ".medium.com"):
		return lead{raw, "somebody's blog"}, true
	case institutionRe.MatchString(host):
		if personalPath.MatchString(u.path) {
			return lead{raw, "a personal page at a university"}, true
		}
		return lead{raw, "an address at a university"}, true
	case personalPath.MatchString(u.path) && strings.HasPrefix(u.path, "/~"):
		return lead{raw, "a personal page"}, true
	}
	return lead{}, false
}
