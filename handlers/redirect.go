package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

var DB *sql.DB

func Redirect(c *gin.Context){
	slug := c.Param("slug")

	var id int
	var originalURL string
	var expiredAt sql.NullTime

	err := DB.QueryRow(
		"SELECT id, original_url, expired_at FROM links WHERE slug = $1", slug,
	).Scan(&id, &originalURL, &expiredAt)

	if err != nil {
		c.HTML(http.StatusNotFound, "404.html", gin.H{
			"Slug": slug,
		})
		return
	}

	if expiredAt.Valid && expiredAt.Time.Before(time.Now()){
		c.HTML(http.StatusGone, "expired.html", gin.H{
			"Slug": slug,
		})
		return
	}

	// Tambah klik +1 di tabel links
	DB.Exec("UPDATE links SET clicks = clicks + 1 WHERE id = $1", id)

	// Catat ke click_logs
	DB.Exec(
		"INSERT INTO click_logs (link_id, ip_address, user_agent) VALUES ($1, $2, $3)",
		id, c.ClientIP(), c.Request.UserAgent(),
	)

	c.Redirect(302, originalURL)
}