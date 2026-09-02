package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/Litchi-drop/TheMajorTravel/backend/internal/config"
	"github.com/Litchi-drop/TheMajorTravel/backend/internal/handler"
	"github.com/Litchi-drop/TheMajorTravel/backend/internal/repository"
)

func main() {
	cfg := config.Load()
	if len(cfg.JWTSecret) < 16 {
		log.Fatal("MT_JWT_SECRET 缺失或过短（至少 16 字符），拒绝启动——在 backend/.env 配置后重试")
	}

	db, err := repository.OpenDB(cfg.PgDSN)
	if err != nil {
		log.Fatalf("PostgreSQL 不可用: %v", err)
	}
	if err := repository.Migrate(db); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	pingCtx, cancelPing := context.WithTimeout(context.Background(), 5*time.Second)
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		log.Fatalf("Redis 不可用: %v", err)
	}
	cancelPing()
	defer rdb.Close()

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: handler.NewRouter(cfg, db, rdb),
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("MajorTravel API 启动: http://localhost:%s/api/health", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("收到退出信号，正在优雅关闭...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
