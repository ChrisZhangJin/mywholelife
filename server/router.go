package server

import (
	"github.com/gin-gonic/gin"

	"mywholelife/store"
)

func NewRouter(st store.MemoryStore, bl store.BlobStore) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.POST("/agent/register", registerAgent(st))
	r.POST("/agent/:id/memory", writeMemory(st, bl))
	r.GET("/agent/:id/init", initBundle(st, bl))
	return r
}
