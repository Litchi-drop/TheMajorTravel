package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"math/big"
	"mime"
	"net/smtp"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/Litchi-drop/TheMajorTravel/backend/internal/config"
)

// 邮箱验证码三件套（报告 §3.6）：TTL 存储 / 60s 重发 NX / 每日限额 INCR；
// 验证码不落 PG，全生命周期在 Redis 内
type EmailService struct {
	cfg *config.Config
	rdb *redis.Client
}

func NewEmailService(cfg *config.Config, rdb *redis.Client) *EmailService {
	return &EmailService{cfg: cfg, rdb: rdb}
}

func (s *EmailService) Enabled() bool { return s.cfg.EmailCodeEnabled }

// SendCode 生成并发送 6 位验证码：60 秒内不可重发、同邮箱每日 10 条、验证码 5 分钟有效
func (s *EmailService) SendCode(ctx context.Context, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))

	ok, err := s.rdb.SetNX(ctx, "mt:code:cd:"+email, 1, 60*time.Second).Result()
	if err != nil {
		return fmt.Errorf("redis: %w", err)
	}
	if !ok {
		return ErrCodeCooldown
	}

	quotaKey := "mt:quota:" + email + ":" + time.Now().Format("20060102")
	n, err := s.rdb.Incr(ctx, quotaKey).Result()
	if err != nil {
		return fmt.Errorf("redis: %w", err)
	}
	if n == 1 {
		s.rdb.Expire(ctx, quotaKey, 24*time.Hour)
	}
	if n > 10 {
		return ErrCodeQuota
	}

	code, err := sixDigitCode()
	if err != nil {
		return err
	}
	if err := s.rdb.Set(ctx, "mt:code:"+email, code, 5*time.Minute).Err(); err != nil {
		return fmt.Errorf("redis: %w", err)
	}

	if err := sendMail(s.cfg, email, code); err != nil {
		// 发信失败清掉冷却与验证码，用户可立刻重试
		s.rdb.Del(ctx, "mt:code:cd:"+email, "mt:code:"+email)
		log.Printf("验证码发信失败 to=%s err=%v", email, err)
		return ErrSMTPFailed
	}
	return nil
}

// Verify GETDEL 一次性消费后常量时间比对；调用方须先确认开关已开启
func (s *EmailService) Verify(ctx context.Context, email, code string) error {
	stored, err := s.rdb.GetDel(ctx, "mt:code:"+email).Result()
	if errors.Is(err, redis.Nil) {
		return ErrCodeWrong
	}
	if err != nil {
		return fmt.Errorf("redis: %w", err)
	}
	if subtle.ConstantTimeCompare([]byte(stored), []byte(code)) != 1 {
		return ErrCodeWrong
	}
	return nil
}

func sixDigitCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", fmt.Errorf("生成验证码: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func sendMail(cfg *config.Config, to, code string) error {
	addr := cfg.SMTPHost + ":" + cfg.SMTPPort
	from := cfg.SMTPUser
	auth := smtp.PlainAuth("", from, cfg.SMTPPass, cfg.SMTPHost)

	subject := mime.QEncoding.Encode("utf-8", "MajorTravel 注册验证码")
	msg := []byte("To: " + to + "\r\n" +
		"From: MajorTravel <" + from + ">\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"你的注册验证码是 " + code + "，5 分钟内有效。\r\n" +
		"若非本人操作，请忽略本邮件。\r\n")

	if cfg.SMTPPort == "465" {
		// 465 为隐式 TLS：先 tls.Dial 握手再走 SMTP；
		// 其余端口用 smtp.SendMail，服务器支持时自动 STARTTLS
		conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: cfg.SMTPHost})
		if err != nil {
			return err
		}
		cl, err := smtp.NewClient(conn, cfg.SMTPHost)
		if err != nil {
			return err
		}
		defer cl.Close()
		if err = cl.Auth(auth); err != nil {
			return err
		}
		if err = cl.Mail(from); err != nil {
			return err
		}
		if err = cl.Rcpt(to); err != nil {
			return err
		}
		w, err := cl.Data()
		if err != nil {
			return err
		}
		if _, err = w.Write(msg); err != nil {
			return err
		}
		if err = w.Close(); err != nil {
			return err
		}
		return cl.Quit()
	}
	return smtp.SendMail(addr, auth, from, []string{to}, msg)
}
