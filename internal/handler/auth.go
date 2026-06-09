package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/grok-fireworks-reg/internal/config"
	"github.com/grok-fireworks-reg/internal/middleware"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	cfg *config.Config
}

// NewAuthHandler 创建 AuthHandler
func NewAuthHandler(cfg *config.Config) *AuthHandler {
	return &AuthHandler{cfg: cfg}
}

// LoginRequest 登录请求体
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login POST /api/auth/login — 用户登录
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入用户名和密码"})
		return
	}

	if req.Username != h.cfg.Auth.Username || req.Password != h.cfg.Auth.Password {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	token, err := middleware.GenerateToken(req.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成 Token 失败"})
		return
	}

	// 设置 cookie 方便浏览器直接访问
	c.SetCookie("token", token, 72*3600, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"ok": true, "token": token})
}

// Me GET /api/auth/me — 获取当前用户信息
func (h *AuthHandler) Me(c *gin.Context) {
	username, _ := c.Get("username")
	c.JSON(http.StatusOK, gin.H{"username": username})
}
