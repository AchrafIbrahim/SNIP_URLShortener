package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func AuthRequired(c *gin.Context) {
	// Ambil token dari header Authorization
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Token tidak ditemukan"})
		c.Abort()
		return
	}

	// Format header: "Bearer tokennya"
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Format token tidak valid"})
		c.Abort()
		return
	}

	tokenString := parts[1]

	// Validasi token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("JWT_SECRET")), nil
	})

	if err != nil || !token.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Token tidak valid"})
		c.Abort()
		return
	}

	// Ambil user_id dari token dan simpan ke context
	claims := token.Claims.(jwt.MapClaims)
	c.Set("user_id", int(claims["user_id"].(float64)))

	c.Next()
}