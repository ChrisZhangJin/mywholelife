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
	Put(ctx context.Context, m Memory) error
	Get(ctx context.Context, agentID, memID string) (Memory, error)
	List(ctx context.Context, agentID, scope, state string) ([]Memory, error)
	Touch(ctx context.Context, agentID, memID string) error
	Compress(ctx context.Context, agentID, memID string) error
	Reheat(ctx context.Context, agentID, memID string) error
	Forget(ctx context.Context, agentID, memID, state string) error
	GetIndex(ctx context.Context, agentID string) ([]byte, error)
	PutIndex(ctx context.Context, agentID string, content []byte) error
}

type BlobStore interface {
	PutFolder(ctx context.Context, relPath string, data []byte) error
	GetFolder(ctx context.Context, relPath string) ([]byte, error)
	Delete(ctx context.Context, relPath string) error
	Exists(ctx context.Context, relPath string) (bool, error)
}
