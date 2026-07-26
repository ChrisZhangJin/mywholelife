package server

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"mywholelife/store"
)

func getRemind(t *testing.T, r *gin.Engine, id, mem string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/agent/"+id+"/remind?mem="+mem, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func tarEntries(t *testing.T, b []byte) map[string]string {
	t.Helper()
	tr := tar.NewReader(bytes.NewReader(b))
	out := map[string]string{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar next: %v", err)
		}
		content, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("tar read %s: %v", h.Name, err)
		}
		out[h.Name] = string(content)
	}
	return out
}

func projectMemID(t *testing.T, st store.MemoryStore, id string) string {
	t.Helper()
	rows, err := st.List(context.Background(), id, store.ScopeProject, "")
	if err != nil {
		t.Fatalf("list project: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want exactly one project memory, got %d", len(rows))
	}
	return rows[0].MemID
}

func TestRemindRoundTrip(t *testing.T) {
	r, st := setup(t)
	ctx := context.Background()
	id := register(t, r, "erin")
	skill := "---\nname: demo\ndescription: demo brief\n---\nbody\n"
	body := makeTar(t, map[string]string{"SKILL.md": skill})
	if code := postMemory(t, r, id, "?scope=project&project=demo", body); code != http.StatusNoContent {
		t.Fatalf("post memory: status %d, want 204", code)
	}
	memID := projectMemID(t, st, id)

	if err := st.Compress(ctx, id, memID); err != nil {
		t.Fatalf("compress: %v", err)
	}
	pre, err := st.Get(ctx, id, memID)
	if err != nil {
		t.Fatalf("get after compress: %v", err)
	}
	if pre.State != store.StateLongTerm {
		t.Fatalf("state after compress = %q, want long_term", pre.State)
	}

	w := getRemind(t, r, id, memID)
	if w.Code != http.StatusOK {
		t.Fatalf("remind: status %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/x-tar" {
		t.Fatalf("remind content-type = %q, want application/x-tar", ct)
	}
	entries := tarEntries(t, w.Body.Bytes())
	if got := entries["SKILL.md"]; got != skill {
		t.Fatalf("remind SKILL.md = %q, want %q", got, skill)
	}

	post, err := st.Get(ctx, id, memID)
	if err != nil {
		t.Fatalf("get after remind: %v", err)
	}
	if post.State != store.StateRecent {
		t.Fatalf("state after remind = %q, want recent", post.State)
	}
	if post.AccessTime < pre.AccessTime {
		t.Fatalf("access_time after remind = %d, want >= %d", post.AccessTime, pre.AccessTime)
	}

	_, initEntries := getInit(t, r, id)
	if idxRaw := initEntries["memory/long-term-memory.md"]; strings.Contains(idxRaw, memID) {
		t.Fatalf("long-term index still lists reheated memory %q: %q", memID, idxRaw)
	}
	found := false
	for name := range initEntries {
		if strings.HasPrefix(name, "skills/"+memID+"/") {
			found = true
		}
	}
	if !found {
		t.Fatalf("init zip missing skills/%s/* after remind; got %v", memID, keys(initEntries))
	}
}

func TestRemindRejectsBadKeys(t *testing.T) {
	r, _ := setup(t)
	id := register(t, r, "frank")
	if w := getRemind(t, r, "..", "demo"); w.Code != http.StatusBadRequest {
		t.Fatalf("bad id: status %d, want 400", w.Code)
	}
	if w := getRemind(t, r, id, "../evil"); w.Code != http.StatusBadRequest {
		t.Fatalf("bad mem: status %d, want 400", w.Code)
	}
}

func TestRemindNotFound(t *testing.T) {
	r, _ := setup(t)
	id := register(t, r, "grace")
	if w := getRemind(t, r, "nonexistentagent", "demo"); w.Code != http.StatusNotFound {
		t.Fatalf("unknown agent: status %d, want 404", w.Code)
	}
	if w := getRemind(t, r, id, "nosuchmem"); w.Code != http.StatusNotFound {
		t.Fatalf("unknown memory: status %d, want 404", w.Code)
	}
}

func TestInitReflectsCompressedIndex(t *testing.T) {
	r, st := setup(t)
	ctx := context.Background()
	id := register(t, r, "heidi")
	body := makeTar(t, map[string]string{
		"SKILL.md": "---\nname: demo\ndescription: demo brief\n---\nbody\n",
	})
	if code := postMemory(t, r, id, "?scope=project&project=demo", body); code != http.StatusNoContent {
		t.Fatalf("post memory: status %d, want 204", code)
	}
	memID := projectMemID(t, st, id)
	if err := st.Compress(ctx, id, memID); err != nil {
		t.Fatalf("compress: %v", err)
	}
	_, entries := getInit(t, r, id)
	idxRaw := entries["memory/long-term-memory.md"]
	if !strings.Contains(idxRaw, memID) {
		t.Fatalf("long-term index missing compressed memory %q: %q", memID, idxRaw)
	}
	if strings.Contains(idxRaw, "(none yet)") {
		t.Fatalf("long-term index still shows placeholder: %q", idxRaw)
	}
}
