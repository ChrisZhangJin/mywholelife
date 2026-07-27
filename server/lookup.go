package server

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"mywholelife/store"
)

func lookupAgent(st store.MemoryStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Query("name")
		if name == "" {
			c.String(http.StatusBadRequest, "missing name")
			return
		}
		if !validKey(name) {
			c.String(http.StatusBadRequest, "invalid name")
			return
		}
		agent, err := st.GetAgentByName(c, name)
		if errors.Is(err, store.ErrNotFound) {
			c.String(http.StatusNotFound, "unknown name")
			return
		} else if err != nil {
			c.String(http.StatusInternalServerError, "lookup failed")
			return
		}
		c.String(http.StatusOK, agent.ID)
	}
}
