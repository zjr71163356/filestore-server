package api

import (
	"filestore-server/service"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const sessionUserKey = "user"

type authPayload struct {
	Username string `json:"username" form:"username" binding:"required"`
	Password string `json:"password" form:"password" binding:"required"`
}

// Signup 用户注册。
func Signup(c *gin.Context) {
	var payload authPayload
	if err := c.ShouldBind(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password required"})
		return
	}

	if err := service.RegisterUser(c.Request.Context(), payload.Username, payload.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "signup success"})
}

// Login 用户登录并写入 session。
func Login(c *gin.Context) {
	var payload authPayload
	if err := c.ShouldBind(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password required"})
		return
	}

	if err := service.AuthenticateUser(c.Request.Context(), payload.Username, payload.Password); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	session := sessions.Default(c)
	session.Set(sessionUserKey, payload.Username)
	if err := session.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "login success"})
}

// Logout 清理 session。
func Logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	if err := session.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to clear session"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "logout success"})
}

// AuthMiddleware 校验 session。
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		user := session.Get(sessionUserKey)
		if user == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Set(sessionUserKey, user)
		c.Next()
	}
}
