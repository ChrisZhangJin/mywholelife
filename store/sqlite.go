package store

import (
	"bytes"
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

var ErrDuplicateName = errors.New("store: agent name already registered")

var _ MemoryStore = (*sqliteStore)(nil)

//go:embed schema.sql
var schemaSQL string

const dsnFormat = "file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)" +
	"&_pragma=foreign_keys(ON)&_txlock=immediate"

type sqliteStore struct {
	db    *sql.DB
	blobs BlobStore
}

func Open(dbPath string, blobs BlobStore) (*sqliteStore, error) {
	db, err := sql.Open("sqlite", fmt.Sprintf(dsnFormat, dbPath))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, schemaSQL); err != nil {
		db.Close()
		return nil, err
	}
	var hasDeletedAt int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM pragma_table_info('memories') WHERE name='deleted_at'`).
		Scan(&hasDeletedAt); err != nil {
		db.Close()
		return nil, err
	}
	if hasDeletedAt == 0 {
		if _, err := db.ExecContext(ctx,
			`ALTER TABLE memories ADD COLUMN deleted_at INTEGER`); err != nil {
			db.Close()
			return nil, err
		}
	}
	return &sqliteStore{db: db, blobs: blobs}, nil
}

func (s *sqliteStore) withTx(ctx context.Context, fn func(*sql.Tx) error) (err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback()
			return
		}
		err = tx.Commit()
	}()
	return fn(tx)
}

func constraintCode(err error) int {
	var c interface{ Code() int }
	if errors.As(err, &c) {
		return c.Code()
	}
	return 0
}

func (s *sqliteStore) RegisterAgent(ctx context.Context, name string) (Agent, error) {
	a := Agent{ID: uuid.NewString(), Name: name, CreatedAt: time.Now().Unix()}
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO agents(id, name, created_at) VALUES(?, ?, ?)`,
			a.ID, a.Name, a.CreatedAt)
		return err
	})
	if err != nil {
		if constraintCode(err) == sqlite3.SQLITE_CONSTRAINT_UNIQUE {
			return Agent{}, ErrDuplicateName
		}
		return Agent{}, err
	}
	return a, nil
}

func (s *sqliteStore) GetAgent(ctx context.Context, id string) (Agent, error) {
	var a Agent
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, created_at FROM agents WHERE id = ?`, id).
		Scan(&a.ID, &a.Name, &a.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Agent{}, ErrNotFound
	}
	if err != nil {
		return Agent{}, err
	}
	return a, nil
}

func (s *sqliteStore) GetAgentByName(ctx context.Context, name string) (Agent, error) {
	var a Agent
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, created_at FROM agents WHERE name = ?`, name).
		Scan(&a.ID, &a.Name, &a.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Agent{}, ErrNotFound
	}
	if err != nil {
		return Agent{}, err
	}
	return a, nil
}

func (s *sqliteStore) Put(ctx context.Context, m Memory) error {
	now := time.Now().Unix()
	pinned := int64(0)
	if m.Pinned {
		pinned = 1
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		// memID is caller-assigned and stable (project name for project scope,
		// "global-<id>" for global). A repeated write to the same memID upserts
		// in place — the row is UPDATED, never duplicated. created_at is left
		// untouched on update so it records first-seen; access_time is bumped to
		// now to reheat recency.
		_, err := tx.ExecContext(ctx,
			`INSERT INTO memories(mem_id,agent_id,scope,state,access_time,pinned,brief,rel_path,created_at)
			 VALUES(?,?,?,?,?,?,?,?,?)
			 ON CONFLICT(mem_id) DO UPDATE SET
			   state=excluded.state, access_time=excluded.access_time,
			   brief=excluded.brief, rel_path=excluded.rel_path`,
			m.MemID, m.AgentID, m.Scope, StateRecent, now, pinned, m.Brief, m.RelPath, now)
		return err
	})
}

func (s *sqliteStore) Touch(ctx context.Context, agentID, memID string) error {
	now := time.Now().Unix()
	return s.withTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`UPDATE memories SET access_time = ? WHERE agent_id = ? AND mem_id = ?`,
			now, agentID, memID)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// agingCandidates returns recent, unpinned, non-global memories whose
// access_time is older than olderThan seconds — the aging/forgetting query.
// The pinned/global exemption (D-08) lives here so global-recent never ages.
func (s *sqliteStore) agingCandidates(ctx context.Context, agentID string, olderThan int64) ([]string, error) {
	now := time.Now().Unix()
	rows, err := s.db.QueryContext(ctx,
		`SELECT mem_id FROM memories
		 WHERE agent_id = ? AND state = 'recent' AND pinned = 0
		   AND scope <> 'global'
		   AND (? - access_time) > ?`,
		agentID, now, olderThan)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func scanMemory(sc interface{ Scan(...any) error }) (Memory, error) {
	var m Memory
	var pinned int64
	var brief, relPath sql.NullString
	err := sc.Scan(&m.MemID, &m.AgentID, &m.Scope, &m.State,
		&m.AccessTime, &pinned, &brief, &relPath, &m.CreatedAt, &m.DeletedAt)
	if err != nil {
		return Memory{}, err
	}
	m.Pinned = pinned != 0
	m.Brief = brief.String
	m.RelPath = relPath.String
	return m, nil
}

func (s *sqliteStore) Get(ctx context.Context, agentID, memID string) (Memory, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT mem_id, agent_id, scope, state, access_time, pinned, brief, rel_path, created_at, deleted_at
		 FROM memories WHERE agent_id = ? AND mem_id = ?`, agentID, memID)
	m, err := scanMemory(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Memory{}, ErrNotFound
	}
	if err != nil {
		return Memory{}, err
	}
	return m, nil
}

func (s *sqliteStore) List(ctx context.Context, agentID, scope, state string) ([]Memory, error) {
	q := `SELECT mem_id, agent_id, scope, state, access_time, pinned, brief, rel_path, created_at, deleted_at
	      FROM memories WHERE agent_id = ?`
	args := []any{agentID}
	if scope != "" {
		q += " AND scope = ?"
		args = append(args, scope)
	}
	if state != "" {
		q += " AND state = ?"
		args = append(args, state)
	}
	q += " ORDER BY access_time DESC"

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Memory
	for rows.Next() {
		m, err := scanMemory(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *sqliteStore) Forget(ctx context.Context, agentID, memID, state string) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`UPDATE memories SET state = ?
			 WHERE agent_id = ? AND mem_id = ? AND pinned = 0 AND scope <> 'global'`,
			state, agentID, memID)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n > 0 {
			return nil
		}
		// No row changed: either the memory is missing or it is exempt
		// (pinned/global-recent, D-08). Exempt rows are a silent no-op.
		var exists int
		err = tx.QueryRowContext(ctx,
			`SELECT 1 FROM memories WHERE agent_id = ? AND mem_id = ?`, agentID, memID).
			Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	})
}

func indexPath(agentID string) string {
	return path.Join("agents", agentID, "long-term-memory.md")
}

func (s *sqliteStore) GetIndex(ctx context.Context, agentID string) ([]byte, error) {
	return s.blobs.GetFolder(ctx, indexPath(agentID))
}

func (s *sqliteStore) PutIndex(ctx context.Context, agentID string, content []byte) error {
	return s.writeIndexAtomic(ctx, agentID, content)
}

// CommitIndex validates the proposed index against the agent's rows BEFORE any
// disk write (D-06): a validation failure returns the error and leaves the prior
// index byte-unchanged. On success it writes atomically via writeIndexAtomic.
func (s *sqliteStore) CommitIndex(ctx context.Context, agentID string, content []byte) error {
	rows, err := s.List(ctx, agentID, "", "")
	if err != nil {
		return err
	}
	if err := validateIndex(rows, content); err != nil {
		return err
	}
	return s.writeIndexAtomic(ctx, agentID, content)
}

// writeIndexAtomic is the single index-write path: it copies any prior index to
// <path>.bak, then writes via the blob store (localBlobStore.PutFolder does a
// tmp+rename atomic replace), so every index write is crash-safe (D-06).
func (s *sqliteStore) writeIndexAtomic(ctx context.Context, agentID string, content []byte) error {
	p := indexPath(agentID)
	if prev, err := s.blobs.GetFolder(ctx, p); err == nil {
		if err := s.blobs.PutFolder(ctx, p+".bak", prev); err != nil {
			return err
		}
	} else if !errors.Is(err, ErrNotFound) {
		return err
	}
	return s.blobs.PutFolder(ctx, p, content)
}

// SoftDelete stamps an explicit deleted_at (the grace-window start) on a
// tombstone (T3-mark, D-04). The deleted_at IS NULL guard makes a re-run a no-op
// that never resets the grace clock (D-07); a missing or exempt row is a silent
// no-op success, mirroring Forget.
func (s *sqliteStore) SoftDelete(ctx context.Context, agentID, memID string, when int64) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`UPDATE memories SET deleted_at = ?
			 WHERE agent_id = ? AND mem_id = ? AND state = 'tombstone'
			   AND deleted_at IS NULL AND pinned = 0 AND scope <> 'global'`,
			when, agentID, memID)
		return err
	})
}

// HardDelete destroys a soft-deleted tombstone with verify-before-destroy
// ordering (D-04/D-07): confirm it is a tombstone with deleted_at set, then
// remove the index line, then the blob, then the row. Each step is idempotent so
// an interrupted run converges on re-run; a non-soft-deleted or missing row is a
// no-op. Rate-limit/grace comparison is the caller's (Plan 02).
func (s *sqliteStore) HardDelete(ctx context.Context, agentID, memID string) error {
	m, err := s.Get(ctx, agentID, memID)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if m.State != StateTombstone || !m.DeletedAt.Valid {
		return nil
	}
	if err := s.removeIndex(ctx, agentID, memID); err != nil {
		return err
	}
	if err := s.blobs.Delete(ctx, m.RelPath); err != nil {
		return err
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		_, e := tx.ExecContext(ctx,
			`DELETE FROM memories WHERE agent_id = ? AND mem_id = ?`, agentID, memID)
		return e
	})
}

// ScanConsistency reconciles FS ↔ DB ↔ index read-only (D-07): it reports orphan
// blobs, dangling rows, torn compress pairs, and index/row mismatches without
// destroying anything. A clean store reports zero findings; repair sequencing is
// the dream job's (Plan 02).
func (s *sqliteStore) ScanConsistency(ctx context.Context, agentID string) (ScanReport, error) {
	var rep ScanReport
	rows, err := s.List(ctx, agentID, "", "")
	if err != nil {
		return rep, err
	}
	rowPaths := map[string]bool{}
	for _, m := range rows {
		if m.RelPath == "" {
			continue
		}
		rowPaths[m.RelPath] = true
		ok, err := s.blobs.Exists(ctx, m.RelPath)
		if err != nil {
			return rep, err
		}
		if !ok {
			rep.Findings = append(rep.Findings, Inconsistency{
				Kind: "dangling_row", MemID: m.MemID, RelPath: m.RelPath})
		}
	}
	blobs, err := s.blobs.Walk(ctx, path.Join("agents", agentID))
	if err != nil {
		return rep, err
	}
	bases := map[string]map[string]bool{}
	for _, p := range blobs {
		var base, ext string
		switch {
		case strings.HasSuffix(p, ".tar.zst"):
			base, ext = strings.TrimSuffix(p, ".tar.zst"), ".tar.zst"
		case strings.HasSuffix(p, ".tar"):
			base, ext = strings.TrimSuffix(p, ".tar"), ".tar"
		default:
			continue
		}
		if bases[base] == nil {
			bases[base] = map[string]bool{}
		}
		bases[base][ext] = true
		if !rowPaths[p] {
			rep.Findings = append(rep.Findings, Inconsistency{Kind: "orphan_blob", RelPath: p})
		}
	}
	for base, exts := range bases {
		if exts[".tar"] && exts[".tar.zst"] {
			rep.Findings = append(rep.Findings, Inconsistency{Kind: "torn_compress", RelPath: base})
		}
	}
	idxContent, err := s.GetIndex(ctx, agentID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return rep, err
	}
	if err := validateIndex(rows, idxContent); err != nil {
		rep.Findings = append(rep.Findings, Inconsistency{Kind: "index_mismatch", Detail: err.Error()})
	}
	return rep, nil
}

// Compress archives a recent memory: .tar → verified .tar.zst, state=long_term,
// index upserted (COMP-01). access_time is left untouched (compress is aging).
// The source .tar is deleted LAST, only after the read-back round-trip verifies
// and the DB pointer commits — never lose a memory to a bad compress (COMP-02,
// D-02, Pattern 1). A non-empty hook overrides the brief-derived index line and
// flows through the same upsertIndexLine sanitize/cap path; hook=="" keeps the
// Phase-3 brief-derived behavior (D-02).
func (s *sqliteStore) Compress(ctx context.Context, agentID, memID, hook string) error {
	m, err := s.Get(ctx, agentID, memID)
	if err != nil {
		return err
	}
	if hook != "" {
		m.Brief = hook
	}
	srcTar, err := s.blobs.GetFolder(ctx, m.RelPath)
	if err != nil {
		return err
	}
	comp, err := zstdCompress(srcTar)
	if err != nil {
		return err
	}
	zstPath := strings.TrimSuffix(m.RelPath, ".tar") + ".tar.zst"
	if err := s.blobs.PutFolder(ctx, zstPath, comp); err != nil {
		return err
	}
	stored, err := s.blobs.GetFolder(ctx, zstPath)
	if err == nil {
		var back []byte
		back, err = zstdDecompress(stored)
		if err == nil && !bytes.Equal(srcTar, back) {
			err = fmt.Errorf("store: compress round-trip verify failed for %s", memID)
		}
	}
	if err != nil {
		_ = s.blobs.Delete(ctx, zstPath) // drop the bad artifact; keep source
		return err
	}
	if err := s.withTx(ctx, func(tx *sql.Tx) error {
		_, e := tx.ExecContext(ctx,
			`UPDATE memories SET state=?, rel_path=? WHERE agent_id=? AND mem_id=?`,
			StateLongTerm, zstPath, agentID, memID)
		return e
	}); err != nil {
		return err
	}
	if err := s.upsertIndex(ctx, agentID, m); err != nil {
		return err
	}
	return s.blobs.Delete(ctx, m.RelPath) // delete source LAST
}

// Reheat is the inverse (D-03): .tar.zst → verified .tar, state=recent,
// access_time=now, index line removed, .tar.zst deleted LAST. An already-recent
// memory is a no-op Touch (idempotent — robust to races with the dream job).
func (s *sqliteStore) Reheat(ctx context.Context, agentID, memID string) error {
	m, err := s.Get(ctx, agentID, memID)
	if err != nil {
		return err
	}
	if m.State == StateRecent {
		return s.Touch(ctx, agentID, memID)
	}
	comp, err := s.blobs.GetFolder(ctx, m.RelPath)
	if err != nil {
		return err
	}
	tar, err := zstdDecompress(comp)
	if err != nil {
		return err
	}
	tarPath := strings.TrimSuffix(m.RelPath, ".tar.zst") + ".tar"
	if err := s.blobs.PutFolder(ctx, tarPath, tar); err != nil {
		return err
	}
	stored, err := s.blobs.GetFolder(ctx, tarPath)
	if err != nil || !bytes.Equal(tar, stored) {
		_ = s.blobs.Delete(ctx, tarPath)
		if err == nil {
			err = fmt.Errorf("store: reheat round-trip verify failed for %s", memID)
		}
		return err
	}
	now := time.Now().Unix()
	if err := s.withTx(ctx, func(tx *sql.Tx) error {
		_, e := tx.ExecContext(ctx,
			`UPDATE memories SET state=?, rel_path=?, access_time=?, deleted_at=NULL WHERE agent_id=? AND mem_id=?`,
			StateRecent, tarPath, now, agentID, memID)
		return e
	}); err != nil {
		return err
	}
	if err := s.removeIndex(ctx, agentID, memID); err != nil {
		return err
	}
	return s.blobs.Delete(ctx, m.RelPath) // delete .tar.zst LAST
}

func (s *sqliteStore) upsertIndex(ctx context.Context, agentID string, m Memory) error {
	content, err := s.GetIndex(ctx, agentID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	return s.PutIndex(ctx, agentID, upsertIndexLine(content, m))
}

func (s *sqliteStore) removeIndex(ctx context.Context, agentID, memID string) error {
	content, err := s.GetIndex(ctx, agentID)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.PutIndex(ctx, agentID, removeIndexLine(content, memID))
}
