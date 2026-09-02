package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims 报告 §4.3：HS256、有效期 7 天、claims 携带 uid/email/jti
type Claims struct {
	UID   int64  `json:"uid"`
	Email string `json:"email"`
	JTI   string `json:"jti"`
	jwt.RegisteredClaims
}

type JWTService struct {
	secret []byte
	ttl    time.Duration
}

func NewJWTService(secret string) *JWTService {
	return &JWTService{secret: []byte(secret), ttl: 7 * 24 * time.Hour}
}

func (s *JWTService) TTL() time.Duration { return s.ttl }

func (s *JWTService) Issue(uid int64, email string) (token, jti string, expiresAt time.Time, err error) {
	raw := make([]byte, 16)
	if _, err = rand.Read(raw); err != nil {
		return "", "", time.Time{}, fmt.Errorf("生成 jti: %w", err)
	}
	jti = hex.EncodeToString(raw)
	now := time.Now()
	expiresAt = now.Add(s.ttl)
	claims := &Claims{
		UID:   uid,
		Email: email,
		JTI:   jti,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	token, err = jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
	return token, jti, expiresAt, err
}

func (s *JWTService) Parse(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	t, err := jwt.ParseWithClaims(tokenStr, claims, func(*jwt.Token) (any, error) {
		return s.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !t.Valid {
		return nil, errors.New("token 无效或已过期")
	}
	return claims, nil
}
