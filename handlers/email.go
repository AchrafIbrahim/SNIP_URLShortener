package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"time"

	"gopkg.in/gomail.v2"
	"github.com/gin-gonic/gin"
)

func generateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func sendVerificationEmail(email, token string) error {
	verifyURL := fmt.Sprintf("http://localhost:9090/verify-email?token=%s", token)
	m := gomail.NewMessage()
	m.SetHeader("From", os.Getenv("SMTP_EMAIL"))
	m.SetHeader("To", email)
	m.SetHeader("Subject", "Verifikasi Email SNIP")
	m.SetBody("text/html", fmt.Sprintf(`
		<div style="font-family: Arial, sans-serif; max-width: 480px; margin: 0 auto;">
			<h2 style="color: #1a1a1a;">Verifikasi Email Kamu</h2>
			<p style="color: #7a7a7a;">Klik tombol di bawah untuk verifikasi email dan aktifkan akun Snip kamu.</p>
			<a href="%s" style="display: inline-block; padding: 12px 24px; background: #1a1a1a; color: white; text-decoration: none; border-radius: 8px; margin: 16px 0;">Verifikasi Email</a>
			<p style="color: #b0b0b0; font-size: 12px;">Link ini akan kedaluwarsa dalam 24 jam. Kalau kamu tidak mendaftar di Snip, abaikan email ini.</p>
		</div>
	`, verifyURL))

	d := gomail.NewDialer(
		"smtp.gmail.com",
		587,
		os.Getenv("SMTP_EMAIL"),
		os.Getenv("SMTP_PASSWORD"),
	)

	// Tambahkan ini untuk debug
    err := d.DialAndSend(m)
    if err != nil {
        fmt.Println("Error kirim email:", err)
    }

	return err
	//return d.DialAndSend(m)
}

func saveVerificationToken(userID int, token string) error {
	expiredAt := time.Now().Add(24 * time.Hour)
	_, err := DB.Exec (
		"INSERT INTO email_verifications (user_id, token, expired_at) VALUES ($1, $2, $3)",
		userID, token, expiredAt,
	)
	if err != nil {
		fmt.Println("Error saving token: ", err)
	}
	return err
}

func VerifyEmail(c *gin.Context) {
	token := c.Query("token")

	if token == "" {
		c.HTML(http.StatusBadRequest, "link_reset_expired.html", nil)
		return
	}

	// Cari token di database
	var userID int
	var expiredAt time.Time
	var verifiedAt sql.NullTime

	err := DB.QueryRow(
		"SELECT user_id, expired_at, verified_at FROM email_verifications WHERE token = $1",
		token,
	).Scan(&userID, &expiredAt, &verifiedAt)

	if err != nil {
		c.HTML(http.StatusNotFound, "link_reset_expired.html", nil)
		return
	}

	// Cek apakah sudah pernah diverifikasi
	if verifiedAt.Valid {
		c.Redirect(302, "/login")
		return
	}

	// Cek apakah token sudah expired
	if time.Now().After(expiredAt) {
		c.HTML(http.StatusBadRequest, "link_reset_expired.html", nil)
		return
	}

	// Update verified_at dan is_verified
	_, err = DB.Exec(
		"UPDATE email_verifications SET verified_at = NOW() WHERE token = $1",
		token,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal verifikasi"})
		return
	}

	_, err = DB.Exec(
		"UPDATE users SET is_verified = TRUE WHERE id = $1",
		userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal update status verifikasi"})
		return
	}

	// Redirect ke halaman email verified
	c.Redirect(302, "/email-verified")
}