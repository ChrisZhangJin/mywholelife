package dream

import (
	"context"
	"log"
	"sort"
	"strings"

	"mywholelife/store"
)

// Job is the one-pass dream orchestrator. It drives the Plan-01 store primitives
// through store.MemoryStore and reads memory bodies through store.BlobStore; the
// LLM (HookGen) and the clock (Now) are injected so the whole pass is hermetic
// and deterministic (RESEARCH §Test Strategy). Now is compared against each
// row's access_time/deleted_at — the job never calls time.Now itself.
type Job struct {
	Store store.MemoryStore
	Blobs store.BlobStore
	Gen   HookGen
	Cfg   Config
	Now   int64
}

// Run executes one dream pass for agentID (D-01): age scan (T1 → hook →
// Compress), forgetting scan (T2 tombstone → T3 soft-delete → post-grace
// rate-capped hard-delete), then the final validate-before-commit index gate.
// Every per-item transition is a no-op when already applied, so an interrupted
// run is safe to re-run (D-07).
func (j *Job) Run(ctx context.Context, agentID string) error {
	if err := j.ageScan(ctx, agentID); err != nil {
		return err
	}
	if err := j.forgetScan(ctx, agentID); err != nil {
		return err
	}
	content, err := j.Store.GetIndex(ctx, agentID)
	if err != nil && err != store.ErrNotFound {
		return err
	}
	return j.Store.CommitIndex(ctx, agentID, content)
}

func (j *Job) ageScan(ctx context.Context, agentID string) error {
	recents, err := j.Store.List(ctx, agentID, "", store.StateRecent)
	if err != nil {
		return err
	}
	for _, m := range recents {
		if j.exempt(m) || j.Now-m.AccessTime <= j.Cfg.T1 {
			continue
		}
		hook := j.hookFor(ctx, m)
		if err := j.Store.Compress(ctx, agentID, m.MemID, hook); err != nil {
			return err
		}
	}
	return nil
}

// hookFor asks the LLM for a per-item hook; any error/empty result degrades to
// "" so the store's Compress applies its deterministic brief-derived fallback
// (D-02). The job never hand-rolls an index line — the hook string is all it
// supplies to store.Compress.
func (j *Job) hookFor(ctx context.Context, m store.Memory) string {
	var body string
	if j.Blobs != nil {
		if b, err := j.Blobs.GetFolder(ctx, m.RelPath); err == nil {
			body = string(b)
		}
	}
	hook, err := j.Gen.Hook(ctx, m.Brief, body)
	if err != nil || strings.TrimSpace(hook) == "" {
		log.Printf("dream: hook unavailable for %s (%v); using brief-derived hook", m.MemID, err)
		return ""
	}
	return hook
}

func (j *Job) forgetScan(ctx context.Context, agentID string) error {
	longs, err := j.Store.List(ctx, agentID, "", store.StateLongTerm)
	if err != nil {
		return err
	}
	for _, m := range longs {
		if j.exempt(m) || j.Now-m.AccessTime <= j.Cfg.T2 {
			continue
		}
		if err := j.Store.Forget(ctx, agentID, m.MemID, store.StateTombstone); err != nil {
			return err
		}
	}

	tombs, err := j.Store.List(ctx, agentID, "", store.StateTombstone)
	if err != nil {
		return err
	}
	var hardDelete []store.Memory
	for _, m := range tombs {
		if j.exempt(m) {
			continue
		}
		if !m.DeletedAt.Valid {
			if j.Now-m.AccessTime > j.Cfg.T3 {
				if err := j.Store.SoftDelete(ctx, agentID, m.MemID, j.Now); err != nil {
					return err
				}
			}
			continue
		}
		if j.Now-m.DeletedAt.Int64 > j.Cfg.Grace {
			hardDelete = append(hardDelete, m)
		}
	}
	return j.hardDelete(ctx, agentID, hardDelete)
}

// hardDelete destroys grace-elapsed soft-deleted tombstones deterministically
// (sorted by memId), capped at DREAM_MAX_DELETIONS per run so a bug can never
// mass-erase the store (D-04). Every drop and every skipped survivor is logged.
func (j *Job) hardDelete(ctx context.Context, agentID string, cands []store.Memory) error {
	sort.Slice(cands, func(a, b int) bool { return cands[a].MemID < cands[b].MemID })
	for i, m := range cands {
		if i >= j.Cfg.MaxDeletions {
			log.Printf("dream: hard-delete rate cap %d reached; skipping %s (deleted_at=%d)",
				j.Cfg.MaxDeletions, m.MemID, m.DeletedAt.Int64)
			continue
		}
		log.Printf("dream: hard-delete %s age=%ds reason=grace-elapsed",
			m.MemID, j.Now-m.DeletedAt.Int64)
		if err := j.Store.HardDelete(ctx, agentID, m.MemID); err != nil {
			return err
		}
	}
	return nil
}

func (j *Job) exempt(m store.Memory) bool {
	return m.Pinned || m.Scope == store.ScopeGlobal
}
