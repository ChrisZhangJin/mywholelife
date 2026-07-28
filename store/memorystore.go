package store

import (
	"context"
	"database/sql"
	"errors"
)

const (
	ScopeGlobal  = "global"
	ScopeProject = "project"

	StateRecent    = "recent"
	StateLongTerm  = "long_term"
	StateTombstone = "tombstone"
)

var (
	ErrNotImplemented = errors.New("store: not implemented until phase 3")
	ErrNotFound       = errors.New("store: not found")
)

type Agent struct {
	ID        string
	Name      string
	CreatedAt int64
}

type Memory struct {
	MemID      string
	AgentID    string
	Scope      string
	State      string
	AccessTime int64
	Pinned     bool
	Brief      string
	RelPath    string
	CreatedAt  int64
	DeletedAt  sql.NullInt64
}

type MemoryStore interface {
	RegisterAgent(ctx context.Context, name string) (Agent, error)
	GetAgent(ctx context.Context, id string) (Agent, error)
	GetAgentByName(ctx context.Context, name string) (Agent, error)
	ReserveProjectMemID(ctx context.Context, agentID, base string) (string, error)
	Put(ctx context.Context, m Memory) error
	Get(ctx context.Context, agentID, memID string) (Memory, error)
	List(ctx context.Context, agentID, scope, state string) ([]Memory, error)
	Touch(ctx context.Context, agentID, memID string) error
	Compress(ctx context.Context, agentID, memID, hook string) error
	Reheat(ctx context.Context, agentID, memID string) error
	Forget(ctx context.Context, agentID, memID, state string) error
	SoftDelete(ctx context.Context, agentID, memID string, when int64) error
	HardDelete(ctx context.Context, agentID, memID string) error
	GetIndex(ctx context.Context, agentID string) ([]byte, error)
	PutIndex(ctx context.Context, agentID string, content []byte) error
	CommitIndex(ctx context.Context, agentID string, content []byte) error
	ScanConsistency(ctx context.Context, agentID string) (ScanReport, error)
}

// Inconsistency is one finding from ScanConsistency: an FS/DB/index divergence
// reported (never auto-repaired) so the dream job can reconcile it (D-07).
type Inconsistency struct {
	Kind    string // orphan_blob | dangling_row | torn_compress | index_mismatch
	MemID   string
	RelPath string
	Detail  string
}

type ScanReport struct {
	Findings []Inconsistency
}

type BlobStore interface {
	PutFolder(ctx context.Context, relPath string, data []byte) error
	GetFolder(ctx context.Context, relPath string) ([]byte, error)
	Delete(ctx context.Context, relPath string) error
	Exists(ctx context.Context, relPath string) (bool, error)
	Walk(ctx context.Context, relPath string) ([]string, error)
}
