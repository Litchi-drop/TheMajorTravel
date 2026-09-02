package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/Litchi-drop/TheMajorTravel/backend/internal/config"
	"github.com/Litchi-drop/TheMajorTravel/backend/internal/middleware"
	"github.com/Litchi-drop/TheMajorTravel/backend/internal/repository"
	"github.com/Litchi-drop/TheMajorTravel/backend/internal/service"
)

// NewRouter 组装依赖与全部路由（报告 §6）
func NewRouter(cfg *config.Config, db *gorm.DB, rdb *redis.Client) *gin.Engine {
	r := gin.Default()

	users := repository.NewUserRepo(db)
	jwtSvc := service.NewJWTService(cfg.JWTSecret)
	emailSvc := service.NewEmailService(cfg, rdb)
	authSvc := service.NewAuthService(cfg, users, rdb, jwtSvc, emailSvc)
	authH := NewAuthHandler(authSvc, emailSvc)

	api := r.Group("/api")
	{
		api.GET("/health", func(c *gin.Context) { health(c, rdb) })

		auth := api.Group("/auth")
		{
			// 报告 §4.3 限流三档：验证码接口同 IP 每日 20 条；注册/登录加同 IP 每日 30 次兜底
			auth.POST("/email/code", middleware.IPRateLimit(rdb, "emailcode", 20), authH.SendEmailCode)
			auth.POST("/register", middleware.IPRateLimit(rdb, "register", 30), authH.Register)
			auth.POST("/login", middleware.IPRateLimit(rdb, "login", 30), authH.Login)
		}

		authed := api.Group("", middleware.JWTAuth(jwtSvc, rdb))
		{
			authed.POST("/auth/logout", authH.Logout)
			authed.GET("/me", authH.Me)
		}
	}

	return r
}
