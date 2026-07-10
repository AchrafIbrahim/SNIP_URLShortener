package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"gopkg.in/gomail.v2"
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

	return d.DialAndSend(m)
}

func saveVerificationToken(userID int, token string) error {
	expiredAt := time.Now().Add(24 * time.Hour)
	_, err := DB.exec (
		"ÏNSERT INTO email_verification (user_id, token, expired_at) VALUE ($1, $2, $3)",
		userID, token, exiredAt,
	)
	return err
}