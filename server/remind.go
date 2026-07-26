package server

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"mywholelife/store"
)

func remind(st store.MemoryStore, bl store.BlobStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		mem := c.Query("mem")
		if !validKey(id) || !validKey(mem) {
			c.String(http.StatusBadRequest, "invalid id or mem")
			return
		}
		if _, err := st.GetAgent(c, id); errors.Is(err, store.ErrNotFound) {
			c.String(http.StatusNotFound, "unknown agent")
			return
		} else if err != nil {
			c.String(http.StatusInternalServerError, "lookup failed")
			return
		}
		if err := st.Reheat(c, id, mem); errors.Is(err, store.ErrNotFound) {
			c.String(http.StatusNotFound, "unknown memory")
			return
		} else if err != nil {
			c.String(http.StatusInternalServerError, "reheat failed")
			return
		}
		m, err := st.Get(c, id, mem)
		if err != nil {
			c.String(http.StatusInternalServerError, "lookup failed")
			return
		}
		tarBytes, err := bl.GetFolder(c, m.RelPath)
		if err != nil {
			c.String(http.StatusInternalServerError, "read failed")
			return
		}
		c.Header("Content-Disposition", `attachment; filename="`+mem+`.tar"`)
		c.Data(http.StatusOK, "application/x-tar", tarBytes)
	}
}
