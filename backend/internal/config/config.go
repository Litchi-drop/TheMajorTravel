package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port      string // HTTP 监听端口
	PgDSN     string // PostgreSQL 连接串
	RedisAddr string // Redis 地址
	JWTSecret string // HS256 签名密钥，只来自环境变量/.env，绝不硬编码

	InviteCode       string // 注册邀请码（一期小队制）
	EmailCodeEnabled bool   // 邮箱验证码开关；关闭时注册免验证码（开发期默认）
	SMTPHost         string
	SMTPPort         string // 465=隐式 TLS，587/25=明文+自动 STARTTLS
	SMTPUser         string // 发件邮箱
	SMTPPass         string // SMTP 授权码
}

// Load 读取环境变量；本地开发时自动加载 backend/.env（不存在则静默跳过，生产直接注入环境变量）
func Load() *Config {
	_ = godotenv.Load()

	cfg := &Config{
		Port:      getenv("MT_PORT", "8080"),
		PgDSN:     getenv("MT_PG_DSN", "postgres://postgres:postgres@localhost:5432/major_travel?sslmode=disable"),
		RedisAddr: getenv("MT_REDIS_ADDR", "localhost:6379"),
		JWTSecret: getenv("MT_JWT_SECRET", ""),

		InviteCode:       getenv("MT_INVITE_CODE", ""),
		EmailCodeEnabled: getbool("MT_EMAIL_CODE_ENABLED", false),
		SMTPHost:         getenv("MT_SMTP_HOST", ""),
		SMTPPort:         getenv("MT_SMTP_PORT", "465"),
		SMTPUser:         getenv("MT_SMTP_USER", ""),
		SMTPPass:         getenv("MT_SMTP_PASS", ""),
	}
	if cfg.InviteCode == "" {
		log.Println("警告: MT_INVITE_CODE 未设置，注册功能将拒绝所有人（正式值写入本机 .env）")
	}
	return cfg
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getbool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}
