#!/bin/sh
# Papers and supplements made the way people make them, with the tools
# people use: figures exported by LibreOffice and matplotlib, a photo tagged
# by exiftool, papers compiled by pdfLaTeX, LuaLaTeX and XeLaTeX, the
# properties "cleared" by exiftool and rewritten by qpdf, a comment added
# by pypdf, a repository zipped and tarred. expect.tsv says what Byline
# must find in each, and what it must not.
#
#   sh testdata/tools.sh OUT          on Linux, with the packages tools.yml installs
#   sh testdata/tools.sh OUT mac      on a Mac, the Finder's zip and tar
#
# Every name in here is made up, or long dead.

set -e
OUT="$1"
mkdir -p "$OUT"
cd "$OUT"
: > expect.tsv

# expect FILE NAMES KIND LEVEL TEXT yes|no
expect() {
    printf '%s\t%s\t%s\t%s\t%s\t%s\n' "$1" "$2" "$3" "$4" "$5" "$6" >> expect.tsv
}

if [ "$2" = "mac" ]; then
    # Finder's Compress is ditto with these options. The attributes are set
    # the way Safari and the quarantine set them on a download.
    mkdir -p mac/data
    echo "a,b" > mac/data/results.csv
    WHERE=$(python3 -c 'import plistlib,sys; sys.stdout.write(plistlib.dumps(["https://mail.example-university.edu/attachment?id=4815","https://mail.example-university.edu/"], fmt=plistlib.FMT_BINARY).hex())')
    xattr -wx com.apple.metadata:kMDItemWhereFroms "$WHERE" mac/data/results.csv
    xattr -w com.apple.quarantine "0083;6720f000;Safari;" mac/data/results.csv
    (cd mac && ditto -c -k --sequesterRsrc --keepParent data ../finder.zip)
    expect finder.zip - link 1 example-university.edu yes
    expect finder.zip - archive 0 Safari yes
    # The Mac's own tar keeps the same attributes, unless told not to.
    (cd mac && tar -czf ../mac.tgz data)
    expect mac.tgz - link 1 example-university.edu yes
    (cd mac && COPYFILE_DISABLE=1 tar --no-mac-metadata -czf ../mac-clean.tgz data)
    expect mac-clean.tgz - link 1 example-university.edu no
    # zip from the command line keeps none of it.
    (cd mac && zip -qr ../cli.zip data)
    expect cli.zip - link 1 example-university.edu no
    exit 0
fi

# The figures. LibreOffice writes the document's author into the PDF it
# exports; matplotlib writes what it is given.
cat > fig-office.fodg <<'END'
<?xml version="1.0" encoding="UTF-8"?>
<office:document xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:meta="urn:oasis:names:tc:opendocument:xmlns:meta:1.0" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:draw="urn:oasis:names:tc:opendocument:xmlns:drawing:1.0" xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0" xmlns:svg="urn:oasis:names:tc:opendocument:xmlns:svg-compatible:1.0" office:version="1.3" office:mimetype="application/vnd.oasis.opendocument.graphics">
 <office:meta><meta:initial-creator>Ada Lovelace</meta:initial-creator><dc:creator>Ada Lovelace</dc:creator><dc:title>architecture</dc:title></office:meta>
 <office:body><office:drawing><draw:page draw:name="p1"><draw:rect svg:x="1cm" svg:y="1cm" svg:width="5cm" svg:height="3cm"><text:p>Encoder</text:p></draw:rect></draw:page></office:drawing></office:body>
</office:document>
END
soffice --headless --convert-to pdf fig-office.fodg >/dev/null 2>&1
python3 - <<'END'
import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
from PIL import Image, PngImagePlugin
fig = plt.figure(figsize=(2, 2))
plt.plot([1, 2, 3])
fig.savefig("fig-mpl.pdf", metadata={"Author": "Grace Hopper"})
fig.savefig("fig-plain.pdf")
Image.new("RGB", (64, 48), (200, 30, 30)).save("photo.jpg", quality=90)
info = PngImagePlugin.PngInfo()
info.add_text("Author", "Mary Somerville")
Image.new("RGB", (64, 48), (30, 30, 200)).save("chunk.png", pnginfo=info)
END
exiftool -q -overwrite_original -Artist="Caroline Herschel" -IPTC:By-line="Caroline Herschel" \
    -XMP-dc:Creator="Caroline Herschel" -SerialNumber=0451234567 \
    -GPSLatitude=51.4769 -GPSLatitudeRef=N -GPSLongitude=0.0005 -GPSLongitudeRef=W photo.jpg
exiftool -q -all= -o photo-clean.jpg photo.jpg

# paper PREAMBLE ENGINE NAME: one paper with every figure in it.
paper() {
    cat > "$3.tex" <<END
$1
\\documentclass{article}
\\usepackage{graphicx}
\\usepackage{hyperref}
$4
\\begin{document}
Byline's lab. The code is at \\href{https://github.com/alovelace/engine}{this address}.

\\includegraphics[width=3cm]{fig-office.pdf}
\\includegraphics[width=3cm]{fig-mpl.pdf}
\\includegraphics[width=3cm]{fig-plain.pdf}
\\includegraphics[width=3cm]{photo.jpg}
\\includegraphics[width=3cm]{photo-clean.jpg}
\\includegraphics[width=3cm]{chunk.png}

We thank Hedy Lamarr for the data.
\\end{document}
END
    "$2" -interaction=nonstopmode "$3.tex" >/dev/null 2>&1 || { echo "$2 failed on $3"; tail -20 "$3.log"; exit 1; }
}

paper "" pdflatex pdftex ""
expect pdftex.pdf - figure 2 "Grace Hopper" yes
expect pdftex.pdf - figure 2 "Ada Lovelace" yes
expect pdftex.pdf - figure 0 "figures with properties" yes
expect pdftex.pdf - picture 2 "Caroline Herschel" yes
expect pdftex.pdf - picture 1 "where it was taken" yes
expect pdftex.pdf - picture 0 "camera serial number" yes
expect pdftex.pdf - picture 2 "Mary Somerville" no
expect pdftex.pdf - link 1 "github.com/alovelace" yes
expect pdftex.pdf "Hedy Lamarr" text 3 "Hedy Lamarr" yes
expect pdftex.pdf "Grace Hopper" figure 3 "Grace Hopper" yes

paper "\\pdfsuppressptexinfo=-1" pdflatex pdftex-suppressed ""
expect pdftex-suppressed.pdf - figure 2 "Grace Hopper" no
expect pdftex-suppressed.pdf - figure 2 "Ada Lovelace" no
expect pdftex-suppressed.pdf - figure 0 "figures with properties" no

if kpsewhich pdfprivacy.sty >/dev/null 2>&1; then
    paper "" pdflatex pdfprivacy "\\usepackage[noptexdata]{pdfprivacy}"
    expect pdfprivacy.pdf - figure 2 "Grace Hopper" no
fi

paper "" lualatex luatex ""
expect luatex.pdf - figure 2 "Grace Hopper" yes
expect luatex.pdf - picture 2 "Caroline Herschel" yes
expect luatex.pdf "Hedy Lamarr" text 3 "Hedy Lamarr" yes

paper "" lualatex luatex-suppressed "\\pdfvariable suppressoptionalinfo 15"
expect luatex-suppressed.pdf - figure 2 "Grace Hopper" no

paper "" xelatex xetex ""
expect xetex.pdf - picture 2 "Caroline Herschel" yes
expect xetex.pdf "Hedy Lamarr" text 3 "Hedy Lamarr" yes

# The paper's own properties, set by hyperref, then "cleared" by exiftool,
# which appends a save and leaves the old properties in the file, then
# rewritten by qpdf, which leaves out what nothing refers to.
paper "" pdflatex hyperref "\\hypersetup{pdfauthor={Ada Lovelace},pdftitle={Engine}}"
expect hyperref.pdf - document 2 "Ada Lovelace" yes
cp hyperref.pdf cleared.pdf
exiftool -q -overwrite_original -all= cleared.pdf
expect cleared.pdf - document 2 "Ada Lovelace" no
expect cleared.pdf - earlier 2 "Ada Lovelace" yes
qpdf --linearize cleared.pdf rewritten.pdf
expect rewritten.pdf - earlier 2 "Ada Lovelace" no
expect rewritten.pdf - document 2 "Ada Lovelace" no

# A comment, as a PDF reader adds one.
python3 - <<'END'
from pypdf import PdfReader, PdfWriter
from pypdf.generic import DictionaryObject, NameObject, TextStringObject, ArrayObject, FloatObject
w = PdfWriter(clone_from="fig-plain.pdf")
note = DictionaryObject({
    NameObject("/Type"): NameObject("/Annot"),
    NameObject("/Subtype"): NameObject("/Text"),
    NameObject("/Rect"): ArrayObject([FloatObject(10), FloatObject(10), FloatObject(30), FloatObject(30)]),
    NameObject("/T"): TextStringObject("Emmy Noether"),
    NameObject("/Contents"): TextStringObject("is this the final version?"),
})
w.add_annotation(page_number=0, annotation=note)
w.write("commented.pdf")
END
expect commented.pdf - comment 2 "Emmy Noether" yes

# The supplementary material: a repository with its .git, zipped and
# tarred the usual ways, and a document from LibreOffice.
mkdir -p sup/engine
(
    cd sup/engine
    git init -q
    git config user.name "Ada Lovelace"
    git config user.email "ada@analytical.example.org"
    printf '[project]\nname = "engine"\nauthors = [{name = "Ada Lovelace", email = "ada@analytical.example.org"}]\n' > pyproject.toml
    printf 'MIT License\n\nCopyright (c) 2026 Ada Lovelace\n' > LICENSE
    printf "DATA = '/home/alovelace/data/bernoulli'\n" > train.py
    git add -A
    git commit -qm "first"
    git remote add origin git@github.com:alovelace/engine.git
)
(cd sup && zip -qr ../supplement.zip engine)
expect supplement.zip - manifest 2 "Ada Lovelace" yes
expect supplement.zip - git 2 "Ada Lovelace" yes
expect supplement.zip - link 1 "github.com/alovelace" yes
expect supplement.zip - file 2 "alovelace" yes
(cd sup && tar --owner=alovelace --group=analytical -czf ../supplement.tgz engine)
expect supplement.tgz - archive 2 "alovelace" yes
(cd sup && tar --owner=0 --group=0 --numeric-owner -czf ../supplement-clean.tgz engine)
expect supplement-clean.tgz - archive 2 "alovelace" no

cat > notes.fodt <<'END'
<?xml version="1.0" encoding="UTF-8"?>
<office:document xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:meta="urn:oasis:names:tc:opendocument:xmlns:meta:1.0" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0" office:version="1.3" office:mimetype="application/vnd.oasis.opendocument.text">
 <office:meta><meta:initial-creator>Charles Babbage</meta:initial-creator><dc:creator>Charles Babbage</dc:creator></office:meta>
 <office:body><office:text><text:p>Notes for the reviewers.</text:p></office:text></office:body>
</office:document>
END
soffice --headless --convert-to docx notes.fodt >/dev/null 2>&1
expect notes.docx - office 2 "Charles Babbage" yes

echo "made $(wc -l < expect.tsv) expectations in $OUT"
