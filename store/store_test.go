package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeBlobStore struct {
	m map[string][]byte
}

func newFakeBlobStore() *fakeBlobStore {
	return &fakeBlobStore{m: map[string][]byte{}}
}

func (f *fakeBlobStore) PutFolder(_ context.Context, relPath string, data []byte) error {
	f.m[relPath] = append([]byte(nil), data...)
	return nil
}

func (f *fakeBlobStore) GetFolder(_ context.Context, relPath string) ([]byte, error) {
	d, ok := f.m[relPath]
	if !ok {
		return nil, ErrNotFound
	}
	return append([]byte(nil), d...), nil
}

func (f *fakeBlobStore) Delete(_ context.Context, relPath string) error {
	delete(f.m, relPath)
	return nil
}

func (f *fakeBlobStore) Exists(_ context.Context, relPath string) (bool, error) {
	_, ok := f.m[relPath]
	return ok, nil
}

func newTestStore(t *testing.T) *sqliteStore {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "idx.db"), newFakeBlobStore())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.db.Close() })
	return s
}

func mustRegister(t *testing.T, s *sqliteStore, name string) Agent {
	t.Helper()
	a, err := s.RegisterAgent(context.Background(), name)
	if err != nil {
		t.Fatalf("RegisterAgent(%q): %v", name, err)
	}
	return a
}

// insertRaw writes a memory row directly so tests can set an arbitrary
// access_time/created_at (the store's write paths always stamp "now").
func insertRaw(t *testing.T, s *sqliteStore, m Memory) {
	t.Helper()
	pinned := int64(0)
	if m.Pinned {
		pinned = 1
	}
	_, err := s.db.Exec(
		`INSERT INTO memories(mem_id,agent_id,scope,state,access_time,pinned,brief,rel_path,created_at)
		 VALUES(?,?,?,?,?,?,?,?,?)`,
		m.MemID, m.AgentID, m.Scope, m.State, m.AccessTime, pinned, m.Brief, m.RelPath, m.CreatedAt)
	if err != nil {
		t.Fatalf("insertRaw(%q): %v", m.MemID, err)
	}
}

func memoryState(t *testing.T, s *sqliteStore, agentID, memID string) (string, int64) {
	t.Helper()
	var state string
	var at int64
	err := s.db.QueryRow(
		`SELECT state, access_time FROM memories WHERE agent_id=? AND mem_id=?`,
		agentID, memID).Scan(&state, &at)
	if err != nil {
		t.Fatalf("memoryState(%q): %v", memID, err)
	}
	return state, at
}

func TestRegisterAgent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	a := mustRegister(t, s, "alice")
	if a.Name != "alice" {
		t.Fatalf("name = %q, want alice", a.Name)
	}
	u, err := uuid.Parse(a.ID)
	if err != nil {
		t.Fatalf("id %q is not a UUID: %v", a.ID, err)
	}
	if u.Version() != 4 {
		t.Fatalf("id %q is UUID v%d, want v4", a.ID, u.Version())
	}
	if a.CreatedAt == 0 {
		t.Fatal("created_at not stamped")
	}

	got, err := s.GetAgent(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if got != a {
		t.Fatalf("GetAgent = %+v, want %+v", got, a)
	}

	if _, err := s.GetAgent(ctx, "no-such-id"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetAgent(unknown) err = %v, want ErrNotFound", err)
	}

	if _, err := s.RegisterAgent(ctx, "alice"); !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("duplicate name err = %v, want ErrDuplicateName", err)
	}
}

func TestPutStampsRecentNow(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	a := mustRegister(t, s, "alice")

	before := time.Now().Unix()
	err := s.Put(ctx, Memory{
		MemID:   "20260726-widgets",
		AgentID: a.ID,
		Scope:   ScopeProject,
		Brief:   "widget skill",
		RelPath: "agents/x/projects/20260726-widgets",
	})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	state, at := memoryState(t, s, a.ID, "20260726-widgets")
	if state != StateRecent {
		t.Fatalf("state = %q, want recent", state)
	}
	if at < before {
		t.Fatalf("access_time %d stamped before Put %d", at, before)
	}
}

func TestTouchUpdatesAccessTime(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	a := mustRegister(t, s, "alice")

	old := time.Now().Unix() - 1_000_000_000
	insertRaw(t, s, Memory{
		MemID: "old-1", AgentID: a.ID, Scope: ScopeProject, State: StateRecent,
		AccessTime: old, CreatedAt: old,
	})

	before := time.Now().Unix()
	if err := s.Touch(ctx, a.ID, "old-1"); err != nil {
		t.Fatalf("Touch: %v", err)
	}
	_, at := memoryState(t, s, a.ID, "old-1")
	if at < before {
		t.Fatalf("access_time %d not bumped to now (%d)", at, before)
	}

	if err := s.Touch(ctx, a.ID, "ghost"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Touch(unknown) err = %v, want ErrNotFound", err)
	}
}

func TestAccessTimeGovernsAging(t *testing.T) {
	// D-07 regression: a memory created long ago but reminded "now" must NOT
	// be selected by an aging query keyed on access_time.
	s := newTestStore(t)
	ctx := context.Background()
	a := mustRegister(t, s, "alice")

	old := time.Now().Unix() - 1_000_000_000
	insertRaw(t, s, Memory{
		MemID: "ancient", AgentID: a.ID, Scope: ScopeProject, State: StateRecent,
		AccessTime: old, CreatedAt: old,
	})

	const threshold = 1000
	aged, err := s.agingCandidates(ctx, a.ID, threshold)
	if err != nil {
		t.Fatalf("agingCandidates: %v", err)
	}
	if !contains(aged, "ancient") {
		t.Fatal("stale memory should be an aging candidate before Touch")
	}

	if err := s.Touch(ctx, a.ID, "ancient"); err != nil {
		t.Fatalf("Touch: %v", err)
	}
	aged, err = s.agingCandidates(ctx, a.ID, threshold)
	if err != nil {
		t.Fatalf("agingCandidates: %v", err)
	}
	if contains(aged, "ancient") {
		t.Fatal("reminded memory must not age out — aging keys on access_time, not created_at")
	}
}

func TestProjectMemIDCollision(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	a := mustRegister(t, s, "alice")

	base := "20260726-widgets"
	for i := 0; i < 3; i++ {
		if err := s.Put(ctx, Memory{MemID: base, AgentID: a.ID, Scope: ScopeProject}); err != nil {
			t.Fatalf("Put #%d: %v", i, err)
		}
	}

	got := listMemIDs(t, s, a.ID)
	want := []string{base, base + "-2", base + "-3"}
	for _, w := range want {
		if !contains(got, w) {
			t.Fatalf("missing collision-suffixed key %q; have %v", w, got)
		}
	}
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func listMemIDs(t *testing.T, s *sqliteStore, agentID string) []string {
	t.Helper()
	rows, err := s.db.Query(`SELECT mem_id FROM memories WHERE agent_id=?`, agentID)
	if err != nil {
		t.Fatalf("listMemIDs: %v", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		ids = append(ids, id)
	}
	return ids
}

func TestOpenIdempotent(t *testing.T) {
	// W1: opening the same file twice re-applies the schema without error.
	p := filepath.Join(t.TempDir(), "idx.db")
	s1, err := Open(p, newFakeBlobStore())
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	s1.db.Close()
	s2, err := Open(p, newFakeBlobStore())
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	s2.db.Close()
}
