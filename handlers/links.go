package handlers

import(
	"math/rand"
	"net/http"
	"strings"
	"time"
	"database/sql"

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
		Slug string `json:"slug"`
		ExpiredAt string `json:"expired_at"`
		MaxClicks int `json:"max_clicks"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input tidak valid"})
		return
	}

	input.Slug = strings.TrimSpace(input.Slug)
	input.URL = strings.TrimSpace(input.URL)

	// Validasi URL
	if !IsValidURL(input.URL) {
    	c.JSON(http.StatusBadRequest, gin.H{"error": "URL tidak valid"})
    	return
	}

	// Kalau ada slug kustom
	if input.Slug != "" && !IsValidSlug(input.Slug) {
   		c.JSON(http.StatusBadRequest, gin.H{"error": "Slug hanya boleh huruf, angka, dan tanda hubung"})
    	return
	}

	// Ambil user_id dari token
	userID := c.MustGet("user_id")

	// Generate slug random character
	slug := input.Slug
	if slug == "" {
		slug = generateSlug(6)
	}

	// Save to DB
	var expiredAt *time.Time
	if input.ExpiredAt != "" {
		t, err := time.Parse("2006-01-02", input.ExpiredAt)
    	if err == nil {
        	expiredAt = &t
    	}
	}

	var maxClicks *int
	if input.MaxClicks > 0 {
    	maxClicks = &input.MaxClicks
	}
	
	_, err := DB.Exec(
		"INSERT INTO links (slug, original_url, user_id, expired_at, max_clicks) VALUES ($1, $2, $3, $4, $5)",
		slug, input.URL, userID, expiredAt, maxClicks,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan link"})
		return
	}

	LogAudit(userID, "SHORTEN_URL", "User mempersingkat URL" +input.URL, c)
	c.JSON(http.StatusOK, gin.H{
		"slug":       	slug,
		"short_url":  	"http://localhost:9090/" + slug,
		"original_url": input.URL,
	})
}

func GetLinks(c *gin.Context){
	userID := c.MustGet("user_id")
	
	rows, err := DB.Query(
		"SELECT id, slug, original_url, clicks, expired_at, created_at FROM links WHERE user_id = $1 ORDER BY created_at DESC",
		userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data link"})
		return
	}
	defer rows.Close()

	var links []gin.H
	for rows.Next(){
		var id int
		var slug, originalURL string
		var clicks int
		var expiredAt sql.NullTime
		var createdAt time.Time

		rows.Scan(&id, &slug, &originalURL, &clicks, &expiredAt, &createdAt)

		status := "aktif"
		if expiredAt.Valid && expiredAt.Time.Before(time.Now()) {
			status = "expired"
		}
		
		links = append(links, gin.H{
			"id":           id,
			"slug":         slug,
			"original_url": originalURL,
			"short_url":    "http://localhost:9090/" + slug,
			"clicks":       clicks,
			"status":		status,
			"created_at":   createdAt.Format("2 Jan 2006"),
		})
	}

	if links == nil {
		links = []gin.H{}
	}

	c.JSON(http.StatusOK, gin.H{"links": links})
}

func GetStats(c *gin.Context) {
	userID := c.MustGet("user_id")

	//Total Link
	var totalLinks int
	DB.QueryRow("SELECT COUNT(*) FROM links WHERE user_id = $1", userID).Scan(&totalLinks)

	//Total Klik
	var totalClicks int
	DB.QueryRow("SELECT COALESCE(SUM(clicks), 0) FROM links WHERE user_id = $1", userID).Scan(&totalClicks)

	//Link Aktif 
	var activeLinks int
	DB.QueryRow(
		`SELECT COUNT(*) FROM links
		WHERE user_id = $1
		AND (expired_at IS NULL OR expired_at > NOW())
		`, userID,
	).Scan(&activeLinks)

	//Klik hari ini 
	var clicksToday int
	DB.QueryRow(`
		SELECT COUNT(*) FROM click_logs
		WHERE link_id IN (SELECT id FROM links WHERE user_id = $1)
		AND clicked_at >= CURRENT_DATE
	`, userID).Scan(&clicksToday)

	c.JSON(http.StatusOK, gin.H{
		"total_links":   totalLinks,
		"total_clicks":  totalClicks,
		"active_links":  activeLinks,
		"clicks_today":  clicksToday,
	})
}