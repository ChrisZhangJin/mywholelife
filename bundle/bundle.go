package bundle

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"path"

	"mywholelife/store"
)

func WriteInitZip(ctx context.Context, w io.Writer, st store.MemoryStore, bl store.BlobStore, agentID string) error {
	zw := zip.NewWriter(w)
	defer zw.Close()

	if b, err := bl.GetFolder(ctx, "agents/"+agentID+"/global.tar"); err == nil {
		if err := copyTarIntoZip(zw, b, "memory/global/"); err != nil {
			return err
		}
	} else if !errors.Is(err, store.ErrNotFound) {
		return err
	}

	idx, err := st.GetIndex(ctx, agentID)
	if errors.Is(err, store.ErrNotFound) {
		idx = []byte("# Long-term memory index\n\n(none yet)\n")
	} else if err != nil {
		return err
	}
	iw, err := zw.Create("memory/long-term-memory.md")
	if err != nil {
		return err
	}
	if _, err := iw.Write(idx); err != nil {
		return err
	}

	recents, err := st.List(ctx, agentID, store.ScopeProject, store.StateRecent)
	if err != nil {
		return err
	}
	for _, m := range recents {
		b, err := bl.GetFolder(ctx, m.RelPath)
		if errors.Is(err, store.ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		if err := copyTarIntoZip(zw, b, "skills/"+m.MemID+"/"); err != nil {
			return err
		}
	}
	return nil
}

func copyTarIntoZip(zw *zip.Writer, tarBytes []byte, prefix string) error {
	tr := tar.NewReader(bytes.NewReader(tarBytes))
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if h.Typeflag != tar.TypeReg || !safeEntryName(h.Name) {
			continue
		}
		fw, err := zw.Create(prefix + path.Clean(h.Name))
		if err != nil {
			return err
		}
		if _, err := io.Copy(fw, tr); err != nil {
			return err
		}
	}
}
