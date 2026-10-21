# Changes

## v1.1

What the first run of v1 over real files on a real machine found, the
same day:

- A text file with "%PDF-" near its top, a test that builds PDFs, was
  taken for a PDF and reported as one that could not be read. A file is a
  PDF now when it starts like one, or is called one.
- A README was read as a package's manifest, and a line of prose that
  began "authors:" came out as an author. READMEs are read as text.
- A path with a one-letter user in it was reported as a home folder.

## v1

The first release. Give it the paper you are about to send for anonymous
review, and the supplementary material with it, and it says whether
anything in them still names the authors.

- In the PDF: the document's properties and XMP, the ones an earlier save
  left behind, the properties pdfTeX and LuaTeX copy from every included
  figure, the metadata and the Illustrator copy a figure brings along,
  EXIF, IPTC and XMP in every JPEG, comments and their authors, links and
  links to files, attachments, bookmarks, form fields, every other string,
  and the words on every page, decoded through each font's own tables.
- In the supplement: package author fields, licences, a `.git` folder's
  settings and reflog, Office documents' creators and commenters, the
  login `tar` writes against each file, and the download addresses a Mac
  keeps in `__MACOSX` and in a tar's headers.
- `--name` and `--names` look for your names everywhere, in the forms files
  write them in.
- Checked against papers built with pdfLaTeX, LuaLaTeX and XeLaTeX,
  figures from LibreOffice and matplotlib, photos tagged by exiftool,
  properties cleared by exiftool and rewritten by qpdf, and archives made
  by a Mac's Finder and tar; and calibrated on recent arXiv papers.
