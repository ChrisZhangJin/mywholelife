package store

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestBlobRoundTrip(t *testing.T) {
	root := t.TempDir()
	b := NewBlobStore(root)
	ctx := context.Background()

	cases := map[string][]byte{
		"agents/a1/global/index.md":                 []byte("global recent memory"),
		"agents/a1/projects/20260726-demo/skill.md": []byte("project skill folder"),
	}
	for rel, data := range cases {
		if err := b.PutFolder(ctx, rel, data); err != nil {
			t.Fatalf("PutFolder(%q): %v", rel, err)
		}
		got, err := b.GetFolder(ctx, rel)
		if err != nil {
			t.Fatalf("GetFolder(%q): %v", rel, err)
		}
		if !bytes.Equal(got, data) {
			t.Errorf("GetFolder(%q) = %q, want %q", rel, got, data)
		}
		ok, err := b.Exists(ctx, rel)
		if err != nil || !ok {
			t.Errorf("Exists(%q) = %v, %v; want true, nil", rel, ok, err)
		}
		// Lands at the D-09 path under the data root.
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Errorf("expected file at data-root path for %q: %v", rel, err)
		}
	}
}

func TestBlobExistsAndDelete(t *testing.T) {
	root := t.TempDir()
	b := NewBlobStore(root)
	ctx := context.Background()
	rel := "agents/a1/global/index.md"

	if ok, err := b.Exists(ctx, rel); err != nil || ok {
		t.Fatalf("Exists before put = %v, %v; want false, nil", ok, err)
	}
	if err := b.PutFolder(ctx, rel, []byte("data")); err != nil {
		t.Fatalf("PutFolder: %v", err)
	}
	if ok, err := b.Exists(ctx, rel); err != nil || !ok {
		t.Fatalf("Exists after put = %v, %v; want true, nil", ok, err)
	}
	if err := b.Delete(ctx, rel); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if ok, err := b.Exists(ctx, rel); err != nil || ok {
		t.Fatalf("Exists after delete = %v, %v; want false, nil", ok, err)
	}
	if _, err := b.GetFolder(ctx, rel); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetFolder after delete = %v; want ErrNotFound", err)
	}
	// Delete is idempotent: removing a missing path is not an error.
	if err := b.Delete(ctx, rel); err != nil {
		t.Errorf("Delete missing = %v; want nil", err)
	}
}

func TestBlobTraversalRejected(t *testing.T) {
	root := t.TempDir()
	b := NewBlobStore(root)
	ctx := context.Background()

	bad := []string{
		"../escape.txt",
		"a/../../b",
		filepath.Join(root, "..", "escape.txt"), // absolute
		"/etc/passwd",
	}
	for _, rel := range bad {
		if err := b.PutFolder(ctx, rel, []byte("x")); !errors.Is(err, ErrUnsafePath) {
			t.Errorf("PutFolder(%q) = %v; want ErrUnsafePath", rel, err)
		}
		if _, err := b.GetFolder(ctx, rel); !errors.Is(err, ErrUnsafePath) {
			t.Errorf("GetFolder(%q) = %v; want ErrUnsafePath", rel, err)
		}
		if _, err := b.Exists(ctx, rel); !errors.Is(err, ErrUnsafePath) {
			t.Errorf("Exists(%q) = %v; want ErrUnsafePath", rel, err)
		}
		if err := b.Delete(ctx, rel); !errors.Is(err, ErrUnsafePath) {
			t.Errorf("Delete(%q) = %v; want ErrUnsafePath", rel, err)
		}
	}
	// No write escaped the data root.
	if _, err := os.Stat(filepath.Join(filepath.Dir(root), "escape.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("traversal write escaped the data root: %v", err)
	}
}
