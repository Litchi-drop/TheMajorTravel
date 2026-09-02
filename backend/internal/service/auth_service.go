package service

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log"
	"net/mail"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"github.com/Litchi-drop/TheMajorTravel/backend/internal/config"
	"github.com/Litchi-drop/TheMajorTravel/backend/internal/model"
	"github.com/Litchi-drop/TheMajorTravel/backend/internal/repository"
)

// 业务错误文案直接可展示（报告 §6 约定）；handler 只负责映射 HTTP 状态码
var (
	ErrInvite       = errors.New("邀请码不对，请联系发起人")
	ErrEmailFormat  = errors.New("邮箱格式不正确")
	ErrPasswordRule = errors.New("密码至少 8 位，最长 72 字节")
	ErrNickname     = errors.New("请填写昵称")
	ErrCodeRequired = errors.New("请输入邮箱验证码")
	ErrEmailUsed    = errors.New("该邮箱已注册，请直接登录")
	ErrBadCreds     = errors.New("邮箱或密码不对")
	ErrLoginLocked  = errors.New("密码错误次数过多，请 15 分钟后再试")
	ErrCodeCooldown = errors.New("验证码已发送，请 60 秒后再试")
	ErrCodeQuota    = errors.New("今日验证码条数已达上限，明天再试")
	ErrCodeWrong    = errors.New("验证码错误或已过期")
	ErrSMTPFailed   = errors.New("验证码发送失败，请稍后重试")
)

const (
	bcryptCost         = 10
	loginFailLimit     = 5
	loginLockDuration  = 15 * time.Minute
	defaultAvatarEmoji = "🙂"
)

// 查无此用户时也走一遍同代价的 bcrypt 比较，避免通过响应时间探测邮箱是否已注册
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("timing-equalizer"), bcryptCost)

type AuthService struct {
	cfg   *config.Config
	users *repository.UserRepo
	rdb   *redis.Client
	jwt   *JWTService
	email *EmailService
}

func NewAuthService(cfg *config.Config, users *repository.UserRepo, rdb *redis.Client, jwt *JWTService, email *EmailService) *AuthService {
	return &AuthService{cfg: cfg, users: users, rdb: rdb, jwt: jwt, email: email}
}

type RegisterInput struct {
	Email      string
	Password   string
	Nickname   string
	InviteCode string
	Code       string // MT_EMAIL_CODE_ENABLED=true 时必填
}

func (s *AuthService) Register(ctx context.Context, in RegisterInput) (*model.User, string, error) {
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.Nickname = strings.TrimSpace(in.Nickname)

	if _, err := mail.ParseAddress(in.Email); err != nil {
		return nil, "", ErrEmailFormat
	}
	if l := len(in.Password); l < 8 || l > 72 {
		return nil, "", ErrPasswordRule
	}
	if in.Nickname == "" {
		return nil, "", ErrNickname
	}
	if s.cfg.InviteCode == "" || subtle.ConstantTimeCompare([]byte(in.InviteCode), []byte(s.cfg.InviteCode)) != 1 {
		return nil, "", ErrInvite
	}
	if s.cfg.EmailCodeEnabled {
		if strings.TrimSpace(in.Code) == "" {
			return nil, "", ErrCodeRequired
		}
		if err := s.email.Verify(ctx, in.Email, strings.TrimSpace(in.Code)); err != nil {
			return nil, "", err
		}
	}

	if _, err := s.users.ByEmail(ctx, in.Email); err == nil {
		return nil, "", ErrEmailUsed
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcryptCost)
	if err != nil {
		return nil, "", fmt.Errorf("bcrypt: %w", err)
	}
	now := time.Now()
	u := &model.User{
		Email:        in.Email,
		PasswordHash: string(hash),
		Nickname:     in.Nickname,
		AvatarEmoji:  defaultAvatarEmoji,
		Role:         "member",
		LastLoginAt:  &now, // 注册即登录
	}
	if err := s.users.Create(ctx, u); err != nil {
		if errors.Is(err, repository.ErrDuplicated) {
			return nil, "", ErrEmailUsed
		}
		return nil, "", err
	}

	token, _, _, err := s.jwt.Issue(u.ID, u.Email)
	if err != nil {
		return nil, "", err
	}
	return u, token, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*model.User, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	failKey := "mt:login:fail:" + email
	if n, _ := s.rdb.Get(ctx, failKey).Int(); n >= loginFailLimit {
		return nil, "", ErrLoginLocked
	}

	u, err := s.users.ByEmail(ctx, email)
	hash := dummyHash
	found := err == nil
	if found {
		hash = []byte(u.PasswordHash)
	}
	if !found || bcrypt.CompareHashAndPassword(hash, []byte(password)) != nil {
		// 每次失败刷新 15 分钟窗口：连续 5 次即锁定，锁定期间正确密码也拒绝
		n, _ := s.rdb.Incr(ctx, failKey).Result()
		s.rdb.Expire(ctx, failKey, loginLockDuration)
		if n >= loginFailLimit {
			return nil, "", ErrLoginLocked
		}
		return nil, "", ErrBadCreds
	}

	s.rdb.Del(ctx, failKey)
	if err := s.users.TouchLastLogin(ctx, u.ID, time.Now()); err != nil {
		log.Printf("更新 last_login_at 失败 uid=%d: %v", u.ID, err)
	}

	token, _, _, err := s.jwt.Issue(u.ID, u.Email)
	if err != nil {
		return nil, "", err
	}
	return u, token, nil
}

// Logout 把 jti 写入黑名单，TTL 取 token 剩余有效期，到期自动出黑名单
func (s *AuthService) Logout(ctx context.Context, jti string, expiresAt time.Time) error {
	remaining := time.Until(expiresAt)
	if remaining < time.Second {
		remaining = time.Second
	}
	return s.rdb.Set(ctx, "mt:jwt:black:"+jti, 1, remaining).Err()
}

func (s *AuthService) UserByID(ctx context.Context, id int64) (*model.User, error) {
	return s.users.ByID(ctx, id)
}
