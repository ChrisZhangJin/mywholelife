package store

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const driverPkg = "modernc.org/sqlite"

// moduleRoot walks up from the test's working directory (the package dir) to
// the directory containing go.mod.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found walking up from working directory")
		}
		dir = parent
	}
}

// TestSeamDriverConfinement enforces the STORE-02/D-01 seam: the SQLite driver
// module may only be imported from files under store/, and exactly one file
// blank-imports the driver to register it.
func TestSeamDriverConfinement(t *testing.T) {
	root := moduleRoot(t)
	fset := token.NewFileSet()

	var importers []string      // repo-relative paths importing the driver module
	var blankImporters []string // files blank-importing the driver package

	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if p != root && (strings.HasPrefix(name, ".") || name == "vendor" || name == "testdata") {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, p, nil, parser.ImportsOnly)
		if perr != nil {
			return perr
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		for _, spec := range f.Imports {
			path := strings.Trim(spec.Path.Value, `"`)
			if path != driverPkg && !strings.HasPrefix(path, driverPkg+"/") {
				continue
			}
			importers = append(importers, rel)
			if path == driverPkg && spec.Name != nil && spec.Name.Name == "_" {
				blankImporters = append(blankImporters, rel)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	for _, rel := range importers {
		clean := strings.TrimPrefix(rel, "./")
		if !strings.HasPrefix(clean, "store/") {
			t.Errorf("driver imported outside store/: %s", rel)
		}
	}

	if len(blankImporters) != 1 {
		t.Fatalf("want exactly one file blank-importing %q, found %d: %v",
			driverPkg, len(blankImporters), blankImporters)
	}
}
