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

	"github.com/Litchi-drop/TheMajorTravel/backend/internal/config"
	"github.com/Litchi-drop/TheMajorTravel/backend/internal/handler"
)

func main() {
	cfg := config.Load()

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: handler.NewRouter(cfg),
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
