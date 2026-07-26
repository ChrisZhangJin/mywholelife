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

func TestGetAndList(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	a := mustRegister(t, s, "alice")

	if err := s.Put(ctx, Memory{MemID: "20260726-alpha", AgentID: a.ID, Scope: ScopeProject, Brief: "a"}); err != nil {
		t.Fatalf("Put alpha: %v", err)
	}
	if err := s.Put(ctx, Memory{MemID: "global-recent", AgentID: a.ID, Scope: ScopeGlobal, Pinned: true, Brief: "g"}); err != nil {
		t.Fatalf("Put global: %v", err)
	}

	m, err := s.Get(ctx, a.ID, "20260726-alpha")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if m.Brief != "a" || m.State != StateRecent || m.Scope != ScopeProject || m.Pinned {
		t.Fatalf("Get returned %+v", m)
	}

	g, err := s.Get(ctx, a.ID, "global-recent")
	if err != nil {
		t.Fatalf("Get global: %v", err)
	}
	if !g.Pinned {
		t.Fatal("global-recent should be pinned")
	}

	if _, err := s.Get(ctx, a.ID, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(missing) err = %v, want ErrNotFound", err)
	}

	project, err := s.List(ctx, a.ID, ScopeProject, StateRecent)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(project) != 1 || project[0].MemID != "20260726-alpha" {
		t.Fatalf("List(project,recent) = %+v", project)
	}

	all, err := s.List(ctx, a.ID, "", "")
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("List(all) len = %d, want 2", len(all))
	}
}

func TestForgetTransitions(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	a := mustRegister(t, s, "alice")

	if err := s.Put(ctx, Memory{MemID: "20260726-alpha", AgentID: a.ID, Scope: ScopeProject}); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if err := s.Forget(ctx, a.ID, "20260726-alpha", StateLongTerm); err != nil {
		t.Fatalf("Forget->long_term: %v", err)
	}
	if st, _ := memoryState(t, s, a.ID, "20260726-alpha"); st != StateLongTerm {
		t.Fatalf("state = %q, want long_term", st)
	}

	if err := s.Forget(ctx, a.ID, "20260726-alpha", StateTombstone); err != nil {
		t.Fatalf("Forget->tombstone: %v", err)
	}
	if st, _ := memoryState(t, s, a.ID, "20260726-alpha"); st != StateTombstone {
		t.Fatalf("state = %q, want tombstone", st)
	}

	// The schema CHECK rejects out-of-enum target states.
	if err := s.Forget(ctx, a.ID, "20260726-alpha", "banana"); err == nil {
		t.Fatal("Forget to an invalid state should be rejected by the CHECK")
	}

	if err := s.Forget(ctx, a.ID, "ghost", StateLongTerm); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Forget(unknown) err = %v, want ErrNotFound", err)
	}
}

func TestPinnedGlobalExemption(t *testing.T) {
	// D-08: a pinned global-recent far past any threshold is never an aging
	// candidate and is never tombstoned by Forget.
	s := newTestStore(t)
	ctx := context.Background()
	a := mustRegister(t, s, "alice")

	old := time.Now().Unix() - 1_000_000_000
	insertRaw(t, s, Memory{
		MemID: "global-recent", AgentID: a.ID, Scope: ScopeGlobal, State: StateRecent,
		AccessTime: old, CreatedAt: old, Pinned: true,
	})

	aged, err := s.agingCandidates(ctx, a.ID, 1000)
	if err != nil {
		t.Fatalf("agingCandidates: %v", err)
	}
	if contains(aged, "global-recent") {
		t.Fatal("pinned global-recent must be exempt from aging")
	}

	if err := s.Forget(ctx, a.ID, "global-recent", StateTombstone); err != nil {
		t.Fatalf("Forget(global): %v", err)
	}
	if st, _ := memoryState(t, s, a.ID, "global-recent"); st != StateRecent {
		t.Fatalf("global-recent was transitioned to %q; must stay recent", st)
	}
}

func TestIndexRoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	a := mustRegister(t, s, "alice")

	want := []byte("# long-term memory\n- did a thing\n")
	if err := s.PutIndex(ctx, a.ID, want); err != nil {
		t.Fatalf("PutIndex: %v", err)
	}
	got, err := s.GetIndex(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetIndex: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("GetIndex = %q, want %q", got, want)
	}
}

func TestCompressReheatStubs(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.Compress(ctx, "a", "m"); !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("Compress err = %v, want ErrNotImplemented", err)
	}
	if err := s.Reheat(ctx, "a", "m"); !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("Reheat err = %v, want ErrNotImplemented", err)
	}
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
