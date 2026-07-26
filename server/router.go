package server

import (
	"github.com/gin-gonic/gin"

	"mywholelife/store"
)

func NewRouter(st store.MemoryStore, bl store.BlobStore) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.POST("/agent/register", registerAgent(st))
	return r
}
