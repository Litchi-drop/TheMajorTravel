package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/Litchi-drop/TheMajorTravel/backend/internal/service"
)

// JWTAuth 校验 Authorization: Bearer <token>，并先查登出黑名单（mt:jwt:black:{jti}）
func JWTAuth(jwt *service.JWTService, rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			c.AbortWithStatusJSON(401, gin.H{"ok": false, "error": "请先登录"})
			return
		}
		claims, err := jwt.Parse(strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"ok": false, "error": "登录已过期，请重新登录"})
			return
		}
		if n, _ := rdb.Exists(c, "mt:jwt:black:"+claims.JTI).Result(); n > 0 {
			c.AbortWithStatusJSON(401, gin.H{"ok": false, "error": "登录已过期，请重新登录"})
			return
		}
		c.Set("uid", claims.UID)
		c.Set("jti", claims.JTI)
		c.Set("jtiExpiresAt", claims.ExpiresAt.Time)
		c.Next()
	}
}
