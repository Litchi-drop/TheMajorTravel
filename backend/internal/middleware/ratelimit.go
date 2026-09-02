package middleware

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// IPRateLimit 同 IP 每日 limit 次的固定窗口限流（Redis INCR + 首次 EXPIRE，报告 §4.3 三档之一）
func IPRateLimit(rdb *redis.Client, scope string, limit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := fmt.Sprintf("mt:rl:%s:%s:%s", scope, c.ClientIP(), time.Now().Format("20060102"))
		n, err := rdb.Incr(c, key).Result()
		if err == nil && n == 1 {
			rdb.Expire(c, key, 24*time.Hour)
		}
		if n > limit {
			c.AbortWithStatusJSON(429, gin.H{"ok": false, "error": "请求太频繁，今日次数已用完，请明天再试"})
			return
		}
		c.Next()
	}
}
