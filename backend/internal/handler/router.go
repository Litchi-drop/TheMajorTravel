package handler

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/Litchi-drop/TheMajorTravel/backend/internal/config"
	"github.com/Litchi-drop/TheMajorTravel/backend/internal/middleware"
	"github.com/Litchi-drop/TheMajorTravel/backend/internal/repository"
	"github.com/Litchi-drop/TheMajorTravel/backend/internal/service"
	"github.com/Litchi-drop/TheMajorTravel/backend/web"
)

// NewRouter 组装依赖与全部路由（报告 §6）
func NewRouter(cfg *config.Config, db *gorm.DB, rdb *redis.Client) *gin.Engine {
	r := gin.Default()

	users := repository.NewUserRepo(db)
	jwtSvc := service.NewJWTService(cfg.JWTSecret)
	emailSvc := service.NewEmailService(cfg, rdb)
	authSvc := service.NewAuthService(cfg, users, rdb, jwtSvc, emailSvc)
	authH := NewAuthHandler(authSvc, emailSvc)
	tripSvc := service.NewTripService(
		repository.NewTripRepo(db), repository.NewPlaceRepo(db),
		repository.NewRevisionRepo(db), users, rdb)
	tripH := NewTripHandler(tripSvc)

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

			// Step 2：行程与点位（全部登录态）
			authed.GET("/trip", tripH.GetTrip)
			authed.GET("/trip/version", tripH.Version)
			authed.POST("/places", tripH.CreatePlace)
			authed.PATCH("/places/:id", tripH.UpdatePlace)
			authed.DELETE("/places/:id", tripH.DeletePlace)
			authed.GET("/places/:id/revisions", tripH.Revisions)
		}
	}

	// 过渡页静态托管：/api 外一律回落 index.html（SPA 兜底）。
	// 不走 FileFromFS——http.FileServer 会把 */index.html 301 成目录路径
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			Err(c, http.StatusNotFound, "接口不存在")
			return
		}
		index, err := fs.ReadFile(web.Dist(), "index.html")
		if err != nil {
			Err(c, http.StatusInternalServerError, "页面资源缺失")
			return
		}
		c.Header("Cache-Control", "no-store")
		c.Data(http.StatusOK, "text/html; charset=utf-8", index)
	})

	return r
}
