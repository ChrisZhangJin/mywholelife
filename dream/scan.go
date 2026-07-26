package dream

import (
	"context"
	"log"
	"strings"

	"mywholelife/store"
)

// Scan reconciles FS ↔ DB ↔ index via the read-only store.ScanConsistency
// (D-07). It is report-only by default; only when repair is set does it take
// destructive action, and only for the blob-level findings it can safely fix
// through the BlobStore seam (orphan blobs, torn-compress stale .tar). Dangling
// rows and index/row mismatches are logged loudly but never silently mutated —
// row loss is unrecoverable and index reconciliation belongs to the store's
// (unexported) line helpers, so dream never hand-rolls markdown.
func (j *Job) Scan(ctx context.Context, agentID string, repair bool) (store.ScanReport, error) {
	rep, err := j.Store.ScanConsistency(ctx, agentID)
	if err != nil {
		return rep, err
	}
	for _, f := range rep.Findings {
		switch f.Kind {
		case "orphan_blob":
			log.Printf("dream: scan orphan_blob %s", f.RelPath)
			if repair {
				if err := j.Blobs.Delete(ctx, f.RelPath); err != nil {
					return rep, err
				}
			}
		case "torn_compress":
			log.Printf("dream: scan torn_compress %s", f.RelPath)
			if repair {
				zst := f.RelPath + ".tar.zst"
				ok, err := j.Blobs.Exists(ctx, zst)
				if err != nil {
					return rep, err
				}
				if ok {
					if err := j.Blobs.Delete(ctx, strings.TrimSuffix(zst, ".tar.zst")+".tar"); err != nil {
						return rep, err
					}
				}
			}
		case "dangling_row":
			log.Printf("dream: scan dangling_row %s (%s) — archive lost, unrecoverable", f.MemID, f.RelPath)
		case "index_mismatch":
			log.Printf("dream: scan index_mismatch — %s", f.Detail)
		}
	}
	return rep, nil
}
