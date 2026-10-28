package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"notes-api-gin/models"
	"notes-api-gin/service"
)

const (
	UserContextKey   = "auth_user"
	ClaimsContextKey = "auth_claims"
	TokenContextKey  = "auth_token"
)

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func Auth(authService service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format, expected 'Bearer <token>'"})
			return
		}

		tokenStr := parts[1]
		user, claims, err := authService.ValidateToken(c.Request.Context(), tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		c.Set(UserContextKey, user)
		c.Set(ClaimsContextKey, claims)
		c.Set(TokenContextKey, tokenStr)

		c.Next()
	}
}

func GetAuthUser(c *gin.Context) *models.UserResponse {
	val, exists := c.Get(UserContextKey)
	if !exists {
		return nil
	}
	user, ok := val.(*models.UserResponse)
	if !ok {
		return nil
	}
	return user
}

func GetAuthToken(c *gin.Context) string {
	val, exists := c.Get(TokenContextKey)
	if !exists {
		return ""
	}
	tok, ok := val.(string)
	if !ok {
		return ""
	}
	return tok
}

