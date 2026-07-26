package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"

	_ "modernc.org/sqlite"
)

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
