package server

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"mywholelife/store"
)

func registerAgent(st store.MemoryStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.GetHeader("X-Agent-Name")
		if name == "" {
			c.String(http.StatusBadRequest, "missing X-Agent-Name")
			return
		}
		agent, err := st.RegisterAgent(c, name)
		if errors.Is(err, store.ErrDuplicateName) {
			c.String(http.StatusConflict, "name already registered")
			return
		}
		if err != nil {
			c.String(http.StatusInternalServerError, "register failed")
			return
		}
		c.String(http.StatusOK, agent.ID)
	}
}
