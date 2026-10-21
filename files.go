package main

// Which files are read, how containers are opened, and how paths are shown.

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// Folders that are other people's code, or a machine's leftovers. A
// supplement with its environment in it would otherwise report every
// library author in site-packages as a name.
var skipDirs = map[string]bool{
	"node_modules": true, "bower_components": true, ".venv": true, "venv": true, "env": true,
	"__pycache__": true, "site-packages": true, ".tox": true, ".mypy_cache": true, ".pytest_cache": true,
	".gradle": true, ".idea": true, ".cache": true, "dist-packages": true,
}

// Data that is read as nothing: weights, arrays, recordings. A supplement
// is mostly these by size, and none of them keeps a name where this could
// look for one.
var dataExt = map[string]bool{
	".npy": true, ".npz": true, ".pt": true, ".pth": true, ".ckpt": true, ".bin": true, ".h5": true,
	".hdf5": true, ".pkl": true, ".pickle": true, ".safetensors": true, ".onnx": true, ".tfrecord": true,
	".parquet": true, ".feather": true, ".arrow": true, ".mat": true, ".wav": true, ".mp3": true,
	".flac": true, ".mp4": true, ".mov": true, ".avi": true, ".mkv": true, ".so": true, ".dll": true,
	".exe": true, ".o": true, ".a": true, ".pyc": true, ".class": true, ".jar": true, ".whl": true,
	".gguf": true, ".joblib": true, ".lmdb": true, ".mdb": true, ".db": true, ".sqlite": true,
}

// borrowedDirs hold code somebody else wrote and the supplement carries:
// their authors' names in it are theirs, and only the names given are
// looked for there.
var borrowedDir = regexp.MustCompile(`(?i)(?:^|/)(?:third[_-]?party|vendor|vendored|external|extern|deps|_deps|submodules?|libs?/[^/]+/(?:LICENSE|COPYING))(?:/|$)`)

// filesIn lists what is under the paths given. A file named on the command
// line is always opened, whatever it is called.
func filesIn(paths []string, show shower) (files []string, skipped map[string]int, err error) {
	skipped = map[string]int{}
	for _, root := range paths {
		abs, err := filepath.Abs(root)
		if err != nil {
			return nil, nil, err
		}
		info, err := os.Stat(abs)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: no such file or folder, or it cannot be opened", show.path(abs))
		}
		if !info.IsDir() {
			files = append(files, abs)
			continue
		}
		err = filepath.WalkDir(abs, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if p == abs {
					return nil
				}
				if d.Name() == ".git" {
					// Only the repository's own records of who it is:
					// the history is a question of its own, and Airing's.
					for _, f := range gitFiles {
						if st, err := os.Stat(filepath.Join(p, f)); err == nil && st.Mode().IsRegular() {
							files = append(files, filepath.Join(p, f))
						}
					}
					skipped[".git"]++
					return fs.SkipDir
				}
				if skipDirs[d.Name()] {
					skipped[d.Name()]++
					return fs.SkipDir
				}
				return nil
			}
			if !d.Type().IsRegular() {
				return nil
			}
			if dataExt[strings.ToLower(filepath.Ext(p))] {
				skipped["data"]++
				return nil
			}
			files = append(files, p)
			return nil
		})
		if err != nil {
			return nil, nil, err
		}
	}
	sort.Strings(files)
	return files, skipped, nil
}

// gitFiles are what is read of a .git folder.
var gitFiles = []string{"config", "logs/HEAD", "FETCH_HEAD", "description"}

// isText says whether a file reads as text: no NUL in the first stretch
// of it, and valid UTF-8 there, give or take a character cut in two at the
// end of the stretch.
func isText(b []byte) bool {
	head := b[:min(len(b), 8192)]
	if bytes.IndexByte(head, 0) >= 0 {
		return false
	}
	for cut := 0; cut < 4 && len(head) > 0; cut++ {
		if utf8.Valid(head) {
			return true
		}
		if len(head) == len(b) {
			return false
		}
		head = head[:len(head)-1]
	}
	return false
}

// entry is a file inside a container, with what the container says about
// who put it there.
type entry struct {
	name  string
	data  []byte
	owner string // the login a tar wrote against it
	group string
	// xattrs are the extended attributes a Mac's tar keeps in the entry's
	// header, the ones a zip from Finder keeps in __MACOSX.
	xattrs map[string][]byte
	skip   bool
}

const maxEntry = 128 << 20

func zipEntries(data []byte) (entries []entry, comment string, gaps []string, err error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, "", nil, err
	}
	for _, f := range r.File {
		if f.FileInfo().IsDir() || dataExt[strings.ToLower(path.Ext(f.Name))] {
			continue
		}
		if f.Flags&1 != 0 {
			gaps = append(gaps, f.Name+" is encrypted")
			continue
		}
		if f.UncompressedSize64 > maxEntry {
			gaps = append(gaps, fmt.Sprintf("%s is %s, more than this reads inside a container", f.Name, size(int64(f.UncompressedSize64))))
			continue
		}
		rc, err := f.Open()
		if err != nil {
			gaps = append(gaps, f.Name+" could not be opened")
			continue
		}
		body, err := io.ReadAll(io.LimitReader(rc, maxEntry+1))
		rc.Close()
		if err != nil {
			gaps = append(gaps, f.Name+" could not be read")
			continue
		}
		entries = append(entries, entry{name: f.Name, data: body})
	}
	return entries, r.Comment, gaps, nil
}

func tarEntries(data []byte) ([]entry, []string, error) {
	var r io.Reader = bytes.NewReader(data)
	if bytes.HasPrefix(data, []byte{0x1f, 0x8b}) {
		z, err := gzip.NewReader(r)
		if err != nil {
			return nil, nil, err
		}
		inner, err := io.ReadAll(io.LimitReader(z, 2<<30))
		if err != nil {
			return nil, nil, err
		}
		if !looksTar(inner) {
			return []entry{{name: "(inside the gzip)", data: inner}}, nil, nil
		}
		r = bytes.NewReader(inner)
	}
	tr := tar.NewReader(r)
	var out []entry
	var gaps []string
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return out, append(gaps, "the archive ends early"), nil
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		e := entry{name: h.Name, owner: h.Uname, group: h.Gname}
		for k, v := range h.PAXRecords {
			// libarchive, which is the tar on a Mac, writes them base64;
			// GNU tar's SCHILY form keeps the bytes as they are.
			if name, ok := strings.CutPrefix(k, "LIBARCHIVE.xattr."); ok {
				if b, err := base64.StdEncoding.DecodeString(v); err == nil {
					if e.xattrs == nil {
						e.xattrs = map[string][]byte{}
					}
					e.xattrs[name] = b
				}
			} else if name, ok := strings.CutPrefix(k, "SCHILY.xattr."); ok {
				if e.xattrs == nil {
					e.xattrs = map[string][]byte{}
				}
				e.xattrs[name] = []byte(v)
			}
		}
		if dataExt[strings.ToLower(path.Ext(h.Name))] {
			e.skip = true // nothing to read, but its owner still counts
			out = append(out, e)
			continue
		}
		if h.Size > maxEntry {
			gaps = append(gaps, fmt.Sprintf("%s is %s, more than this reads inside a container", h.Name, size(h.Size)))
			continue
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			gaps = append(gaps, h.Name+" could not be read")
			continue
		}
		e.data = body
		out = append(out, e)
	}
	return out, gaps, nil
}

func looksTar(b []byte) bool {
	return len(b) > 262 && string(b[257:262]) == "ustar"
}

// shower writes paths the way the report shows them: the home folder as
// ~, which keeps the one part of a path that names a person out of a
// report that might get pasted somewhere. --reveal shows it.
type shower struct {
	home   string
	reveal bool
}

func (s shower) path(p string) string {
	if s.reveal || s.home == "" {
		return p
	}
	rel, err := filepath.Rel(s.home, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return p
	}
	return "~/" + filepath.ToSlash(rel)
}

// inner is a path that came out of a file, with the user name in any home
// folder in it masked, unless --reveal is given.
func (s shower) inner(p string) string {
	if s.reveal {
		return p
	}
	return homeRe.ReplaceAllStringFunc(p, func(m string) string {
		sub := homeRe.FindStringSubmatch(m)
		if len(sub) < 2 || sub[1] == "" || machineUsers[strings.ToLower(sub[1])] {
			return m
		}
		return strings.TrimSuffix(m, sub[1]) + maskText(sub[1])
	})
}

// text is a value that came out of a file, masked to the first and last
// letters of each word unless --reveal is given: enough for the owner to
// know their own, not enough for anybody else to read.
func (s shower) text(t string) string {
	// The long dash is not printed by anything in this series, not even
	// when a file has one.
	t = strings.ReplaceAll(t, string(rune(0x2014)), "-")
	t = strings.Join(strings.Fields(t), " ")
	if runes(t) > 90 {
		t = string([]rune(t)[:87]) + "..."
	}
	if s.reveal {
		return t
	}
	return maskText(t)
}
