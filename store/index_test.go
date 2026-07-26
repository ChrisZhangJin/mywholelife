package store

import (
	"bytes"
	"strings"
	"testing"
)

func TestZstdRoundTrip(t *testing.T) {
	inputs := [][]byte{
		nil,
		[]byte(""),
		[]byte("hello zstd"),
		bytes.Repeat([]byte("the quick brown fox\n"), 4096),
	}
	for _, in := range inputs {
		comp, err := zstdCompress(in)
		if err != nil {
			t.Fatalf("zstdCompress: %v", err)
		}
		back, err := zstdDecompress(comp)
		if err != nil {
			t.Fatalf("zstdDecompress: %v", err)
		}
		if !bytes.Equal(back, in) {
			t.Fatalf("round-trip mismatch: got %d bytes, want %d", len(back), len(in))
		}
	}
}

func TestZstdDecompressCorrupt(t *testing.T) {
	comp, err := zstdCompress([]byte("some payload worth a frame"))
	if err != nil {
		t.Fatalf("zstdCompress: %v", err)
	}
	if _, err := zstdDecompress(comp[:len(comp)-3]); err == nil {
		t.Fatal("zstdDecompress on a truncated frame should error")
	}
	if _, err := zstdDecompress([]byte("not a zstd frame at all")); err == nil {
		t.Fatal("zstdDecompress on garbage should error")
	}
}

func TestZstdDecompressBomb(t *testing.T) {
	// A frame whose decompressed size exceeds the cap must be rejected rather
	// than allocated (T-03-02 defense-in-depth).
	comp, err := zstdCompress(make([]byte, maxDecompressedBytes+1024))
	if err != nil {
		t.Fatalf("zstdCompress: %v", err)
	}
	if _, err := zstdDecompress(comp); err == nil {
		t.Fatal("zstdDecompress should reject a frame larger than the decompression cap")
	}
}

func TestUpsertIndexLineIdempotent(t *testing.T) {
	m := Memory{MemID: "20260726-widgets", Brief: "widget skill"}
	first := upsertIndexLine(nil, m)
	second := upsertIndexLine(first, m)
	if !bytes.Equal(first, second) {
		t.Fatalf("upsert not idempotent:\nfirst=%q\nsecond=%q", first, second)
	}
	if !strings.Contains(string(first), "20260726-widgets") {
		t.Fatalf("upsert did not include memId: %q", first)
	}
	if !strings.HasPrefix(string(first), "# Long-term memory index") {
		t.Fatalf("upsert missing stable header: %q", first)
	}
}

func TestUpsertIndexLineAppendStableOrder(t *testing.T) {
	content := upsertIndexLine(nil, Memory{MemID: "20260726-alpha", Brief: "a"})
	content = upsertIndexLine(content, Memory{MemID: "20260726-beta", Brief: "b"})
	content = upsertIndexLine(content, Memory{MemID: "20260726-gamma", Brief: "c"})

	// Re-upsert an existing entry replaces in place (order preserved).
	content = upsertIndexLine(content, Memory{MemID: "20260726-beta", Brief: "b2"})

	rows, idx := parseIndex(content)
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d: %q", len(rows), content)
	}
	order := []string{rows[0].memID, rows[1].memID, rows[2].memID}
	want := []string{"20260726-alpha", "20260726-beta", "20260726-gamma"}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
	if rows[idx["20260726-beta"]].hook != "b2" {
		t.Fatalf("beta hook not updated: %q", rows[idx["20260726-beta"]].hook)
	}
}

func TestRemoveIndexLine(t *testing.T) {
	content := upsertIndexLine(nil, Memory{MemID: "20260726-alpha", Brief: "a"})
	content = upsertIndexLine(content, Memory{MemID: "20260726-beta", Brief: "b"})

	// Absent memId is a byte-identical no-op.
	same := removeIndexLine(content, "20260726-nope")
	if !bytes.Equal(same, content) {
		t.Fatalf("remove of absent memId changed content:\n%q\n%q", content, same)
	}

	// Present memId is dropped; the other line survives.
	got := removeIndexLine(content, "20260726-alpha")
	if strings.Contains(string(got), "20260726-alpha") {
		t.Fatalf("removed line still present: %q", got)
	}
	if !strings.Contains(string(got), "20260726-beta") {
		t.Fatalf("unrelated line was dropped: %q", got)
	}
}

func TestValidateIndex(t *testing.T) {
	rows := []Memory{
		{MemID: "20260726-alpha", State: StateLongTerm, Brief: "a"},
		{MemID: "20260726-beta", State: StateLongTerm, Brief: "b"},
		{MemID: "20260726-recent", State: StateRecent, Brief: "r"},
	}
	content := upsertIndexLine(nil, rows[0])
	content = upsertIndexLine(content, rows[1])

	if err := validateIndex(rows, content); err != nil {
		t.Fatalf("validateIndex on a consistent set: %v", err)
	}

	// Missing line for a long_term row.
	missing := removeIndexLine(content, "20260726-beta")
	if err := validateIndex(rows, missing); err == nil {
		t.Fatal("validateIndex should fail when a long_term row has no line")
	}

	// Extra line with no matching long_term row.
	extra := upsertIndexLine(content, Memory{MemID: "20260726-ghost", Brief: "g"})
	if err := validateIndex(rows, extra); err == nil {
		t.Fatal("validateIndex should fail on an index line with no long_term row")
	}
}

func TestValidateIndexTombstone(t *testing.T) {
	lt := Memory{MemID: "20260726-lt", State: StateLongTerm, Brief: "lt"}
	tomb := Memory{MemID: "20260726-tomb", State: StateTombstone, Brief: "tomb"}
	rows := []Memory{lt, tomb}

	content := upsertIndexLine(nil, lt)
	content = upsertIndexLine(content, tomb)
	if err := validateIndex(rows, content); err != nil {
		t.Fatalf("a tombstone row WITH a line must pass (D-06): %v", err)
	}

	// A tombstone row WITHOUT a line fails — its remind-able breadcrumb is gone.
	missing := removeIndexLine(content, "20260726-tomb")
	if err := validateIndex(rows, missing); err == nil {
		t.Fatal("tombstone row without an index line must fail")
	}

	// A line WITHOUT any row fails.
	extra := upsertIndexLine(content, Memory{MemID: "20260726-ghost", Brief: "g"})
	if err := validateIndex(rows, extra); err == nil {
		t.Fatal("index line without a long_term/tombstone row must fail")
	}
}

func TestIndexFieldSanitization(t *testing.T) {
	m := Memory{
		MemID: "20260726-widgets",
		Brief: "line one\nline two | piped\r\nmore",
	}
	content := upsertIndexLine(nil, m)
	rows, idx := parseIndex(content)
	l := rows[idx["20260726-widgets"]]
	if strings.ContainsAny(l.hook, "|\r\n") {
		t.Fatalf("hook not sanitized: %q", l.hook)
	}
	if strings.ContainsAny(l.name, "|\r\n") {
		t.Fatalf("name not sanitized: %q", l.name)
	}
	if l.memID != "20260726-widgets" {
		t.Fatalf("memId must be verbatim, got %q", l.memID)
	}

	long := Memory{MemID: "20260726-long", Brief: strings.Repeat("x", 400)}
	rows2, idx2 := parseIndex(upsertIndexLine(nil, long))
	if got := len(rows2[idx2["20260726-long"]].hook); got > 120 {
		t.Fatalf("hook not capped: len=%d", got)
	}
}
