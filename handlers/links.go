package handlers

import(
	"math/rand"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func generateSlug(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func Shorten(c *gin.Context) {
	var input struct {
		URL string `json:"url"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input tidak valid"})
		return
	}

	if !strings.HasPrefix(input.URL, "http://") && !strings.HasPrefix(input.URL, "https://"){
		c.JSON(http.StatusBadRequest, gin.H{"error":"URL harus diawali http:// atau https://"})
		return
	}

	// Ambil user_id dari token
	userID := c.MustGet("user_id")

	// Generate slug random character
	slug := generateSlug(6)

	// Save to DB
	_, err := DB.Exec(
		"INSERT INTO links (slug, original_url, user_id) VALUES ($1, $2, $3)",
		slug, input.URL, userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan link"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"slug":       	slug,
		"short_url":  	"http://localhost:9090/" + slug,
		"original_url": input.URL,
	})
}