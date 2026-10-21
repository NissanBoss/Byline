# Byline

Whether a paper you are about to send for anonymous review still names its
authors: in the properties pdfTeX copies from every figure, in what a camera
wrote into a photo, in comments, links and attachments, in the properties an
earlier save left behind, and in the supplementary material that goes with
it.

A byline is the line under a headline that says who wrote the piece. A
paper sent for double-blind review is not supposed to have one, and many
still do, in places nobody reads.

```
EXAMPLE
```

That is the paper and the supplement the test lab builds with pdfLaTeX,
LibreOffice, matplotlib, exiftool and git, cut short; every finding is
followed by what to do about it, left out here after the first.
`byline --name "Grace Hopper" --name "Caroline Herschel" pdftex.pdf supplement.zip`
printed it.

## The problem

Most of computer science decides what to publish by double-blind review.
NeurIPS 2025 received 21,575 papers and AAAI 2026 nearly 29,000, each read
by reviewers who are not supposed to know whose it is. Venues desk-reject
papers that give their authors away, and every author kit says the same
thing: anonymise the text, and clear the PDF's metadata.

The metadata people clear is the paper's own: the Author and Title a
properties dialog shows, the ones `exiftool` and `pdfinfo` print. The ones
that give them away are elsewhere.

- **Every PDF figure brings its own properties into the paper.** pdfTeX,
  which is what pdfLaTeX and Overleaf run, copies the document properties of
  each PDF it includes into the paper, under an entry called PTEX.InfoDict,
  and it does so unless told not to. PowerPoint and Word write the name of
  whoever is signed in to Office into every PDF they save, and LibreOffice
  writes the document's author. The architecture diagram drawn in
  PowerPoint and saved as a PDF puts its author's name inside the
  anonymous paper, where no viewer shows it and no checker reads it.
  CALIBRATION-FIGURES
- **A photo goes in with what the camera wrote.** pdfTeX, LuaTeX and
  XeTeX all put a JPEG into the paper byte for byte: the Artist the camera
  was set up with, its serial number, where it was taken, the by-line an
  editor added.
- **Clearing the properties does not always clear them.** `exiftool
  -all= paper.pdf` appends a save to the file and leaves the earlier
  properties in it: `exiftool -PDF-update:all=` puts them back, as
  exiftool's own documentation says. Rewriting the file drops them.
- **A comment keeps the name of whoever left it**, as the PDF reader it
  was made in calls them, which is usually the account it runs under.
- **The supplement names its authors in the ordinary course of things.**
  A package says who wrote it, a licence says whose it is, a `.git` folder
  has the remote and every commit's author, `tar` writes the login of
  whoever packed it against every file, and a zip made by a Mac's Finder
  keeps, in `__MACOSX`, the address each file was downloaded from. A Mac's
  own `tar` keeps that address too, even with `COPYFILE_DISABLE=1` and
  `--no-mac-metadata`, which is what every answer about `._` files says to
  use.

None of these is in what anybody reads before submitting. Byline reads all
of them.

## What already exists

- [acmsigsoft/submission-checker](https://github.com/acmsigsoft/submission-checker),
  used by ACM SIGSOFT conferences, checks a PDF's own Author and Creator
  fields, emails on the first page and phrases like "our previous work".
  It reads the document's properties and nothing under them: a figure's
  are never looked at.
- [arxiv_latex_cleaner](https://github.com/google-research/arxiv-latex-cleaner),
  from Google Research, cleans the LaTeX sources before they go to arXiv:
  comments, unused files, the `\todo`s. It never sees the compiled PDF.
- [pdfprivacy](https://ctan.org/pkg/pdfprivacy) is a LaTeX package that
  stops pdfTeX writing the PTEX entries, figures' properties included. It
  works, for whoever knows to load it; the lab below checks it does.
- [ExifTool](https://exiftool.org) prints and clears a PDF's properties,
  the document's own. Clearing them appends a save, and the old ones stay
  in the file.
- [dbr-check](https://github.com/abramhindle/dbr-check) and
  [DeskReject Guard](https://github.com/KrishivSIyer/deskreject-gaurd) read
  the text and the pictures as they are shown: an email on the first page,
  a university's crest. Neither opens what the file keeps.
- [Anonymous GitHub](https://github.com/tdurieux/anonymous_github) serves a
  repository for review without its owner. It is the answer to one of the
  findings here, not a check.
- The study [You Have Been LaTeXpOsEd](https://arxiv.org/abs/2510.03761)
  went through a hundred thousand arXiv source packages and found GPS in
  pictures, credentials and comments in the sources. It is about what the
  sources leak once posted, not about a paper under review.

Nothing opens each figure inside a compiled paper and says whose name it
carries, or takes your names and says everywhere in the paper and the
supplement they are.

## What it reads

```bash
byline paper.pdf
byline paper.pdf supplement.zip code/
byline --name "Jane Doe" --name jdoe --name jane.doe@uni.example paper.pdf
byline --names authors.txt paper.pdf supplement.zip
```

**In a PDF**, everything it keeps text in:

- the document's properties and XMP metadata, the ones in force and the
  ones earlier saves left in the file;
- the properties pdfTeX and LuaTeX copied from each included figure, and
  the file name and the clock they kept with them;
- the metadata a figure brought with it, and the whole copy of its own
  file Illustrator keeps inside a PDF it saves;
- EXIF, IPTC and XMP in every JPEG and JPEG 2000 picture: artist,
  copyright, camera owner, by-line, serial numbers, position;
- comments and highlights, and the name on each, the deleted ones a later
  save left behind included;
- where every link goes, and the paths of links to files on the machine
  it was made on;
- attached files, read as files in their own right;
- bookmarks, form fields, and every other string in the file;
- the words on every page, decoded through each font's own tables, so that
  a name set in Computer Modern is found as a name. What follows the
  References heading is kept apart, up to an appendix.

**In the supplement**, a folder, a zip or a tarball, and the PDFs, Office
documents and pictures in it: the authors in pyproject.toml, setup.py,
package.json, Cargo.toml, DESCRIPTION, CITATION.cff and .zenodo.json; the
holder in a licence; a `.git` folder's settings and reflog, the history
itself left to [Airing](https://github.com/NissanBoss/Airing); the creator,
last saver and commenters of a Word, PowerPoint or Excel file; the login
`tar` wrote against each file; the attributes a Mac kept in `__MACOSX` or
in a tar's headers; and in every text file, addresses, home folders and
links to accounts. Libraries in `node_modules`, a virtual environment or a
`third_party` folder are their authors', and only the names given are
looked for there.

What it finds comes at four levels.

- **YOU** is one of the names given with `--name` or `--names`, found. A
  name is looked for in the forms files write it in: as given, surname
  first, with initials, with the affiliation's number stuck to it, run
  together as a login is, with accents or without. Never looser: a surname
  on its own would find everybody who shares it.
- **NAMES** is somebody named where a person goes: an Author, a comment's
  author, a camera's Artist, a committer; and an email address or a home
  folder anywhere. Without names given, this is how a figure's author is
  reported.
- **POINTS** is a lead: a link to an account or a university page, a
  document shared from a drive, which shows its owner to whoever opens it,
  the place a photo was taken.
- **NOTE** is the rest worth knowing: the time zone a figure's clock was
  set to, a camera's serial number, a name of yours in the references.

The exit code is 1 when something is at NAMES or above, 0 when not, 2 when
it was run wrong and 3 when nothing could be read, which is not the same as
nothing found.

## How it was checked

**The lab.** `testdata/tools.sh` makes papers the way people make them, and
`tools.yml` checks each in GitHub Actions for what must be in it and what
must not:

LAB

**The calibration.** arXiv's papers are not anonymous, which is what makes
them useful: their authors are listed beside them, so whether a figure
names one of them can be checked. `calibrate.yml` reads recent papers in
computer science and statistics, with their authors given and without.

CALIBRATION

**What the calibration taught**, all of it in the code:

LESSONS

## What it cannot do

- **It does not judge the writing.** A paper can give its authors away by
  what it says: "in our previous work [12]", the dataset only one lab has,
  the style. Language models have become good at guessing authors from the
  text alone, as
  [Large Language Models Threaten Double-blind Review](https://arxiv.org/abs/2608.05157)
  measures. None of that is in a file's metadata, and none of it is looked
  at here.
- **It does not read pictures as pictures.** A university's crest in a
  slide, a name in a screenshot, a watermark: telling those needs OCR,
  which is a guess. Only what is written into a picture's metadata is read.
- **It does not read the LaTeX sources.** Comments in the .tex, a `\author`
  left in, a name in a file nobody uses: that is what arxiv_latex_cleaner
  is for. Byline reads what the reviewers get.
- **It does not know who you are without being told.** An Author in a
  figure may be a co-author or somebody whose figure was reused. Give your
  names with `--name` and it says which are yours.
- **Some files cannot be read.** An encrypted PDF, words set in a font
  that draws pictures instead of letters, a figure in a format it does not
  decode: each is said, and a report with a gap in it never calls itself
  clear. The old binary Office formats, .doc and .ppt, are not read, and
  neither are the maker notes in which some cameras keep their owner's name.
- **The git history is not read.** Only a `.git` folder's settings and
  reflog are. Every commit's author is in the objects, and
  [Airing](https://github.com/NissanBoss/Airing) reads those.

## It only reads

- **Nothing is written.** Not a report file, not a cleaned copy. There are
  tests that read the source and fail the build if a call that writes,
  deletes or renames anything appears in it.
- **Nothing is looked up or sent.** No name is searched for anywhere, no
  link is followed. There is no network code in it, and a test fails the
  build if any appears. It does not run other programs either.
- **What it finds is masked.** Names, addresses and links are cut to their
  first and last letters, so the report can be pasted somewhere; the home
  folder is shown as `~`. `--reveal` shows them.
- **A gap is not a pass.** A file it could not read, a font it could not
  decode, an archive entry that is encrypted: each is said, first.
- **Doing it right is not a finding.** "Microsoft Office User" and
  "Anonymous Author(s)" are not names. A link to anonymous.4open.science,
  openreview.net or a library's repository is not a lead. Citing your own
  earlier work in the third person, as venues ask, is a note and not a
  slip.
- **Exact, not statistical.** A name is found when it is there, in one of
  the forms listed above. Nothing is a likelihood.

## Running it

Download the package for your system from the Releases page, unpack it,
and run the binary. There is nothing to install.

```bash
byline --name "Jane Doe" paper.pdf supplement.zip
```

```bash
byline --reveal --names authors.txt paper.pdf
```

Windows marks anything that came from the internet, so the first run may
bring up a blue "Windows protected your PC" box. Right click the zip,
Properties, tick Unblock at the bottom, and then extract it. Building it
yourself avoids the question.

## Building

Needs [Go](https://go.dev) and nothing else. No dependencies at all.

```bash
go build -o byline .
```

```bash
go test ./...
```

```bash
sh testdata/tools.sh /tmp/tools
BYLINE_TOOLS=/tmp/tools go test -run TestMadeByTools -v
```

The tests build the PDFs they check object by object, the photos with
their EXIF and the archives a Mac makes, so what each case is made of is
in the test and not in a file nobody can read. The lab needs TeX Live,
LibreOffice, exiftool and qpdf; `tools.yml` installs them.

## Licence

MIT. Do what you like with it.
