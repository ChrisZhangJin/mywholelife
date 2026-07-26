package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

var ErrUnsafePath = errors.New("store: rel_path escapes data root")

var _ BlobStore = (*localBlobStore)(nil)

type localBlobStore struct {
	root string
}

func NewBlobStore(root string) *localBlobStore {
	return &localBlobStore{root: filepath.Clean(root)}
}

// resolve joins relPath under the data root and rejects any relPath that is
// absolute or that escapes the root (D-09 confinement, RESEARCH V12). The
// cleaned absolute path must stay prefixed by the data root.
func (b *localBlobStore) resolve(relPath string) (string, error) {
	if filepath.IsAbs(relPath) {
		return "", ErrUnsafePath
	}
	full := filepath.Clean(filepath.Join(b.root, relPath))
	if full != b.root && !strings.HasPrefix(full, b.root+string(os.PathSeparator)) {
		return "", ErrUnsafePath
	}
	return full, nil
}

func (b *localBlobStore) PutFolder(_ context.Context, relPath string, data []byte) error {
	full, err := b.resolve(relPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, data, 0o644)
}

func (b *localBlobStore) GetFolder(_ context.Context, relPath string) ([]byte, error) {
	full, err := b.resolve(relPath)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(full)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	return data, err
}

func (b *localBlobStore) Delete(_ context.Context, relPath string) error {
	full, err := b.resolve(relPath)
	if err != nil {
		return err
	}
	return os.RemoveAll(full)
}

func (b *localBlobStore) Exists(_ context.Context, relPath string) (bool, error) {
	full, err := b.resolve(relPath)
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(full); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
