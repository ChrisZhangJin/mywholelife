package server

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"mywholelife/store"
)

func setup(t *testing.T) (*gin.Engine, store.MemoryStore) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	blobs := store.NewBlobStore(t.TempDir())
	st, err := store.Open(filepath.Join(t.TempDir(), "idx.db"), blobs)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	return NewRouter(st, blobs), st
}

func register(t *testing.T, r *gin.Engine, name string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/agent/register", nil)
	req.Header.Set("X-Agent-Name", name)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("register %s: status %d, want 200", name, w.Code)
	}
	return strings.TrimSpace(w.Body.String())
}

func makeTar(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name: name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatalf("tar header %s: %v", name, err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("tar write %s: %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	return buf.Bytes()
}

func postMemory(t *testing.T, r *gin.Engine, id, query string, body []byte) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/agent/"+id+"/memory"+query, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/x-tar")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

func getInit(t *testing.T, r *gin.Engine, id string) (int, map[string]string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/agent/"+id+"/init", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		return w.Code, nil
	}
	zr, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	entries := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		b, _ := io.ReadAll(rc)
		rc.Close()
		entries[f.Name] = string(b)
	}
	return w.Code, entries
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestWriteThenInitRoundTrip(t *testing.T) {
	r, _ := setup(t)
	id := register(t, r, "alice")
	body := makeTar(t, map[string]string{
		"SKILL.md": "---\nname: demo\ndescription: demo brief\n---\nbody\n",
	})
	if code := postMemory(t, r, id, "?scope=project&project=demo", body); code != http.StatusNoContent {
		t.Fatalf("post memory: status %d, want 204", code)
	}
	code, entries := getInit(t, r, id)
	if code != http.StatusOK {
		t.Fatalf("init: status %d, want 200", code)
	}
	if _, ok := entries["memory/long-term-memory.md"]; !ok {
		t.Fatalf("init zip missing memory/long-term-memory.md; got %v", keys(entries))
	}
	found := false
	for name := range entries {
		if strings.HasPrefix(name, "skills/") && strings.HasSuffix(name, "-demo/SKILL.md") {
			found = true
		}
	}
	if !found {
		t.Fatalf("init zip missing skills/*-demo/SKILL.md; got %v", keys(entries))
	}
}

func TestGlobalBundle(t *testing.T) {
	r, _ := setup(t)
	id := register(t, r, "bob")
	body := makeTar(t, map[string]string{
		"SKILL.md": "---\ndescription: g\n---\nglobal body\n",
	})
	if code := postMemory(t, r, id, "?scope=global", body); code != http.StatusNoContent {
		t.Fatalf("post global: status %d, want 204", code)
	}
	_, entries := getInit(t, r, id)
	if _, ok := entries["memory/global/SKILL.md"]; !ok {
		t.Fatalf("init zip missing memory/global/SKILL.md; got %v", keys(entries))
	}
}

func TestUnknownAgent(t *testing.T) {
	r, _ := setup(t)
	body := makeTar(t, map[string]string{"SKILL.md": "---\n---\n"})
	if code := postMemory(t, r, "UNKNOWN", "?scope=project&project=x", body); code != http.StatusNotFound {
		t.Fatalf("post unknown: status %d, want 404", code)
	}
	code, _ := getInit(t, r, "UNKNOWN")
	if code != http.StatusNotFound {
		t.Fatalf("init unknown: status %d, want 404", code)
	}
}

func TestTraversalRejected(t *testing.T) {
	r, _ := setup(t)
	id := register(t, r, "carol")
	body := makeTar(t, map[string]string{
		"SKILL.md":   "---\ndescription: ok\n---\n",
		"../evil.md": "pwned",
	})
	if code := postMemory(t, r, id, "?scope=project&project=x", body); code != http.StatusBadRequest {
		t.Fatalf("traversal post: status %d, want 400", code)
	}
	_, entries := getInit(t, r, id)
	for name := range entries {
		if strings.Contains(name, "evil") {
			t.Fatalf("init zip contains evil entry %q", name)
		}
	}
}

func TestGlobalPerAgentIsolation(t *testing.T) {
	r, st := setup(t)
	a := register(t, r, "agent-a")
	b := register(t, r, "agent-b")
	ta := makeTar(t, map[string]string{"SKILL.md": "---\ndescription: A\n---\nA-global\n"})
	tb := makeTar(t, map[string]string{"SKILL.md": "---\ndescription: B\n---\nB-global\n"})
	if code := postMemory(t, r, a, "?scope=global", ta); code != http.StatusNoContent {
		t.Fatalf("post a global: status %d, want 204", code)
	}
	if code := postMemory(t, r, b, "?scope=global", tb); code != http.StatusNoContent {
		t.Fatalf("post b global: status %d, want 204", code)
	}
	_, ea := getInit(t, r, a)
	_, eb := getInit(t, r, b)
	if got := ea["memory/global/SKILL.md"]; !strings.Contains(got, "A-global") {
		t.Fatalf("agent A global = %q, want A-global", got)
	}
	if got := eb["memory/global/SKILL.md"]; !strings.Contains(got, "B-global") {
		t.Fatalf("agent B global = %q, want B-global", got)
	}
	ra, err := st.List(context.Background(), a, store.ScopeGlobal, store.StateRecent)
	if err != nil {
		t.Fatalf("list a: %v", err)
	}
	if len(ra) != 1 || ra[0].MemID != "global-"+a {
		t.Fatalf("agent A global rows = %+v, want one mem_id=global-%s", ra, a)
	}
	rb, err := st.List(context.Background(), b, store.ScopeGlobal, store.StateRecent)
	if err != nil {
		t.Fatalf("list b: %v", err)
	}
	if len(rb) != 1 || rb[0].MemID != "global-"+b {
		t.Fatalf("agent B global rows = %+v, want one mem_id=global-%s", rb, b)
	}
}
