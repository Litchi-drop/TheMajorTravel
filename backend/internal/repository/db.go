package repository

import (
	"context"
	"fmt"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Litchi-drop/TheMajorTravel/backend/internal/model"
)

// OpenDB 连接 PostgreSQL；连接问题在此快速失败，不拖到首个请求
func OpenDB(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("连接 PostgreSQL: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("Ping PostgreSQL: %w", err)
	}
	return db, nil
}

// Migrate 建表/补列（幂等）；唯一性用 lower(email) 函数索引，
// 与报告 §5.2 的 idx_users_email_lower 等价
func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&model.User{}, &model.Trip{}, &model.Day{}, &model.Place{}, &model.PlaceRevision{}); err != nil {
		return fmt.Errorf("AutoMigrate: %w", err)
	}
	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_lower ON users(lower(email))`).Error; err != nil {
		return fmt.Errorf("创建 email 唯一索引: %w", err)
	}
	log.Println("数据库迁移完成: users, trips, days, places, place_revisions")
	return nil
}
