package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port      string // HTTP 监听端口
	PgDSN     string // PostgreSQL 连接串
	RedisAddr string // Redis 地址
	JWTSecret string // HS256 签名密钥，只来自环境变量/.env，绝不硬编码
	SeedUsers string // Step 0 占位：Step 1 改邀请码制后移除
}

// Load 读取环境变量；本地开发时自动加载 backend/.env（不存在则静默跳过，生产用环境变量）
func Load() *Config {
	_ = godotenv.Load()

	cfg := &Config{
		Port:      getenv("MT_PORT", "8080"),
		PgDSN:     getenv("MT_PG_DSN", "postgres://postgres:postgres@localhost:5432/major_travel?sslmode=disable"),
		RedisAddr: getenv("MT_REDIS_ADDR", "localhost:6379"),
		JWTSecret: getenv("MT_JWT_SECRET", ""),
		SeedUsers: getenv("MT_SEED_USERS", ""),
	}
	if cfg.JWTSecret == "" {
		log.Println("警告: MT_JWT_SECRET 未设置，认证功能（Step 1）上线前必须在 .env 配置")
	}
	return cfg
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
