package store

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// faultBlobStore corrupts writes to any path ending in corruptSuffix, so a
// Compress/Reheat round-trip read-back verify sees garbage — exercising the
// verify-before-delete negative path (T-03-01).
type faultBlobStore struct {
	BlobStore
	corruptSuffix string
}

func (f *faultBlobStore) PutFolder(ctx context.Context, relPath string, data []byte) error {
	if f.corruptSuffix != "" && strings.HasSuffix(relPath, f.corruptSuffix) {
		data = []byte("corrupted-not-a-zstd-frame")
	}
	return f.BlobStore.PutFolder(ctx, relPath, data)
}

func newTestStoreWithBlobs(t *testing.T, blobs BlobStore) *sqliteStore {
	t.Helper()
	s, err := Open(t.TempDir()+"/idx.db", blobs)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.db.Close() })
	return s
}

func seedRecent(t *testing.T, s *sqliteStore, blobs BlobStore, agentID, memID string, tar []byte) int64 {
	t.Helper()
	rel := "agents/" + agentID + "/projects/" + memID + ".tar"
	if err := blobs.PutFolder(context.Background(), rel, tar); err != nil {
		t.Fatalf("seed PutFolder: %v", err)
	}
	old := time.Now().Unix() - 100_000
	insertRaw(t, s, Memory{
		MemID: memID, AgentID: agentID, Scope: ScopeProject, State: StateRecent,
		AccessTime: old, CreatedAt: old, Brief: "widget skill", RelPath: rel,
	})
	return old
}

func TestCompressReheatRoundTrip(t *testing.T) {
	ctx := context.Background()
	blobs := newFakeBlobStore()
	s := newTestStoreWithBlobs(t, blobs)
	a := mustRegister(t, s, "alice")

	tar := bytes.Repeat([]byte("SKILL.md contents and more\n"), 500)
	oldAT := seedRecent(t, s, blobs, a.ID, "20260726-widgets", tar)
	tarPath := "agents/" + a.ID + "/projects/20260726-widgets.tar"
	zstPath := "agents/" + a.ID + "/projects/20260726-widgets.tar.zst"

	if err := s.Compress(ctx, a.ID, "20260726-widgets"); err != nil {
		t.Fatalf("Compress: %v", err)
	}

	if ok, _ := blobs.Exists(ctx, zstPath); !ok {
		t.Fatal("Compress did not write .tar.zst")
	}
	if ok, _ := blobs.Exists(ctx, tarPath); ok {
		t.Fatal("Compress must delete the source .tar")
	}
	m, _ := s.Get(ctx, a.ID, "20260726-widgets")
	if m.State != StateLongTerm {
		t.Fatalf("state = %q, want long_term", m.State)
	}
	if !strings.HasSuffix(m.RelPath, ".tar.zst") {
		t.Fatalf("rel_path = %q, want .tar.zst", m.RelPath)
	}
	if m.AccessTime != oldAT {
		t.Fatalf("Compress must NOT bump access_time: got %d, want %d", m.AccessTime, oldAT)
	}
	idxContent, err := s.GetIndex(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetIndex: %v", err)
	}
	if _, idx := parseIndex(idxContent); len(idx) != 1 || idx["20260726-widgets"] != 0 {
		t.Fatalf("index should hold exactly the compressed line: %q", idxContent)
	}

	// Reheat restores the original bytes.
	beforeReheat := time.Now().Unix()
	if err := s.Reheat(ctx, a.ID, "20260726-widgets"); err != nil {
		t.Fatalf("Reheat: %v", err)
	}
	restored, err := blobs.GetFolder(ctx, tarPath)
	if err != nil {
		t.Fatalf("GetFolder(restored .tar): %v", err)
	}
	if !bytes.Equal(restored, tar) {
		t.Fatal("Reheat did not restore byte-identical .tar")
	}
	if ok, _ := blobs.Exists(ctx, zstPath); ok {
		t.Fatal("Reheat must delete the .tar.zst")
	}
	m, _ = s.Get(ctx, a.ID, "20260726-widgets")
	if m.State != StateRecent {
		t.Fatalf("state = %q, want recent", m.State)
	}
	if !strings.HasSuffix(m.RelPath, ".tar") || strings.HasSuffix(m.RelPath, ".tar.zst") {
		t.Fatalf("rel_path = %q, want .tar", m.RelPath)
	}
	if m.AccessTime < beforeReheat {
		t.Fatalf("Reheat must bump access_time: got %d, want >= %d", m.AccessTime, beforeReheat)
	}
	idxContent, _ = s.GetIndex(ctx, a.ID)
	if _, idx := parseIndex(idxContent); len(idx) != 0 {
		t.Fatalf("Reheat must remove the index line: %q", idxContent)
	}
}

func TestCompressVerifyFailureKeepsSource(t *testing.T) {
	ctx := context.Background()
	blobs := &faultBlobStore{BlobStore: newFakeBlobStore(), corruptSuffix: ".tar.zst"}
	s := newTestStoreWithBlobs(t, blobs)
	a := mustRegister(t, s, "alice")

	tar := bytes.Repeat([]byte("payload\n"), 200)
	seedRecent(t, s, blobs, a.ID, "20260726-widgets", tar)
	tarPath := "agents/" + a.ID + "/projects/20260726-widgets.tar"
	zstPath := "agents/" + a.ID + "/projects/20260726-widgets.tar.zst"

	if err := s.Compress(ctx, a.ID, "20260726-widgets"); err == nil {
		t.Fatal("Compress must error when round-trip verify fails")
	}
	if ok, _ := blobs.Exists(ctx, tarPath); !ok {
		t.Fatal("source .tar must survive a failed verify")
	}
	if ok, _ := blobs.Exists(ctx, zstPath); ok {
		t.Fatal("the bad .tar.zst must be removed on verify failure")
	}
	if m, _ := s.Get(ctx, a.ID, "20260726-widgets"); m.State != StateRecent {
		t.Fatalf("state must stay recent on verify failure, got %q", m.State)
	}
}

func TestReheatIdempotentOnRecent(t *testing.T) {
	ctx := context.Background()
	blobs := newFakeBlobStore()
	s := newTestStoreWithBlobs(t, blobs)
	a := mustRegister(t, s, "alice")

	oldAT := seedRecent(t, s, blobs, a.ID, "20260726-widgets", []byte("tar"))
	tarPath := "agents/" + a.ID + "/projects/20260726-widgets.tar"

	before := time.Now().Unix()
	if err := s.Reheat(ctx, a.ID, "20260726-widgets"); err != nil {
		t.Fatalf("Reheat on already-recent must be a no-op Touch, got: %v", err)
	}
	m, _ := s.Get(ctx, a.ID, "20260726-widgets")
	if m.State != StateRecent {
		t.Fatalf("state = %q, want recent", m.State)
	}
	if m.RelPath != tarPath {
		t.Fatalf("rel_path changed on idempotent reheat: %q", m.RelPath)
	}
	if m.AccessTime < before && m.AccessTime == oldAT {
		t.Fatal("Reheat on recent must still bump access_time (Touch)")
	}
}

func TestCompressReheatNotFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStoreWithBlobs(t, newFakeBlobStore())
	a := mustRegister(t, s, "alice")

	if err := s.Compress(ctx, a.ID, "ghost"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Compress(missing) err = %v, want ErrNotFound", err)
	}
	if err := s.Reheat(ctx, a.ID, "ghost"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Reheat(missing) err = %v, want ErrNotFound", err)
	}
}
