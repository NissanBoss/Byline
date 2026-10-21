"""Download recent papers from arXiv, with their authors, for the calibration.

    python3 testdata/arxiv.py OUT N [START]

arXiv's papers are not anonymous, which is what makes them useful here:
the authors are listed beside each one, so whether a figure names one of
them can be checked. The papers are read in the CI and thrown away with
the runner; only counts are printed, never a name.

arXiv asks for one request every three seconds and one connection at a
time, from export.arxiv.org. That is what this does.
"""
import json
import os
import sys
import time
import urllib.request
import xml.etree.ElementTree as ET

UA = {"User-Agent": "byline-calibration/1 (counts only)"}
CATS = "cat:cs.LG+OR+cat:cs.CV+OR+cat:cs.CL+OR+cat:cs.AI+OR+cat:cs.CR+OR+cat:cs.RO+OR+cat:cs.SE+OR+cat:cs.HC+OR+cat:cs.IR+OR+cat:stat.ML"
NS = {"a": "http://www.w3.org/2005/Atom"}


def get(url, limit=80 << 20):
    req = urllib.request.Request(url, headers=UA)
    with urllib.request.urlopen(req, timeout=120) as r:
        return r.read(limit + 1)


def main():
    out, want = sys.argv[1], int(sys.argv[2])
    start = int(sys.argv[3]) if len(sys.argv) > 3 else 0
    os.makedirs(out, exist_ok=True)
    papers = []
    at = start
    while len(papers) < want:
        n = min(200, want - len(papers))
        feed = get(f"https://export.arxiv.org/api/query?search_query={CATS}&sortBy=submittedDate&sortOrder=descending&start={at}&max_results={n}")
        entries = ET.fromstring(feed).findall("a:entry", NS)
        if not entries:
            break
        for e in entries:
            pid = e.find("a:id", NS).text.rsplit("/", 1)[-1]
            authors = [a.find("a:name", NS).text for a in e.findall("a:author", NS)]
            cat = e.find("{http://arxiv.org/schemas/atom}primary_category")
            papers.append({"id": pid, "authors": authors, "category": cat.get("term") if cat is not None else ""})
        at += n
        time.sleep(3)
    read = 0
    with open(os.path.join(out, "papers.jsonl"), "w") as index:
        for p in papers[:want]:
            time.sleep(3)
            try:
                body = get(f"https://export.arxiv.org/pdf/{p['id']}")
            except Exception as err:
                print("could not fetch one:", type(err).__name__)
                continue
            if not body.startswith(b"%PDF") or len(body) > 80 << 20:
                continue
            name = p["id"].replace("/", "_") + ".pdf"
            with open(os.path.join(out, name), "wb") as f:
                f.write(body)
            p["file"] = name
            index.write(json.dumps(p) + "\n")
            read += 1
    print(f"{read} papers of {len(papers)} listed")


main()
