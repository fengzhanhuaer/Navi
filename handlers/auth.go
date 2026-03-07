// handlers/auth.go
// 单用户注册 / 登录 / 状态检查

package handlers

import (
	"net/http"
	"os"
	"time"

	"navi/db"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// jwtSecret 从环境变量读取，启动时由 main 保证已设置
func jwtSecret() []byte {
	s := os.Getenv("JWT_SECRET")
	if s == "" {
		s = "navi-default-secret-change-me"
	}
	return []byte(s)
}

// ─────────────────────────────────────────────
// GET /api/auth/status
// 返回 { registered: bool } 供前端决定显示注册还是登录
// ─────────────────────────────────────────────

func AuthStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"registered": db.UserExists()})
}

// ─────────────────────────────────────────────
// POST /api/auth/register
// body: { username, password }
// 仅首次可用，已有用户时返回 403
// ─────────────────────────────────────────────

func Register(c *gin.Context) {
	var body struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名和密码不能为空"})
		return
	}
	if len(body.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "密码至少 6 位"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
		return
	}

	if err := db.CreateUser(body.Username, string(hash)); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "已完成注册，不允许再次注册"})
		return
	}

	token, err := makeToken(body.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成 token 失败"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"token": token})
}

// ─────────────────────────────────────────────
// POST /api/auth/login
// body: { username, password }
// ─────────────────────────────────────────────

func Login(c *gin.Context) {
	var body struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名和密码不能为空"})
		return
	}

	user, err := db.GetUserByUsername(body.Username)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(body.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	token, err := makeToken(user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成 token 失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token})
}

// ─────────────────────────────────────────────
// JWT 工具
// ─────────────────────────────────────────────

func makeToken(username string) (string, error) {
	claims := jwt.MapClaims{
		"sub": username,
		"exp": time.Now().Add(30 * 24 * time.Hour).Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(jwtSecret())
}

// AuthMiddleware 验证 Bearer token，失败返回 401
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if len(header) < 8 || header[:7] != "Bearer " {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
			return
		}
		tokenStr := header[7:]
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return jwtSecret(), nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "登录已过期，请重新登录"})
			return
		}
		c.Next()
	}
}
