package store

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func columnCount(t *testing.T, s *sqliteStore, col string) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow(
		`SELECT count(*) FROM pragma_table_info('memories') WHERE name = ?`, col).Scan(&n); err != nil {
		t.Fatalf("pragma count(%q): %v", col, err)
	}
	return n
}

func deletedAt(t *testing.T, s *sqliteStore, agentID, memID string) sql.NullInt64 {
	t.Helper()
	var d sql.NullInt64
	if err := s.db.QueryRow(
		`SELECT deleted_at FROM memories WHERE agent_id = ? AND mem_id = ?`,
		agentID, memID).Scan(&d); err != nil {
		t.Fatalf("deletedAt(%q): %v", memID, err)
	}
	return d
}

func TestDeletedAtMigration(t *testing.T) {
	p := filepath.Join(t.TempDir(), "legacy.db")

	// Pre-create a legacy schema WITHOUT the deleted_at column.
	raw, err := sql.Open("sqlite", "file:"+p)
	if err != nil {
		t.Fatalf("open raw: %v", err)
	}
	if _, err := raw.Exec(`CREATE TABLE agents (
	  id TEXT PRIMARY KEY, name TEXT UNIQUE NOT NULL, created_at INTEGER NOT NULL)`); err != nil {
		t.Fatalf("legacy agents: %v", err)
	}
	if _, err := raw.Exec(`CREATE TABLE memories (
	  mem_id TEXT PRIMARY KEY,
	  agent_id TEXT NOT NULL REFERENCES agents(id),
	  scope TEXT NOT NULL CHECK(scope IN ('global','project')),
	  state TEXT NOT NULL CHECK(state IN ('recent','long_term','tombstone')),
	  access_time INTEGER NOT NULL,
	  pinned INTEGER NOT NULL DEFAULT 0,
	  brief TEXT, rel_path TEXT, created_at INTEGER NOT NULL)`); err != nil {
		t.Fatalf("legacy memories: %v", err)
	}
	raw.Close()

	// Open runs the guarded ALTER, adding deleted_at to the pre-existing table.
	s, err := Open(p, newFakeBlobStore())
	if err != nil {
		t.Fatalf("Open (migrate): %v", err)
	}
	if n := columnCount(t, s, "deleted_at"); n != 1 {
		t.Fatalf("migration did not add deleted_at (count=%d)", n)
	}
	if _, err := s.db.Exec(`SELECT deleted_at FROM memories LIMIT 0`); err != nil {
		t.Fatalf("deleted_at not queryable after migration: %v", err)
	}
	s.db.Close()

	// Idempotent: reopening finds the column present and skips the ALTER.
	s2, err := Open(p, newFakeBlobStore())
	if err != nil {
		t.Fatalf("Open (reopen): %v", err)
	}
	if n := columnCount(t, s2, "deleted_at"); n != 1 {
		t.Fatalf("reopen changed column count to %d", n)
	}
	s2.db.Close()
}

func TestCompressHookSeam(t *testing.T) {
	ctx := context.Background()
	blobs := newFakeBlobStore()
	s := newTestStoreWithBlobs(t, blobs)
	a := mustRegister(t, s, "alice")

	tar := bytes.Repeat([]byte("skill body\n"), 50)
	seedRecent(t, s, blobs, a.ID, "20260726-hooked", tar)
	seedRecent(t, s, blobs, a.ID, "20260726-plain", tar)

	// A non-empty hook carries untrusted LLM text — it must land sanitized and
	// capped, never injecting the index's '|'/newline structure.
	if err := s.Compress(ctx, a.ID, "20260726-hooked", "custom | hook\ntext"); err != nil {
		t.Fatalf("Compress(hook): %v", err)
	}
	// An empty hook preserves the Phase-3 brief-derived line.
	if err := s.Compress(ctx, a.ID, "20260726-plain", ""); err != nil {
		t.Fatalf("Compress(\"\"): %v", err)
	}

	content, err := s.GetIndex(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetIndex: %v", err)
	}
	rows, idx := parseIndex(content)
	if len(rows) != 2 {
		t.Fatalf("index should hold 2 lines, got %d: %q", len(rows), content)
	}

	hooked := rows[idx["20260726-hooked"]]
	if strings.ContainsAny(hooked.hook, "|\r\n") {
		t.Fatalf("untrusted hook injected index structure: %q", hooked.hook)
	}
	if len(hooked.hook) > hookMaxLen {
		t.Fatalf("hook not capped: len=%d", len(hooked.hook))
	}
	if !strings.Contains(hooked.hook, "custom") || !strings.Contains(hooked.hook, "text") {
		t.Fatalf("hook text not carried into the index line: %q", hooked.hook)
	}

	plain := rows[idx["20260726-plain"]]
	if plain.hook != "widget skill" {
		t.Fatalf("empty hook must derive from brief, got %q", plain.hook)
	}
}

func TestReheatClearsDeletedAt(t *testing.T) {
	ctx := context.Background()
	blobs := newFakeBlobStore()
	s := newTestStoreWithBlobs(t, blobs)
	a := mustRegister(t, s, "alice")

	tar := bytes.Repeat([]byte("archived body\n"), 40)
	comp, err := zstdCompress(tar)
	if err != nil {
		t.Fatalf("zstdCompress: %v", err)
	}
	rel := "agents/" + a.ID + "/projects/20260726-tomb.tar.zst"
	if err := blobs.PutFolder(ctx, rel, comp); err != nil {
		t.Fatalf("seed blob: %v", err)
	}
	insertRaw(t, s, Memory{
		MemID: "20260726-tomb", AgentID: a.ID, Scope: ScopeProject, State: StateTombstone,
		AccessTime: 1000, CreatedAt: 1000, Brief: "archived", RelPath: rel,
	})

	if err := s.SoftDelete(ctx, a.ID, "20260726-tomb", 12345); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}
	if got := deletedAt(t, s, a.ID, "20260726-tomb"); !got.Valid || got.Int64 != 12345 {
		t.Fatalf("SoftDelete did not stamp deleted_at: %+v", got)
	}

	if err := s.Reheat(ctx, a.ID, "20260726-tomb"); err != nil {
		t.Fatalf("Reheat: %v", err)
	}
	m, err := s.Get(ctx, a.ID, "20260726-tomb")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if m.State != StateRecent {
		t.Fatalf("state = %q, want recent", m.State)
	}
	if m.DeletedAt.Valid {
		t.Fatalf("Reheat must clear deleted_at, got %d", m.DeletedAt.Int64)
	}
}

func TestSoftDelete(t *testing.T) {
	ctx := context.Background()
	s := newTestStoreWithBlobs(t, newFakeBlobStore())
	a := mustRegister(t, s, "alice")

	insertRaw(t, s, Memory{MemID: "t1", AgentID: a.ID, Scope: ScopeProject, State: StateTombstone, AccessTime: 1, CreatedAt: 1})
	insertRaw(t, s, Memory{MemID: "pinned", AgentID: a.ID, Scope: ScopeProject, State: StateTombstone, AccessTime: 1, CreatedAt: 1, Pinned: true})
	insertRaw(t, s, Memory{MemID: "glob", AgentID: a.ID, Scope: ScopeGlobal, State: StateTombstone, AccessTime: 1, CreatedAt: 1})

	if err := s.SoftDelete(ctx, a.ID, "t1", 500); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}
	if got := deletedAt(t, s, a.ID, "t1"); !got.Valid || got.Int64 != 500 {
		t.Fatalf("deleted_at = %+v, want 500", got)
	}

	// A re-run must not reset the grace clock (deleted_at IS NULL guard).
	if err := s.SoftDelete(ctx, a.ID, "t1", 900); err != nil {
		t.Fatalf("SoftDelete re-run: %v", err)
	}
	if got := deletedAt(t, s, a.ID, "t1"); got.Int64 != 500 {
		t.Fatalf("second SoftDelete reset grace clock to %d", got.Int64)
	}

	// Pinned and global tombstones are exempt.
	if err := s.SoftDelete(ctx, a.ID, "pinned", 500); err != nil {
		t.Fatalf("SoftDelete pinned: %v", err)
	}
	if deletedAt(t, s, a.ID, "pinned").Valid {
		t.Fatal("pinned tombstone must not be soft-deleted")
	}
	if err := s.SoftDelete(ctx, a.ID, "glob", 500); err != nil {
		t.Fatalf("SoftDelete global: %v", err)
	}
	if deletedAt(t, s, a.ID, "glob").Valid {
		t.Fatal("global tombstone must not be soft-deleted")
	}
}

func TestHardDelete(t *testing.T) {
	ctx := context.Background()
	blobs := newFakeBlobStore()
	s := newTestStoreWithBlobs(t, blobs)
	a := mustRegister(t, s, "alice")

	rel := "agents/" + a.ID + "/projects/20260726-doomed.tar.zst"
	if err := blobs.PutFolder(ctx, rel, []byte("archive")); err != nil {
		t.Fatalf("seed blob: %v", err)
	}
	m := Memory{MemID: "20260726-doomed", AgentID: a.ID, Scope: ScopeProject, State: StateTombstone, AccessTime: 1, CreatedAt: 1, Brief: "doomed", RelPath: rel}
	insertRaw(t, s, m)
	if err := s.PutIndex(ctx, a.ID, upsertIndexLine(nil, m)); err != nil {
		t.Fatalf("seed index: %v", err)
	}
	if err := s.SoftDelete(ctx, a.ID, "20260726-doomed", 100); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}

	if err := s.HardDelete(ctx, a.ID, "20260726-doomed"); err != nil {
		t.Fatalf("HardDelete: %v", err)
	}
	if _, err := s.Get(ctx, a.ID, "20260726-doomed"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("row must be gone, got %v", err)
	}
	if ok, _ := blobs.Exists(ctx, rel); ok {
		t.Fatal("blob must be deleted")
	}
	content, _ := s.GetIndex(ctx, a.ID)
	if _, idx := parseIndex(content); len(idx) != 0 {
		t.Fatalf("index line must be removed: %q", content)
	}

	// Re-run is an idempotent no-op.
	if err := s.HardDelete(ctx, a.ID, "20260726-doomed"); err != nil {
		t.Fatalf("HardDelete re-run: %v", err)
	}

	// A tombstone that is NOT soft-deleted must not be destroyed.
	rel2 := "agents/" + a.ID + "/projects/20260726-safe.tar.zst"
	if err := blobs.PutFolder(ctx, rel2, []byte("keep")); err != nil {
		t.Fatalf("seed safe blob: %v", err)
	}
	insertRaw(t, s, Memory{MemID: "20260726-safe", AgentID: a.ID, Scope: ScopeProject, State: StateTombstone, AccessTime: 1, CreatedAt: 1, RelPath: rel2})
	if err := s.HardDelete(ctx, a.ID, "20260726-safe"); err != nil {
		t.Fatalf("HardDelete safe: %v", err)
	}
	if _, err := s.Get(ctx, a.ID, "20260726-safe"); err != nil {
		t.Fatalf("non-soft-deleted tombstone must survive: %v", err)
	}
	if ok, _ := blobs.Exists(ctx, rel2); !ok {
		t.Fatal("non-soft-deleted blob must survive")
	}
}

func TestCommitIndex(t *testing.T) {
	ctx := context.Background()
	blobs := newFakeBlobStore()
	s := newTestStoreWithBlobs(t, blobs)
	a := mustRegister(t, s, "alice")

	m1 := Memory{MemID: "20260726-a", AgentID: a.ID, Scope: ScopeProject, State: StateLongTerm, AccessTime: 1, CreatedAt: 1, Brief: "a", RelPath: "agents/x/a.tar.zst"}
	insertRaw(t, s, m1)
	good := upsertIndexLine(nil, m1)

	if err := s.CommitIndex(ctx, a.ID, good); err != nil {
		t.Fatalf("CommitIndex(valid): %v", err)
	}
	if got, _ := s.GetIndex(ctx, a.ID); !bytes.Equal(got, good) {
		t.Fatalf("valid index not written: %q", got)
	}

	// A second valid commit keeps a .bak of the prior version.
	m2 := Memory{MemID: "20260726-b", AgentID: a.ID, Scope: ScopeProject, State: StateLongTerm, AccessTime: 1, CreatedAt: 1, Brief: "b", RelPath: "agents/x/b.tar.zst"}
	insertRaw(t, s, m2)
	good2 := upsertIndexLine(good, m2)
	if err := s.CommitIndex(ctx, a.ID, good2); err != nil {
		t.Fatalf("CommitIndex(valid 2): %v", err)
	}
	bak, err := blobs.GetFolder(ctx, indexPath(a.ID)+".bak")
	if err != nil {
		t.Fatalf("expected a .bak of the prior index: %v", err)
	}
	if !bytes.Equal(bak, good) {
		t.Fatalf(".bak is not the prior version: %q", bak)
	}

	// Invalid content (a dropped long_term line) is rejected; disk is untouched.
	before, _ := s.GetIndex(ctx, a.ID)
	bad := removeIndexLine(good2, "20260726-b")
	if err := s.CommitIndex(ctx, a.ID, bad); err == nil {
		t.Fatal("CommitIndex must reject content missing a long_term line")
	}
	after, _ := s.GetIndex(ctx, a.ID)
	if !bytes.Equal(before, after) {
		t.Fatal("a failed CommitIndex must leave the on-disk index byte-unchanged")
	}
	if ok, _ := blobs.Exists(ctx, indexPath(a.ID)+".tmp"); ok {
		t.Fatal("a failed CommitIndex must not leave a .tmp file")
	}
}

func TestScanConsistency(t *testing.T) {
	ctx := context.Background()
	blobs := newFakeBlobStore()
	s := newTestStoreWithBlobs(t, blobs)
	a := mustRegister(t, s, "alice")

	// Clean baseline: one long_term memory, its blob, and a valid index.
	rel := "agents/" + a.ID + "/projects/20260726-ok.tar.zst"
	if err := blobs.PutFolder(ctx, rel, []byte("archive")); err != nil {
		t.Fatalf("seed blob: %v", err)
	}
	m := Memory{MemID: "20260726-ok", AgentID: a.ID, Scope: ScopeProject, State: StateLongTerm, AccessTime: 1, CreatedAt: 1, Brief: "ok", RelPath: rel}
	insertRaw(t, s, m)
	if err := s.PutIndex(ctx, a.ID, upsertIndexLine(nil, m)); err != nil {
		t.Fatalf("seed index: %v", err)
	}

	rep, err := s.ScanConsistency(ctx, a.ID)
	if err != nil {
		t.Fatalf("ScanConsistency (clean): %v", err)
	}
	if len(rep.Findings) != 0 {
		t.Fatalf("a clean store must report zero, got %+v", rep.Findings)
	}

	// Inject an orphan blob (no row) and a stale index line (no row).
	orphan := "agents/" + a.ID + "/projects/20260726-orphan.tar.zst"
	if err := blobs.PutFolder(ctx, orphan, []byte("norow")); err != nil {
		t.Fatalf("seed orphan: %v", err)
	}
	stale := upsertIndexLine(upsertIndexLine(nil, m), Memory{MemID: "20260726-ghost", Brief: "ghost"})
	if err := blobs.PutFolder(ctx, indexPath(a.ID), stale); err != nil {
		t.Fatalf("inject stale index: %v", err)
	}

	rep, err = s.ScanConsistency(ctx, a.ID)
	if err != nil {
		t.Fatalf("ScanConsistency (dirty): %v", err)
	}
	kinds := map[string]bool{}
	for _, f := range rep.Findings {
		kinds[f.Kind] = true
	}
	if !kinds["orphan_blob"] {
		t.Fatalf("orphan blob not reported: %+v", rep.Findings)
	}
	if !kinds["index_mismatch"] {
		t.Fatalf("stale index line not reported: %+v", rep.Findings)
	}
}
