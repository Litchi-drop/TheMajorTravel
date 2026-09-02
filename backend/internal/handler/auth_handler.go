package handler

import (
	"errors"
	"net/http"
	"net/mail"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Litchi-drop/TheMajorTravel/backend/internal/model"
	"github.com/Litchi-drop/TheMajorTravel/backend/internal/service"
)

type AuthHandler struct {
	auth  *service.AuthService
	email *service.EmailService
}

func NewAuthHandler(auth *service.AuthService, email *service.EmailService) *AuthHandler {
	return &AuthHandler{auth: auth, email: email}
}

// POST /api/auth/email/code
func (h *AuthHandler) SendEmailCode(c *gin.Context) {
	var req struct {
		Email string `json:"email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Err(c, http.StatusBadRequest, "请求参数格式不对")
		return
	}
	if !h.email.Enabled() {
		OK(c, gin.H{"sent": false, "message": "开发模式：邮箱验证码未开启，注册无需验证码"})
		return
	}
	if _, err := mail.ParseAddress(req.Email); err != nil {
		Err(c, http.StatusBadRequest, service.ErrEmailFormat.Error())
		return
	}
	if err := h.email.SendCode(c.Request.Context(), req.Email); err != nil {
		respondAuthErr(c, err)
		return
	}
	OK(c, gin.H{"sent": true, "message": "验证码已发送，请在 5 分钟内使用"})
}

// POST /api/auth/register — 注册成功即登录，直接返回 token
func (h *AuthHandler) Register(c *gin.Context) {
	var req struct {
		Email      string `json:"email"`
		Password   string `json:"password"`
		Nickname   string `json:"nickname"`
		InviteCode string `json:"inviteCode"`
		Code       string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Err(c, http.StatusBadRequest, "请求参数格式不对")
		return
	}
	u, token, err := h.auth.Register(c.Request.Context(), service.RegisterInput{
		Email:      req.Email,
		Password:   req.Password,
		Nickname:   req.Nickname,
		InviteCode: req.InviteCode,
		Code:       req.Code,
	})
	if err != nil {
		respondAuthErr(c, err)
		return
	}
	OK(c, gin.H{"token": token, "user": publicUser(u)})
}

// POST /api/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Err(c, http.StatusBadRequest, "请求参数格式不对")
		return
	}
	u, token, err := h.auth.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		respondAuthErr(c, err)
		return
	}
	OK(c, gin.H{"token": token, "user": publicUser(u)})
}

// POST /api/auth/logout（需登录）— jti 进黑名单，原 token 立即失效
func (h *AuthHandler) Logout(c *gin.Context) {
	exp, _ := c.Get("jtiExpiresAt")
	if err := h.auth.Logout(c.Request.Context(), c.GetString("jti"), exp.(time.Time)); err != nil {
		Err(c, http.StatusInternalServerError, "退出登录失败，请重试")
		return
	}
	OK(c, gin.H{"message": "已退出登录"})
}

// GET /api/me（需登录）
func (h *AuthHandler) Me(c *gin.Context) {
	u, err := h.auth.UserByID(c.Request.Context(), c.GetInt64("uid"))
	if err != nil {
		Err(c, http.StatusUnauthorized, "请先登录")
		return
	}
	OK(c, publicUser(u))
}

func publicUser(u *model.User) gin.H {
	return gin.H{
		"id":          u.ID,
		"email":       u.Email,
		"nickname":    u.Nickname,
		"avatarEmoji": u.AvatarEmoji,
		"role":        u.Role,
	}
}

// respondAuthErr 把业务哨兵错误映射为 HTTP 状态码；未知错误记日志、对外只给通用文案
func respondAuthErr(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, service.ErrEmailFormat),
		errors.Is(err, service.ErrPasswordRule),
		errors.Is(err, service.ErrNickname),
		errors.Is(err, service.ErrCodeRequired),
		errors.Is(err, service.ErrCodeWrong):
		status = http.StatusBadRequest
	case errors.Is(err, service.ErrInvite):
		status = http.StatusForbidden
	case errors.Is(err, service.ErrEmailUsed):
		status = http.StatusConflict
	case errors.Is(err, service.ErrBadCreds):
		status = http.StatusUnauthorized
	case errors.Is(err, service.ErrLoginLocked),
		errors.Is(err, service.ErrCodeCooldown),
		errors.Is(err, service.ErrCodeQuota):
		status = http.StatusTooManyRequests
	case errors.Is(err, service.ErrSMTPFailed):
		status = http.StatusBadGateway
	}
	if status == http.StatusInternalServerError {
		c.Error(err)
		Err(c, status, "服务开小差了，请稍后再试")
		return
	}
	Err(c, status, err.Error())
}
