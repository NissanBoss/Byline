package main

// What a camera or an editor writes into a picture: EXIF, IPTC and XMP in a
// JPEG, the text chunks of a PNG, the tags of a TIFF.
//
// This matters inside a paper because pdfTeX does not re-encode a JPEG. It
// puts the file in byte for byte, with everything the camera and every
// editor since wrote into it, and nothing that shows the page shows those.
// A PNG is a different story: pdfTeX unpacks its pixels and writes them out
// again, and its text chunks are left behind.

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"unicode/utf16"
)

// pictureValue is one thing a picture says, as a field without its place.
type pictureValue struct {
	label  string
	value  string
	author bool
	level  level // set with fixed, for what is not text to judge
	fixed  bool
}

func jpegValues(data []byte) []pictureValue {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return nil
	}
	var out []pictureValue
	for at := 2; at+4 <= len(data); {
		if data[at] != 0xFF {
			break
		}
		marker := data[at+1]
		if marker == 0xD8 || marker >= 0xD0 && marker <= 0xD7 || marker == 0x01 || marker == 0xFF {
			at++
			continue
		}
		if marker == 0xDA || marker == 0xD9 {
			break // the picture itself starts; metadata does not come after
		}
		n := int(binary.BigEndian.Uint16(data[at+2:]))
		if n < 2 || at+2+n > len(data) {
			break
		}
		body := data[at+4 : at+2+n]
		switch {
		case marker == 0xE1 && bytes.HasPrefix(body, []byte("Exif\x00")):
			out = append(out, exifValues(body[6:])...)
		case marker == 0xE1 && bytes.HasPrefix(body, []byte("http://ns.adobe.com/xap/1.0/\x00")):
			for _, v := range xmpValues(string(body[29:])) {
				out = append(out, pictureValue{label: v.label, value: v.value, author: v.author})
			}
		case marker == 0xE1 && bytes.HasPrefix(body, []byte("http://ns.adobe.com/xmp/extension/\x00")):
			if len(body) > 75 {
				for _, v := range xmpValues(string(body[75:])) {
					out = append(out, pictureValue{label: v.label, value: v.value, author: v.author})
				}
			}
		case marker == 0xED && bytes.HasPrefix(body, []byte("Photoshop 3.0\x00")):
			out = append(out, iptcValues(body[14:])...)
		case marker == 0xFE:
			out = append(out, pictureValue{label: "comment", value: string(bytes.TrimRight(body, "\x00"))})
		}
		at += 2 + n
	}
	return out
}

// exifValues reads the TIFF structure EXIF is made of.
func exifValues(b []byte) []pictureValue {
	t, err := parseTIFF(b)
	if err != nil {
		return nil
	}
	var out []pictureValue
	add := func(label, v string, author bool) {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, pictureValue{label: label, value: v, author: author})
		}
	}
	read := func(d ifd) {
		add("artist", d.text(0x013B), true)
		add("copyright", d.text(0x8298), true)
		add("description", d.text(0x010E), false)
		add("made by", d.text(0x0131), false)
		add("author", xpText(d, 0x9C9D), true)
		add("title", xpText(d, 0x9C9B), false)
		add("comment", xpText(d, 0x9C9C), false)
		add("keywords", xpText(d, 0x9C9E), false)
		add("subject", xpText(d, 0x9C9F), false)
	}
	read(t.ifd0)
	if ex := t.sub(t.ifd0, 0x8769); ex != nil {
		add("camera owner", ex.text(0xA430), true)
		add("comment", userComment(ex[0x9286]), false)
		if s := ex.text(0xA431); s != "" && strings.Trim(s, "0 ") != "" {
			out = append(out, pictureValue{label: "camera serial number", value: s, level: tell, fixed: true})
		}
		if s := ex.text(0xA435); s != "" && strings.Trim(s, "0 ") != "" {
			out = append(out, pictureValue{label: "lens serial number", value: s, level: tell, fixed: true})
		}
		if z := ex.text(0x9011); z != "" && z != "+00:00" && z != "-00:00" {
			out = append(out, pictureValue{label: "clock", value: "UTC" + z, level: tell, fixed: true})
		}
	}
	if gps := t.sub(t.ifd0, 0x8825); gps != nil {
		lat, okLat := gpsDegrees(gps, 2, 1)
		lon, okLon := gpsDegrees(gps, 4, 3)
		if okLat && okLon && (lat != 0 || lon != 0) {
			out = append(out, pictureValue{label: "where it was taken", value: fmt.Sprintf("%.5f, %.5f", lat, lon), level: points, fixed: true})
		}
	}
	return out
}

// xpText reads the tags Windows Explorer writes, which are UTF-16 kept as
// bytes.
func xpText(d ifd, tag uint16) string {
	e, ok := d[tag]
	if !ok || len(e.value) < 2 {
		return ""
	}
	units := make([]uint16, 0, len(e.value)/2)
	for i := 0; i+1 < len(e.value); i += 2 {
		u := uint16(e.value[i]) | uint16(e.value[i+1])<<8
		if u == 0 {
			break
		}
		units = append(units, u)
	}
	return string(utf16.Decode(units))
}

// userComment skips the eight bytes that say which character set follows.
func userComment(e tiffEntry) string {
	if len(e.value) <= 8 {
		return ""
	}
	head, body := string(e.value[:8]), e.value[8:]
	if strings.HasPrefix(head, "UNICODE") {
		return utf16Text(body, e.order == binary.BigEndian)
	}
	return strings.TrimRight(string(body), "\x00 ")
}

func gpsDegrees(d ifd, tag, ref uint16) (float64, bool) {
	e, ok := d[tag]
	if !ok || e.kind != 5 || len(e.value) < 24 {
		return 0, false
	}
	part := func(i int) float64 {
		n := e.order.Uint32(e.value[8*i:])
		den := e.order.Uint32(e.value[8*i+4:])
		if den == 0 {
			return 0
		}
		return float64(n) / float64(den)
	}
	v := part(0) + part(1)/60 + part(2)/3600
	if r := d.text(ref); r == "S" || r == "W" {
		v = -v
	}
	return v, true
}

// iptcValues reads the IPTC block inside Photoshop's resources, the one
// news agencies use: its By-line field is, word for word, the name of
// whoever made the picture.
func iptcValues(b []byte) []pictureValue {
	var out []pictureValue
	for at := 0; at+12 <= len(b); {
		if string(b[at:at+4]) != "8BIM" {
			break
		}
		id := binary.BigEndian.Uint16(b[at+4:])
		nameLen := int(b[at+6])
		skip := 1 + nameLen
		if skip%2 == 1 {
			skip++
		}
		p := at + 6 + skip
		if p+4 > len(b) {
			break
		}
		size := int(binary.BigEndian.Uint32(b[p:]))
		p += 4
		if size < 0 || p+size > len(b) {
			break
		}
		if id == 0x0404 {
			out = append(out, iptcRecords(b[p:p+size])...)
		}
		at = p + size
		if at%2 == 1 {
			at++
		}
	}
	return out
}

var iptcFields = map[byte]struct {
	label  string
	author bool
}{
	5: {"object name", false}, 80: {"by-line", true}, 85: {"by-line title", false}, 110: {"credit", true},
	115: {"source", false}, 116: {"copyright", true}, 118: {"contact", true}, 120: {"caption", false},
	122: {"caption writer", true}, 90: {"city", false}, 92: {"place", false}, 95: {"region", false}, 101: {"country", false},
}

func iptcRecords(b []byte) []pictureValue {
	var out []pictureValue
	for at := 0; at+5 <= len(b); {
		if b[at] != 0x1C {
			break
		}
		rec, ds := b[at+1], b[at+2]
		n := int(binary.BigEndian.Uint16(b[at+3:]))
		at += 5
		if n&0x8000 != 0 || at+n > len(b) {
			break
		}
		if f, ok := iptcFields[ds]; ok && rec == 2 {
			if v := strings.TrimSpace(string(b[at : at+n])); v != "" {
				out = append(out, pictureValue{label: f.label, value: v, author: f.author})
			}
		}
		at += n
	}
	return out
}

// pngValues reads a PNG's text chunks and its EXIF, which only matter
// outside a PDF: inside one the pixels were copied and the chunks were not.
func pngValues(data []byte) []pictureValue {
	if !bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
		return nil
	}
	var out []pictureValue
	for at := 8; at+12 <= len(data); {
		n := int(binary.BigEndian.Uint32(data[at:]))
		kindOf := string(data[at+4 : at+8])
		if n < 0 || at+12+n > len(data) {
			break
		}
		body := data[at+8 : at+8+n]
		switch kindOf {
		case "tEXt":
			if k, v, ok := bytes.Cut(body, []byte{0}); ok {
				out = append(out, pngText(string(k), string(v))...)
			}
		case "zTXt":
			if k, v, ok := bytes.Cut(body, []byte{0}); ok && len(v) > 1 {
				if r, err := zlib.NewReader(bytes.NewReader(v[1:])); err == nil {
					text, _ := io.ReadAll(io.LimitReader(r, 4<<20))
					out = append(out, pngText(string(k), string(text))...)
				}
			}
		case "iTXt":
			parts := bytes.SplitN(body, []byte{0}, 3)
			if len(parts) == 3 && len(parts[1]) >= 2 {
				compressed := parts[1][0] == 1
				rest := parts[2]
				// language tag, then translated keyword, each ending in 0
				for range 2 {
					if i := bytes.IndexByte(rest, 0); i >= 0 {
						rest = rest[i+1:]
					}
				}
				if compressed {
					if r, err := zlib.NewReader(bytes.NewReader(rest)); err == nil {
						rest, _ = io.ReadAll(io.LimitReader(r, 4<<20))
					}
				}
				out = append(out, pngText(string(parts[0]), string(rest))...)
			}
		case "eXIf":
			out = append(out, exifValues(body)...)
		case "IEND":
			return out
		}
		at += 12 + n
	}
	return out
}

func pngText(key, value string) []pictureValue {
	if key == "XML:com.adobe.xmp" {
		var out []pictureValue
		for _, v := range xmpValues(value) {
			out = append(out, pictureValue{label: v.label, value: v.value, author: v.author})
		}
		return out
	}
	author := false
	switch strings.ToLower(key) {
	case "author", "copyright", "artist", "creator":
		author = true
	}
	return []pictureValue{{label: strings.ToLower(key), value: value, author: author}}
}

func tiffValues(data []byte) []pictureValue {
	if len(data) < 8 || string(data[:2]) != "II" && string(data[:2]) != "MM" {
		return nil
	}
	return exifValues(data)
}
