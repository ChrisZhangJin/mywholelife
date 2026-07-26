package store

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"path"
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
	if _, err := db.ExecContext(context.Background(), schemaSQL); err != nil {
		db.Close()
		return nil, err
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

func (s *sqliteStore) Put(ctx context.Context, m Memory) error {
	now := time.Now().Unix()
	pinned := int64(0)
	if m.Pinned {
		pinned = 1
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		memID := m.MemID
		if m.Scope == ScopeProject {
			id, err := nextProjectMemID(ctx, tx, m.AgentID, m.MemID)
			if err != nil {
				return err
			}
			memID = id
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO memories(mem_id,agent_id,scope,state,access_time,pinned,brief,rel_path,created_at)
			 VALUES(?,?,?,?,?,?,?,?,?)
			 ON CONFLICT(mem_id) DO UPDATE SET
			   state=excluded.state, access_time=excluded.access_time,
			   brief=excluded.brief, rel_path=excluded.rel_path`,
			memID, m.AgentID, m.Scope, StateRecent, now, pinned, m.Brief, m.RelPath, now)
		return err
	})
}

// nextProjectMemID returns the first free YYYYMMDD-projectName key for the
// agent, appending -2/-3/... on same-day collisions (D-06). The free key is
// computed inside the write transaction (check-then-suffix, never a PK-error
// retry).
func nextProjectMemID(ctx context.Context, tx *sql.Tx, agentID, base string) (string, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT mem_id FROM memories WHERE agent_id = ? AND (mem_id = ? OR mem_id LIKE ?)`,
		agentID, base, base+"-%")
	if err != nil {
		return "", err
	}
	defer rows.Close()
	taken := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return "", err
		}
		taken[id] = true
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if !taken[base] {
		return base, nil
	}
	for n := 2; ; n++ {
		cand := fmt.Sprintf("%s-%d", base, n)
		if !taken[cand] {
			return cand, nil
		}
	}
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
		&m.AccessTime, &pinned, &brief, &relPath, &m.CreatedAt)
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
		`SELECT mem_id, agent_id, scope, state, access_time, pinned, brief, rel_path, created_at
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
	q := `SELECT mem_id, agent_id, scope, state, access_time, pinned, brief, rel_path, created_at
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
	return s.blobs.PutFolder(ctx, indexPath(agentID), content)
}

func (s *sqliteStore) Compress(ctx context.Context, agentID, memID string) error {
	return ErrNotImplemented
}

func (s *sqliteStore) Reheat(ctx context.Context, agentID, memID string) error {
	return ErrNotImplemented
}
