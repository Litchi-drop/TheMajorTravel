package model

import "time"

// User 对应报告 §5.2 users 表；email 写入前统一转小写，
// 唯一性由 lower(email) 函数唯一索引保证（见 repository.Migrate）
type User struct {
	ID           int64   `gorm:"primaryKey"`
	Email        string  `gorm:"size:255;not null"`
	PasswordHash string  `gorm:"size:255;not null"` // bcrypt，明文绝不落库
	Nickname     string  `gorm:"size:64;not null"`
	AvatarEmoji  string  `gorm:"size:16;not null"`
	Phone        *string `gorm:"size:32;uniqueIndex"` // 二期手机号注册/短信登录时启用
	Role         string  `gorm:"size:16;not null;default:member"`
	CreatedAt    time.Time
	LastLoginAt  *time.Time
}
