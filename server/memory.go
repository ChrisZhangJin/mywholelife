package server

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"mywholelife/bundle"
	"mywholelife/store"
)

const maxUploadBytes = 8 << 20

func writeMemory(st store.MemoryStore, bl store.BlobStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		scope := c.Query("scope")
		project := c.Query("project")

		if !validKey(id) {
			c.String(http.StatusBadRequest, "invalid agent id")
			return
		}
		if scope == store.ScopeProject && project != "" && !validKey(project) {
			c.String(http.StatusBadRequest, "invalid project")
			return
		}

		if _, err := st.GetAgent(c, id); errors.Is(err, store.ErrNotFound) {
			c.String(http.StatusNotFound, "unknown agent")
			return
		} else if err != nil {
			c.String(http.StatusInternalServerError, "lookup failed")
			return
		}

		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadBytes)
		buf, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.String(http.StatusRequestEntityTooLarge, "body too large")
			return
		}

		brief, err := bundle.ValidateAndBrief(buf)
		if err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}

		var memID, rel string
		var pinned bool
		switch scope {
		case store.ScopeProject:
			if project == "" {
				c.String(http.StatusBadRequest, "missing project")
				return
			}
			memID = time.Now().Format("20060102") + "-" + project
			rel = "agents/" + id + "/projects/" + memID + ".tar"
		case store.ScopeGlobal:
			memID = "global-" + id
			rel = "agents/" + id + "/global.tar"
			pinned = true
		default:
			c.String(http.StatusBadRequest, "invalid scope")
			return
		}

		if err := bl.PutFolder(c, rel, buf); err != nil {
			c.String(http.StatusInternalServerError, "store blob")
			return
		}
		if err := st.Put(c, store.Memory{
			MemID: memID, AgentID: id, Scope: scope,
			Brief: brief, RelPath: rel, Pinned: pinned,
		}); err != nil {
			c.String(http.StatusInternalServerError, "store index")
			return
		}
		c.Status(http.StatusNoContent)
	}
}
