package handlers

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
)

var DB *sql.DB

func redirect(c *gin.Context){
	slug := c.Param("slug")

	var originalURL string
	var expiredAt sql.NullTime

	err := DB.QueryRow(
		"SELECT original_url, expired_at FROM links WHERE slug = $1", slug
	).Scan(&originalURL, &expiredAt)

	if err != nil {
		c.HTML(http.StatusNotFound, "404.html", gin.H{
			"slug": slug,
		})
		return
	}

	if expiredAt.Valid && expiredAt.Time.Before(time.Now()){
		c.HTML(http.StatusGone, "expired.html", gin.H{
			"Slug": slug,
		})
		return
	}
	c.Redirect(302, originalURL)
}