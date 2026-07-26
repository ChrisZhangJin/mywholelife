package dream

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mywholelife/store"
)

func newJobStore(t *testing.T) (store.MemoryStore, store.BlobStore, string) {
	t.Helper()
	root := t.TempDir()
	blobs := store.NewBlobStore(root)
	st, err := store.Open(filepath.Join(root, "idx.db"), blobs)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	a, err := st.RegisterAgent(context.Background(), "alice")
	if err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	return st, blobs, a.ID
}

// seedRecent writes a .tar body and a recent project memory; Put stamps
// access_time≈now, so a Job.Now set N days ahead crosses the thresholds.
func seedRecent(t *testing.T, st store.MemoryStore, blobs store.BlobStore, agentID, memID string) {
	t.Helper()
	ctx := context.Background()
	rel := "agents/" + agentID + "/projects/" + memID + ".tar"
	if err := blobs.PutFolder(ctx, rel, []byte("skill body for "+memID)); err != nil {
		t.Fatalf("seed blob: %v", err)
	}
	if err := st.Put(ctx, store.Memory{
		MemID: memID, AgentID: agentID, Scope: store.ScopeProject,
		Brief: "widget skill", RelPath: rel,
	}); err != nil {
		t.Fatalf("seed Put: %v", err)
	}
}

func stateOf(t *testing.T, st store.MemoryStore, agentID, memID string) string {
	t.Helper()
	m, err := st.Get(context.Background(), agentID, memID)
	if err != nil {
		t.Fatalf("Get(%q): %v", memID, err)
	}
	return m.State
}

func TestT1CompressWithHook(t *testing.T) {
	ctx := context.Background()
	st, blobs, ag := newJobStore(t)
	seedRecent(t, st, blobs, ag, "20260101-hooked")
	base := time.Now().Unix()

	j := &Job{Store: st, Blobs: blobs, Gen: fakeHookGen{hook: "consolidated widget notes"},
		Cfg: FromEnv(), Now: base + 20*day}
	if err := j.Run(ctx, ag); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if s := stateOf(t, st, ag, "20260101-hooked"); s != store.StateLongTerm {
		t.Fatalf("state = %q, want long_term", s)
	}
	if ok, _ := blobs.Exists(ctx, "agents/"+ag+"/projects/20260101-hooked.tar.zst"); !ok {
		t.Fatal(".tar.zst not written")
	}
	if ok, _ := blobs.Exists(ctx, "agents/"+ag+"/projects/20260101-hooked.tar"); ok {
		t.Fatal("source .tar must be deleted")
	}
	idx, _ := st.GetIndex(ctx, ag)
	if !strings.Contains(string(idx), "consolidated widget notes") {
		t.Fatalf("LLM hook did not reach the index via Compress(hook): %q", idx)
	}
}

func TestT1FallbackOnHookError(t *testing.T) {
	ctx := context.Background()
	st, blobs, ag := newJobStore(t)
	seedRecent(t, st, blobs, ag, "20260101-fb")
	base := time.Now().Unix()

	j := &Job{Store: st, Blobs: blobs, Gen: fakeHookGen{err: errors.New("llm down")},
		Cfg: FromEnv(), Now: base + 20*day}
	if err := j.Run(ctx, ag); err != nil {
		t.Fatalf("Run must succeed on LLM failure (fallback): %v", err)
	}
	if s := stateOf(t, st, ag, "20260101-fb"); s != store.StateLongTerm {
		t.Fatalf("state = %q, want long_term", s)
	}
	idx, _ := st.GetIndex(ctx, ag)
	if !strings.Contains(string(idx), "widget skill") {
		t.Fatalf("fallback did not derive hook from brief: %q", idx)
	}
}

func TestT2Tombstone(t *testing.T) {
	ctx := context.Background()
	st, blobs, ag := newJobStore(t)
	seedRecent(t, st, blobs, ag, "20260101-t2")
	base := time.Now().Unix()
	cfg := FromEnv()

	j := &Job{Store: st, Blobs: blobs, Gen: fakeHookGen{hook: "h"}, Cfg: cfg, Now: base + 20*day}
	if err := j.Run(ctx, ag); err != nil {
		t.Fatalf("Run 1: %v", err)
	}
	if s := stateOf(t, st, ag, "20260101-t2"); s != store.StateLongTerm {
		t.Fatalf("after T1 state = %q, want long_term", s)
	}

	j.Now = base + 100*day
	if err := j.Run(ctx, ag); err != nil {
		t.Fatalf("Run 2: %v", err)
	}
	if s := stateOf(t, st, ag, "20260101-t2"); s != store.StateTombstone {
		t.Fatalf("after T2 state = %q, want tombstone", s)
	}
	if ok, _ := blobs.Exists(ctx, "agents/"+ag+"/projects/20260101-t2.tar.zst"); !ok {
		t.Fatal("tombstone must KEEP its .tar.zst")
	}
	idx, _ := st.GetIndex(ctx, ag)
	if !strings.Contains(string(idx), "20260101-t2") {
		t.Fatalf("tombstone must KEEP its index line: %q", idx)
	}
}

func TestT3SoftDeleteThenHardDeleteWithCap(t *testing.T) {
	ctx := context.Background()
	st, blobs, ag := newJobStore(t)
	base := time.Now().Unix()
	ids := []string{"20260101-a", "20260101-b", "20260101-c", "20260101-d", "20260101-e"}
	for _, id := range ids {
		seedRecent(t, st, blobs, ag, id)
	}
	cfg := FromEnv()
	cfg.MaxDeletions = 3

	// One pass 200d out: compress -> tombstone -> soft-delete (T3 mark).
	j := &Job{Store: st, Blobs: blobs, Gen: fakeHookGen{hook: "h"}, Cfg: cfg, Now: base + 200*day}
	if err := j.Run(ctx, ag); err != nil {
		t.Fatalf("Run (mark): %v", err)
	}
	for _, id := range ids {
		m, _ := st.Get(ctx, ag, id)
		if m.State != store.StateTombstone || !m.DeletedAt.Valid {
			t.Fatalf("%s not soft-deleted tombstone: %+v", id, m)
		}
	}
	remaining, _ := st.List(ctx, ag, "", store.StateTombstone)
	if len(remaining) != 5 {
		t.Fatalf("nothing should hard-delete before grace, have %d", len(remaining))
	}

	// Next pass past grace: 5 candidates, cap 3 -> 3 deleted, 2 survive.
	j.Now = base + 200*day + 40*day
	if err := j.Run(ctx, ag); err != nil {
		t.Fatalf("Run (hard-delete): %v", err)
	}
	remaining, _ = st.List(ctx, ag, "", store.StateTombstone)
	if len(remaining) != 2 {
		t.Fatalf("rate cap: want 2 survivors, have %d", len(remaining))
	}
	// Sorted by memId, a & b & c are destroyed; d & e survive.
	for _, gone := range []string{"20260101-a", "20260101-b", "20260101-c"} {
		if _, err := st.Get(ctx, ag, gone); !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("%s should be hard-deleted, err=%v", gone, err)
		}
		if ok, _ := blobs.Exists(ctx, "agents/"+ag+"/projects/"+gone+".tar.zst"); ok {
			t.Fatalf("%s blob must be gone", gone)
		}
	}
	for _, kept := range []string{"20260101-d", "20260101-e"} {
		if _, err := st.Get(ctx, ag, kept); err != nil {
			t.Fatalf("%s must survive the cap: %v", kept, err)
		}
	}
}

func TestRerunNoop(t *testing.T) {
	ctx := context.Background()
	st, blobs, ag := newJobStore(t)
	seedRecent(t, st, blobs, ag, "20260101-p")
	seedRecent(t, st, blobs, ag, "20260101-q")
	base := time.Now().Unix()

	j := &Job{Store: st, Blobs: blobs, Gen: fakeHookGen{hook: "stable hook"}, Cfg: FromEnv(), Now: base + 20*day}
	if err := j.Run(ctx, ag); err != nil {
		t.Fatalf("Run 1: %v", err)
	}
	idx1, _ := st.GetIndex(ctx, ag)

	if err := j.Run(ctx, ag); err != nil {
		t.Fatalf("Run 2: %v", err)
	}
	idx2, _ := st.GetIndex(ctx, ag)
	if string(idx1) != string(idx2) {
		t.Fatalf("re-run mutated the index:\n%q\n%q", idx1, idx2)
	}
	for _, id := range []string{"20260101-p", "20260101-q"} {
		if s := stateOf(t, st, ag, id); s != store.StateLongTerm {
			t.Fatalf("%s state drifted to %q", id, s)
		}
	}
}

func TestFinalGateBlocksCorruption(t *testing.T) {
	ctx := context.Background()
	st, blobs, ag := newJobStore(t)
	seedRecent(t, st, blobs, ag, "20260101-g")
	base := time.Now().Unix()

	j := &Job{Store: st, Blobs: blobs, Gen: fakeHookGen{hook: "h"}, Cfg: FromEnv(), Now: base + 20*day}
	if err := j.Run(ctx, ag); err != nil {
		t.Fatalf("Run 1: %v", err)
	}

	// Drop the long_term line — the index now violates the 1:1 invariant.
	corrupt := []byte("# Long-term memory index\n")
	if err := st.PutIndex(ctx, ag, corrupt); err != nil {
		t.Fatalf("PutIndex corrupt: %v", err)
	}

	if err := j.Run(ctx, ag); err == nil {
		t.Fatal("final CommitIndex gate must reject a corrupt index (non-zero)")
	}
	after, _ := st.GetIndex(ctx, ag)
	if string(after) != string(corrupt) {
		t.Fatalf("a failed gate must not write a new index: %q", after)
	}
}
