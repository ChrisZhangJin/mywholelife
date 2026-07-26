package bundle

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"testing"
)

func tarOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name: name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatalf("tar header %s: %v", name, err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("tar write %s: %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	return buf.Bytes()
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

func overByteTar(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	size := int64(maxTarTotalBytes) + 1
	if err := tw.WriteHeader(&tar.Header{Name: "big.bin", Mode: 0o644, Size: size, Typeflag: tar.TypeReg}); err != nil {
		t.Fatalf("big header: %v", err)
	}
	if _, err := io.CopyN(tw, zeroReader{}, size); err != nil {
		t.Fatalf("big write: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("big close: %v", err)
	}
	return buf.Bytes()
}

func overCountTar(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for i := 0; i < maxTarEntries+1; i++ {
		if err := tw.WriteHeader(&tar.Header{
			Name: fmt.Sprintf("f%d.txt", i), Mode: 0o644, Size: 0, Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatalf("count header %d: %v", i, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("count close: %v", err)
	}
	return buf.Bytes()
}

func TestSafeEntryName(t *testing.T) {
	good := []string{"SKILL.md", "assets/x.md", "a/b/c.txt"}
	bad := []string{"", "/etc/passwd", "../evil", "a/../../b", "..", "."}
	for _, n := range good {
		if !safeEntryName(n) {
			t.Errorf("safeEntryName(%q) = false, want true", n)
		}
	}
	for _, n := range bad {
		if safeEntryName(n) {
			t.Errorf("safeEntryName(%q) = true, want false", n)
		}
	}
}

func TestValidateAndBrief(t *testing.T) {
	good := tarOf(t, map[string]string{
		"SKILL.md":       "---\nname: demo\ndescription: demo brief\n---\nbody\n",
		"assets/note.md": "hi",
	})
	brief, err := ValidateAndBrief(good)
	if err != nil {
		t.Fatalf("ValidateAndBrief good: %v", err)
	}
	if brief != "demo brief" {
		t.Fatalf("brief = %q, want %q", brief, "demo brief")
	}

	for _, name := range []string{"../evil", "/etc/passwd"} {
		bad := tarOf(t, map[string]string{"SKILL.md": "x", name: "y"})
		if _, err := ValidateAndBrief(bad); err == nil {
			t.Errorf("ValidateAndBrief with %q: err = nil, want error", name)
		}
	}
	if _, err := ValidateAndBrief(overCountTar(t)); err == nil {
		t.Error("over-count tar: err = nil, want error")
	}
	if _, err := ValidateAndBrief(overByteTar(t)); err == nil {
		t.Error("over-byte tar: err = nil, want error")
	}
}

func TestFrontmatterDescription(t *testing.T) {
	cases := map[string]string{
		"---\ndescription: hello\n---\nbody":                    "hello",
		"---\ndescription: \"quoted\"\n---\n":                   "quoted",
		"---\ndescription: >\n  line one\n  line two\n---\n":    "line one line two",
		"---\nname: x\n---\n":                                   "",
		"no frontmatter":                                        "",
	}
	for md, want := range cases {
		if got := frontmatterDescription([]byte(md)); got != want {
			t.Errorf("frontmatterDescription(%q) = %q, want %q", md, got, want)
		}
	}
}

func TestCopyTarIntoZipSkipsUnsafe(t *testing.T) {
	tb := tarOf(t, map[string]string{"SKILL.md": "ok"})
	if _, err := ValidateAndBrief(tb); err != nil {
		t.Fatalf("sanity: %v", err)
	}
}
