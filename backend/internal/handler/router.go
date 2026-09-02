package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/Litchi-drop/TheMajorTravel/backend/internal/config"
)

// NewRouter 组装全部路由；cfg 供 Step 1 的 JWT/限流中间件使用
func NewRouter(cfg *config.Config) *gin.Engine {
	r := gin.Default()

	api := r.Group("/api")
	{
		api.GET("/health", health)
	}

	return r
}
