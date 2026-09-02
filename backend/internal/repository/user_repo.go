package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/Litchi-drop/TheMajorTravel/backend/internal/model"
)

var (
	ErrNotFound   = errors.New("record not found")
	ErrDuplicated = errors.New("duplicated")
)

type UserRepo struct {
	db *gorm.DB
}

func NewUserRepo(db *gorm.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(ctx context.Context, u *model.User) error {
	if err := r.db.WithContext(ctx).Create(u).Error; err != nil {
		// 23505 = PG 唯一约束冲突；前置查询挡住绝大多数重复，这里是并发注册的兜底
		if strings.Contains(err.Error(), "23505") {
			return ErrDuplicated
		}
		return fmt.Errorf("创建用户: %w", err)
	}
	return nil
}

func (r *UserRepo) ByEmail(ctx context.Context, email string) (*model.User, error) {
	var u model.User
	err := r.db.WithContext(ctx).Where("lower(email) = ?", strings.ToLower(email)).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("按邮箱查询用户: %w", err)
	}
	return &u, nil
}

func (r *UserRepo) ByID(ctx context.Context, id int64) (*model.User, error) {
	var u model.User
	err := r.db.WithContext(ctx).First(&u, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("按 ID 查询用户: %w", err)
	}
	return &u, nil
}

func (r *UserRepo) TouchLastLogin(ctx context.Context, id int64, t time.Time) error {
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Update("last_login_at", t).Error
}
