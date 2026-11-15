// SPDX-License-Identifier: MIT

package nsd_test

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/nsd"
)

// The limits of an archive (SECURITY.md, "Limits"): each with its
// boundary and with what a refused load allocated.

// directoryOnly returns a ZIP archive that is a central directory of n
// headers and nothing else: every entry is an empty stored file named
// e<i>.txt at offset 0.
func directoryOnly(n int) []byte { return handBuilt(n) }

// handBuilt returns a ZIP archive with the given documents, stored
// without compression as d<i>.nsd or d<i>.nsdoc, followed by n entries
// that exist in the central directory only: empty stored files named
// e<i>.txt at offset 0. It is written by hand because it is built many
// times with 100 000 entries.
func handBuilt(n int, docs ...string) []byte {
	var b, dir bytes.Buffer
	header := func(name string, data []byte, offset int) {
		h := make([]byte, 46)
		copy(h, "PK\x01\x02")
		binary.LittleEndian.PutUint16(h[4:], 20) // version made by
		binary.LittleEndian.PutUint16(h[6:], 20) // version needed
		binary.LittleEndian.PutUint32(h[16:], crc32.ChecksumIEEE(data))
		binary.LittleEndian.PutUint32(h[20:], uint32(len(data)))
		binary.LittleEndian.PutUint32(h[24:], uint32(len(data)))
		binary.LittleEndian.PutUint16(h[28:], uint16(len(name)))
		binary.LittleEndian.PutUint32(h[42:], uint32(offset))
		dir.Write(h)
		dir.WriteString(name)
	}
	for i, d := range docs {
		name := fmt.Sprintf("d%d.nsd", i)
		if strings.HasPrefix(d, "<NSDoc") {
			name = fmt.Sprintf("d%d.nsdoc", i)
		}
		header(name, []byte(d), b.Len())
		local := make([]byte, 30)
		copy(local, "PK\x03\x04")
		binary.LittleEndian.PutUint16(local[4:], 20)
		binary.LittleEndian.PutUint32(local[14:], crc32.ChecksumIEEE([]byte(d)))
		binary.LittleEndian.PutUint32(local[18:], uint32(len(d)))
		binary.LittleEndian.PutUint32(local[22:], uint32(len(d)))
		binary.LittleEndian.PutUint16(local[26:], uint16(len(name)))
		b.Write(local)
		b.WriteString(name)
		b.WriteString(d)
	}
	for i := range n {
		header(fmt.Sprintf("e%d.txt", i), nil, 0)
	}
	offset := b.Len()
	b.Write(dir.Bytes())
	end := make([]byte, 22)
	copy(end, "PK\x05\x06")
	records := uint16(n + len(docs)) // the count modulo 65536, as the format has it
	binary.LittleEndian.PutUint16(end[8:], records)
	binary.LittleEndian.PutUint16(end[10:], records)
	binary.LittleEndian.PutUint32(end[12:], uint32(dir.Len()))
	binary.LittleEndian.PutUint32(end[16:], uint32(offset))
	b.Write(end)
	return b.Bytes()
}

// stored returns an archive with one entry that holds data without
// compression.
func stored(t testing.TB, name string, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// The number of entries is bounded before the directory of an archive
// is read, also for an archive inside an archive.
func TestArchiveEntryLimitBeforeTheDirectoryIsRead(t *testing.T) {
	for _, tc := range []struct {
		records int
		ok      bool
	}{{0, true}, {1, true}, {1023, true}, {1024, true}, {1025, false}, {100000, false}} {
		inner := directoryOnly(tc.records)
		for _, nested := range []bool{false, true} {
			data, want := inner, tc.ok
			if nested {
				// The archive that holds the other one is an entry itself.
				data, want = stored(t, "inner.zip", inner), tc.ok && tc.records < 1024
			}
			var p *nsd.Package
			var err error
			allocs := testing.AllocsPerRun(1, func() { p, err = nsd.ReadZIP(bytes.NewReader(data)) })
			switch {
			case want && (err != nil || len(p.Diagnostics()) != tc.records):
				t.Errorf("%d entries, nested %v: error %v", tc.records, nested, err)
			case !want && !errors.Is(err, model.ErrLimitExceeded):
				t.Errorf("%d entries, nested %v: error = %v, want ErrLimitExceeded", tc.records, nested, err)
			case !want && allocs > 200:
				// A refused archive costs the read of its bytes and the
				// error: nothing per entry.
				t.Errorf("%d entries, nested %v: %.0f allocations for a refused archive, budget 200", tc.records, nested, allocs)
			}
		}
	}
	// The limit counts the entries of nested archives together.
	half := directoryOnly(600)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range []string{"a.zip", "b.zip"} {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(half); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := nsd.ReadZIP(bytes.NewReader(buf.Bytes())); !errors.Is(err, model.ErrLimitExceeded) {
		t.Errorf("two nested archives of 600 entries: error = %v, want ErrLimitExceeded", err)
	}
	if _, err := nsd.ReadZIP(bytes.NewReader(buf.Bytes()), nsd.WithMaxArchiveEntries(1202)); err != nil {
		t.Errorf("1202 entries with a limit of 1202: %v", err)
	}
	if _, err := nsd.ReadZIP(bytes.NewReader(buf.Bytes()), nsd.WithMaxArchiveEntries(1201)); !errors.Is(err, model.ErrLimitExceeded) {
		t.Errorf("1202 entries with a limit of 1201: error = %v, want ErrLimitExceeded", err)
	}
}

// deflated returns content compressed with deflate.
func deflated(t testing.TB, content []byte) []byte {
	t.Helper()
	var compressed bytes.Buffer
	fw, err := flate.NewWriter(&compressed, flate.BestSpeed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := fw.Close(); err != nil {
		t.Fatal(err)
	}
	return compressed.Bytes()
}

// oneEntry returns a ZIP archive with a single deflated entry, a.nsd,
// whose central directory header declares the given compressed size: in the
// 32-bit field, or in the ZIP64 extra field when zip64 is set. The local
// header and the data are those of the content.
func oneEntry(content, compressed []byte, declared uint64, zip64 bool) []byte {
	const name = "a.nsd"
	crc := crc32.ChecksumIEEE(content)
	var b bytes.Buffer
	local := make([]byte, 30)
	copy(local, "PK\x03\x04")
	binary.LittleEndian.PutUint16(local[4:], 20)
	binary.LittleEndian.PutUint16(local[8:], 8) // deflate
	binary.LittleEndian.PutUint32(local[14:], crc)
	binary.LittleEndian.PutUint32(local[18:], uint32(len(compressed)))
	binary.LittleEndian.PutUint32(local[22:], uint32(len(content)))
	binary.LittleEndian.PutUint16(local[26:], uint16(len(name)))
	b.Write(local)
	b.WriteString(name)
	b.Write(compressed)

	dirOffset := b.Len()
	var extra []byte
	central := make([]byte, 46)
	copy(central, "PK\x01\x02")
	binary.LittleEndian.PutUint16(central[4:], 45)
	binary.LittleEndian.PutUint16(central[6:], 45)
	binary.LittleEndian.PutUint16(central[10:], 8)
	binary.LittleEndian.PutUint32(central[16:], crc)
	if zip64 {
		// Both sizes move to the extra field: uncompressed, then compressed.
		binary.LittleEndian.PutUint32(central[20:], 0xFFFFFFFF)
		binary.LittleEndian.PutUint32(central[24:], 0xFFFFFFFF)
		extra = make([]byte, 20)
		binary.LittleEndian.PutUint16(extra[0:], 1)
		binary.LittleEndian.PutUint16(extra[2:], 16)
		binary.LittleEndian.PutUint64(extra[4:], uint64(len(content)))
		binary.LittleEndian.PutUint64(extra[12:], declared)
	} else {
		binary.LittleEndian.PutUint32(central[20:], uint32(declared))
		binary.LittleEndian.PutUint32(central[24:], uint32(len(content)))
	}
	binary.LittleEndian.PutUint16(central[28:], uint16(len(name)))
	binary.LittleEndian.PutUint16(central[30:], uint16(len(extra)))
	b.Write(central)
	b.WriteString(name)
	b.Write(extra)
	dirSize := b.Len() - dirOffset

	end := make([]byte, 22)
	copy(end, "PK\x05\x06")
	binary.LittleEndian.PutUint16(end[8:], 1)
	binary.LittleEndian.PutUint16(end[10:], 1)
	binary.LittleEndian.PutUint32(end[12:], uint32(dirSize))
	binary.LittleEndian.PutUint32(end[16:], uint32(dirOffset))
	b.Write(end)
	return b.Bytes()
}

// The compression ratio is computed from a compressed size the archive
// can hold: a declared size beyond the archive is refused.
func TestCompressionRatioWithDeclaredSizes(t *testing.T) {
	content := append(append([]byte(nsOpen), bytes.Repeat([]byte(" "), 2<<20)...), "</NS>"...)
	compressed := deflated(t, content)
	actual := uint64(len(compressed))
	size := uint64(len(oneEntry(content, compressed, actual, false)))
	if ratio := uint64(len(content)) / size; ratio <= 200 {
		t.Fatalf("the fixture has a ratio of %d over its archive, want more than 200", ratio)
	}

	for _, tc := range []struct {
		name     string
		declared uint64
		zip64    bool
		want     error
	}{
		{"the size of the data", actual, false, model.ErrLimitExceeded},
		{"the size of the data, zip64", actual, true, model.ErrLimitExceeded},
		// A declared size up to the size of the archive is what the ratio
		// is computed from: here 2 MiB over a few KiB.
		{"the size of the archive", size, false, model.ErrLimitExceeded},
		{"one more than the archive", size + 1, false, model.ErrInvalidNSD},
		{"the 32-bit maximum", 0xFFFFFFFE, false, model.ErrInvalidNSD},
		{"zip64, the size of the archive", size + 20, true, model.ErrLimitExceeded},
		{"zip64, one more than the archive", size + 20 + 1, true, model.ErrInvalidNSD},
		{"zip64, above 32 bits", 1 << 32, true, model.ErrInvalidNSD},
		{"zip64, the int64 maximum", 1<<63 - 1, true, model.ErrInvalidNSD},
		{"zip64, above the int64 maximum", 1 << 63, true, model.ErrInvalidNSD},
		{"zip64, the uint64 maximum", 1<<64 - 1, true, model.ErrInvalidNSD},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := nsd.ReadZIP(bytes.NewReader(oneEntry(content, compressed, tc.declared, tc.zip64)))
			if !errors.Is(err, tc.want) || p != nil {
				t.Fatalf("package %v, error = %v, want %v", p != nil, err, tc.want)
			}
			if tc.want == model.ErrInvalidNSD && !strings.Contains(err.Error(), "compressed bytes, more than") ||
				tc.want == model.ErrLimitExceeded && !strings.Contains(err.Error(), "compression ratio") {
				t.Errorf("error %v", err)
			}
		})
	}
	// The same entry with a ratio the limit allows is loaded.
	p, err := nsd.ReadZIP(bytes.NewReader(oneEntry(content, compressed, actual, true)), nsd.WithMaxCompressionRatio(1<<20))
	if err != nil || len(p.Namespaces()) != 1 {
		t.Errorf("with a ratio limit of 2^20: error = %v", err)
	}
}

// Every XML document of an archive is bounded by MaxDocumentSize:
// schemas and manifests as well as namespaces.
func TestDocumentSizeLimitForSchemasAndManifests(t *testing.T) {
	const limit = 4 << 10
	pad := func(open, close string, size int) []byte {
		return []byte(open + strings.Repeat(" ", size-len(open)-len(close)) + close)
	}
	docs := map[string]func(size int) []byte{
		"a.nsd": func(size int) []byte { return pad(nsOpen, "</NS>", size) },
		"NSD.xsd": func(size int) []byte {
			return pad(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`, "</xs:schema>", size)
		},
		"IECManifest.xml": func(size int) []byte { return pad("<IECManifest>", "</IECManifest>", size) },
		"Manifest.xml":    func(size int) []byte { return pad("<IECManifest>", "</IECManifest>", size) },
	}
	dir := t.TempDir()
	for name, build := range docs {
		for _, tc := range []struct {
			size int
			ok   bool
		}{{limit - 1, true}, {limit, true}, {limit + 1, false}, {8 * limit, false}} {
			data := build(tc.size)
			_, err := nsd.ReadZIP(bytes.NewReader(buildZIP(t, entry{name: "pkg/" + name, data: data})), nsd.WithMaxDocumentSize(limit))
			if tc.ok && err != nil || !tc.ok && !errors.Is(err, model.ErrLimitExceeded) {
				t.Errorf("%s of %d bytes in an archive: error = %v", name, tc.size, err)
			}
			file := filepath.Join(dir, name)
			if err := os.WriteFile(file, data, 0o600); err != nil {
				t.Fatal(err)
			}
			_, err = nsd.LoadFile(file, nsd.WithMaxDocumentSize(limit))
			if tc.ok && err != nil || !tc.ok && !errors.Is(err, model.ErrLimitExceeded) {
				t.Errorf("%s of %d bytes as a file: error = %v", name, tc.size, err)
			}
		}
	}
}

// A package carries at most 100 000 findings over all its documents.
func TestPackageFindingLimit(t *testing.T) {
	namespace := func(id string, findings int, body string) string {
		var b strings.Builder
		fmt.Fprintf(&b, `<NS xmlns="http://www.iec.ch/61850/2016/NSD" id="%s" version="2026"`, id)
		for j := range findings {
			fmt.Fprintf(&b, ` a%d="1"`, j)
		}
		return b.String() + `>` + body + `</NS>`
	}
	const (
		nsdoc      = `<NSDoc xmlns="http://www.iec.ch/61850/2016/NSD" id="T" version="2026" lang="en"><Doc id="other">x</Doc></NSDoc>`
		otherNSDoc = `<NSDoc xmlns="http://www.iec.ch/61850/2016/NSD" id="U" version="2026" lang="en"><Doc id="other">x</Doc></NSDoc>`
		documented = `<Abbreviations><Abbreviation name="A" descID="d1"/></Abbreviations>`
	)
	for _, tc := range []struct {
		name    string
		entries int
		docs    []string
		lang    string
		want    int // findings of the package, or -1 when the load is refused
	}{
		{"entries at the limit", 100000, nil, "", 100000},
		{"entries above the limit", 100001, nil, "", -1},
		{"a document at the limit", 99990, []string{namespace("T", 10, "")}, "", 100000},
		{"a document above the limit", 99990, []string{namespace("T", 11, "")}, "", -1},
		// The findings of the package itself, made when it is complete.
		{"an identical copy above the limit", 100000, []string{namespace("T", 0, ""), namespace("T", 0, "")}, "", -1},
		{"no documentation above the limit", 100000, []string{namespace("T", 0, ""), otherNSDoc}, "en", -1},
		{"unmatched documentation above the limit", 100000, []string{namespace("T", 0, documented), nsdoc}, "en", -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := nsd.ReadZIP(bytes.NewReader(handBuilt(tc.entries, tc.docs...)),
				nsd.WithMaxArchiveEntries(200000), nsd.WithLanguage(tc.lang), nsd.WithSource("pkg.zip"))
			if tc.want >= 0 {
				if err != nil || len(p.Diagnostics()) != tc.want {
					t.Fatalf("error = %v", err)
				}
				return
			}
			var se *model.SourceError
			if !errors.Is(err, model.ErrLimitExceeded) || !errors.As(err, &se) || se.Source != "pkg.zip" || !strings.Contains(se.Reason, "more than 100000 findings") {
				t.Fatalf("error = %v, want ErrLimitExceeded for the package", err)
			}
		})
	}
}

// The limits of an archive, each with what the refused load allocated.
func TestArchiveLimitBudgets(t *testing.T) {
	ns := []byte(nsOpen + strings.Repeat(" ", 2<<20) + "</NS>")
	t.Run("size of the archive file", func(t *testing.T) {
		// 4 MiB of input, a limit of 64 KiB: reading stops at the limit.
		data := bytes.Repeat([]byte("PK\x03\x04"), 1<<20)
		var err error
		cost := allocated(func() { _, err = nsd.ReadZIP(bytes.NewReader(data), nsd.WithMaxArchiveSize(64<<10)) })
		if !errors.Is(err, model.ErrLimitExceeded) || cost > 1<<20 {
			t.Errorf("error = %v, %d bytes allocated, budget 1 MiB", err, cost)
		}
	})
	t.Run("uncompressed size", func(t *testing.T) {
		// Two entries of 2 MiB and a budget of 3 MiB: the second entry
		// declares more than what is left and is not decompressed.
		data := buildZIP(t, entry{name: "a.nsd", data: ns}, entry{name: "b.nsd", data: append([]byte("<!-- b -->"), ns...)})
		var err error
		cost := allocated(func() {
			_, err = nsd.ReadZIP(bytes.NewReader(data), nsd.WithMaxArchiveSize(3<<20), nsd.WithMaxCompressionRatio(1<<20))
		})
		// The first document is read (five times its size while the
		// buffer grows) and imported; the second costs nothing. Reading
		// both would cost twice as much.
		if !errors.Is(err, model.ErrLimitExceeded) || !strings.Contains(err.Error(), "b.nsd: entry declares") || cost > 32<<20 {
			t.Errorf("error = %v, %d bytes allocated, budget 32 MiB", err, cost)
		}
	})
	t.Run("compression ratio", func(t *testing.T) {
		data := buildZIP(t, entry{name: "a.nsd", data: ns})
		var err error
		cost := allocated(func() { _, err = nsd.ReadZIP(bytes.NewReader(data)) })
		// The entry is decompressed within the size budget, then the
		// ratio refuses it before it is parsed.
		if !errors.Is(err, model.ErrLimitExceeded) || !strings.Contains(err.Error(), "compression ratio") || cost > 16<<20 {
			t.Errorf("error = %v, %d bytes allocated, budget 16 MiB", err, cost)
		}
	})
	t.Run("nesting", func(t *testing.T) {
		data := stored(t, "a.zip", stored(t, "b.zip", stored(t, "c.nsd", ns)))
		var err error
		cost := allocated(func() { _, err = nsd.ReadZIP(bytes.NewReader(data)) })
		// Two levels are read, each about five times its size while the
		// buffer grows; the third archive is refused unopened.
		if !errors.Is(err, model.ErrLimitExceeded) || !strings.Contains(err.Error(), "nested deeper") || cost > 16*uint64(len(data)) {
			t.Errorf("error = %v, %d bytes allocated, budget %d", err, cost, 16*len(data))
		}
	})
}

// At the boundary of the archive limits: one below, at, one above.
func TestArchiveLimitBoundaries(t *testing.T) {
	ns := []byte(nsOpen + "</NS>")
	three := buildZIP(t, entry{name: "a.nsd", data: ns}, entry{name: "b.txt"}, entry{name: "c.txt"})
	size := int64(len(three))
	for _, tc := range []struct {
		name string
		opt  nsd.Option
		ok   bool
	}{
		{"entries-1", nsd.WithMaxArchiveEntries(2), false},
		{"entries", nsd.WithMaxArchiveEntries(3), true},
		{"entries+1", nsd.WithMaxArchiveEntries(4), true},
		{"archive file-1", nsd.WithMaxArchiveSize(size - 1), false},
		{"archive file", nsd.WithMaxArchiveSize(size), true},
		{"archive file+1", nsd.WithMaxArchiveSize(size + 1), true},
	} {
		_, err := nsd.ReadZIP(bytes.NewReader(three), tc.opt)
		if tc.ok && err != nil || !tc.ok && !errors.Is(err, model.ErrLimitExceeded) {
			t.Errorf("%s: error = %v", tc.name, err)
		}
	}
	// The uncompressed budget, with an entry that is larger than the
	// archive file that holds it.
	doc := []byte(nsOpen + strings.Repeat(" ", 8000) + "</NS>")
	packed := buildZIP(t, entry{name: "a.nsd", data: doc})
	for delta, ok := range map[int64]bool{-1: false, 0: true, 1: true} {
		_, err := nsd.ReadZIP(bytes.NewReader(packed), nsd.WithMaxArchiveSize(int64(len(doc))+delta))
		if ok && err != nil || !ok && !errors.Is(err, model.ErrLimitExceeded) {
			t.Errorf("uncompressed budget %+d: error = %v", delta, err)
		}
	}
}
