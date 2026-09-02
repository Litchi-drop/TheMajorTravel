package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// GET /api/health — 存活探测；tripVersion 读 Redis（mt:trip:version，Step 2 起有写入；无值视为 0）
func health(c *gin.Context, rdb *redis.Client) {
	version, err := rdb.Get(c, "mt:trip:version").Int()
	if err != nil {
		version = 0
	}
	OK(c, gin.H{"status": "up", "tripVersion": version})
}
