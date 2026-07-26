package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"mywholelife/store"
)

func newTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	blobs := store.NewBlobStore(t.TempDir())
	st, err := store.Open(filepath.Join(t.TempDir(), "idx.db"), blobs)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	return NewRouter(st, blobs)
}

func TestRegisterAgent(t *testing.T) {
	r := newTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/agent/register", nil)
	req.Header.Set("X-Agent-Name", "alice")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("register: status = %d, want 200", w.Code)
	}
	id := strings.TrimSpace(w.Body.String())
	if len(id) < 32 || !strings.Contains(id, "-") {
		t.Fatalf("register: body = %q, want a UUID", id)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("register: Content-Type = %q, want text/plain", ct)
	}

	req = httptest.NewRequest(http.MethodPost, "/agent/register", nil)
	req.Header.Set("X-Agent-Name", "alice")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("duplicate register: status = %d, want 409", w.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/agent/register", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing name: status = %d, want 400", w.Code)
	}
}
