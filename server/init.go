package server

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"mywholelife/bundle"
	"mywholelife/store"
)

func initBundle(st store.MemoryStore, bl store.BlobStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if !validKey(id) {
			c.String(http.StatusBadRequest, "invalid agent id")
			return
		}
		if _, err := st.GetAgent(c, id); errors.Is(err, store.ErrNotFound) {
			c.String(http.StatusNotFound, "unknown agent")
			return
		} else if err != nil {
			c.String(http.StatusInternalServerError, "lookup failed")
			return
		}
		c.Header("Content-Type", "application/zip")
		c.Header("Content-Disposition", `attachment; filename="init.zip"`)
		if err := bundle.WriteInitZip(c, c.Writer, st, bl, id); err != nil {
			_ = c.Error(err)
		}
	}
}
