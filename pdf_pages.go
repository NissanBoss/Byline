package main

// The page tree, and what each page draws from.

import (
	"fmt"
	"sort"
)

// Pages returns the page objects in reading order. It follows the page tree
// when there is one, and falls back to object order when there is not,
// because a file with a damaged tree is still worth checking.
func (d *Document) Pages() []Dict {
	if pages := d.pagesFromTree(); len(pages) > 0 {
		return pages
	}
	var refs []Ref
	for ref, obj := range d.Objects {
		if dict, ok := obj.(Dict); ok && dict["Type"] == Name("Page") {
			refs = append(refs, ref)
		}
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].Num < refs[j].Num })
	var out []Dict
	for _, r := range refs {
		out = append(out, d.Objects[r].(Dict))
	}
	if len(out) > 0 {
		d.note("the page tree could not be followed, so pages are numbered by object order")
	}
	return out
}

func (d *Document) catalog() Dict {
	for _, t := range d.trailersNewestFirst() {
		if root, ok := d.Resolve(t["Root"]).(Dict); ok {
			return root
		}
	}
	for _, obj := range d.Objects {
		if dict, ok := obj.(Dict); ok && dict["Type"] == Name("Catalog") {
			return dict
		}
	}
	return nil
}

// trailersNewestFirst is every trailer the file has, the index streams'
// included, the one in force first.
func (d *Document) trailersNewestFirst() []Dict {
	var out []Dict
	for i := len(d.Revisions) - 1; i >= 0; i-- {
		if d.Revisions[i].Trailer != nil {
			out = append(out, d.Revisions[i].Trailer)
		}
	}
	for i := len(d.Trailers) - 1; i >= 0; i-- {
		out = append(out, d.Trailers[i])
	}
	return out
}

func (d *Document) pagesFromTree() []Dict {
	root := d.catalog()
	if root == nil {
		return nil
	}
	var out []Dict
	var walk func(node any, depth int)
	walk = func(node any, depth int) {
		if depth > 32 {
			return // a page tree this deep is a loop
		}
		dict, ok := d.Resolve(node).(Dict)
		if !ok {
			return
		}
		switch dict["Type"] {
		case Name("Page"):
			out = append(out, dict)
		default:
			kids, _ := d.Resolve(dict["Kids"]).(Array)
			for _, kid := range kids {
				walk(kid, depth+1)
			}
		}
	}
	walk(root["Pages"], 0)
	return out
}

// content gathers a page's drawing instructions. A page may hold several
// streams, and the spec says to treat them as one, joined in order.
func (d *Document) content(pg Dict) ([]byte, error) {
	var streams []*Stream
	switch c := d.Resolve(pg["Contents"]).(type) {
	case *Stream:
		streams = append(streams, c)
	case Array:
		for _, item := range c {
			if s, ok := d.Resolve(item).(*Stream); ok {
				streams = append(streams, s)
			}
		}
	case nil:
		return nil, nil // a blank page
	default:
		return nil, fmt.Errorf("this page has no content stream this reader understands")
	}

	var joined []byte
	for _, s := range streams {
		data, err := d.Decoded(s)
		if err != nil {
			return joined, err
		}
		joined = append(joined, data...)
		joined = append(joined, '\n')
	}
	return joined, nil
}

// resources finds the resource dictionary a page draws from. It can sit on
// the page itself or on any ancestor in the page tree, and a reader that
// only looked at the page would fail to resolve half the forms in a
// document produced by a word processor.
func (d *Document) resources(pg Dict) Dict {
	node := pg
	for range 32 {
		if res, ok := d.Resolve(node["Resources"]).(Dict); ok {
			return res
		}
		parent, ok := d.Resolve(node["Parent"]).(Dict)
		if !ok {
			break
		}
		node = parent
	}
	return nil
}

// drawnOn says which page first draws each picture and form, following
// forms inside forms, so a figure can be called "on page 3" rather than by
// an object number nobody can look up.
func (d *Document) drawnOn(pages []Dict) map[Ref]int {
	out := map[Ref]int{}
	var visit func(res Dict, page, depth int)
	visit = func(res Dict, page, depth int) {
		if depth > 12 || res == nil {
			return
		}
		xs, _ := d.Resolve(res["XObject"]).(Dict)
		for _, v := range xs {
			ref, ok := v.(Ref)
			if !ok {
				continue
			}
			if _, seen := out[ref]; seen {
				continue
			}
			out[ref] = page
			if s, ok := d.Resolve(ref).(*Stream); ok && s.Dict["Subtype"] == Name("Form") {
				inner, _ := d.Resolve(s.Dict["Resources"]).(Dict)
				visit(inner, page, depth+1)
			}
		}
	}
	for i, pg := range pages {
		visit(d.resources(pg), i+1, 0)
	}
	return out
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
