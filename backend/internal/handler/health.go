package handler

import "github.com/gin-gonic/gin"

// GET /api/health — 存活探测；tripVersion 后续从 Redis 读取（Step 2）
func health(c *gin.Context) {
	OK(c, gin.H{"status": "up", "tripVersion": 0})
}
