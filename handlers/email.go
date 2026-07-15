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
	"golang.org/x/crypto/bcrypt"
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

func ForgotPassword(c *gin.Context){
	var input struct {
		Email string `json:"email"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input tidak valid"})
		return
	}

	//Cari user berdasarkan email
	var userID int
	err := DB.QueryRow(
		"SELECT id FROM users WHERE email =  $1", input.Email,
	).Scan(&userID)

	if err != nil{
		c.JSON(http.StatusOK, gin.H{"message": "Kalau email terdaftar, link reset akan dikirim." })
		return
	}

	//Generate token
	token, err := generateToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal generate token"})
		return
	}

	//Save token ke table reset password
	expiredAt := time.Now().Add(1 * time.Hour)
	_, err = DB.Exec(
		"INSERT INTO password_resets (user_id, token, expired_at) VALUES ($1, $2, $3)",
		userID, token, expiredAt,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal simpan token"})
		return
	}

	//Kirim email reset password
	if err := sendResetPasswordEmail(input.Email, token); err != nil {
		fmt.Println("Error kirim email:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal kirim email"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Link reset password sudah dikirim ke email kamu."})
}

func sendResetPasswordEmail(email, token string) error {
	resetURL := fmt.Sprintf("http://localhost:9090/new-password?token=%s", token)

	m := gomail.NewMessage()
	m.SetHeader("From", os.Getenv("SMTP_EMAIL"))
	m.SetHeader("To", email)
	m.SetHeader("Subject", "Reset Password Snip")
	m.SetBody("text/html", fmt.Sprintf(`
		<div style="font-family: Arial, sans-serif; max-width: 480px; margin: 0 auto;">
			<h2 style="color: #1a1a1a;">Reset Password</h2>
			<p style="color: #7a7a7a;">Klik tombol di bawah untuk membuat password baru. Link ini berlaku selama 1 jam.</p>
			<a href="%s" style="display: inline-block; padding: 12px 24px; background: #1a1a1a; color: white; text-decoration: none; border-radius: 8px; margin: 16px 0;">Reset Password</a>
			<p style="color: #b0b0b0; font-size: 12px;">Kalau kamu tidak meminta reset password, abaikan email ini.</p>
		</div>
	`, resetURL))

	d := gomail.NewDialer(
		"smtp.gmail.com",
		587,
		os.Getenv("SMTP_EMAIL"),
		os.Getenv("SMTP_PASSWORD"),
	)

	return d.DialAndSend(m)
}

func ResetPassword(c *gin.Context) {
	var input struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input tidak valid"})
		return
	}

	if valid, msg := IsValidPassword(input.Password); !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	// Cari token di database
	var userID int
	var expiredAt time.Time
	var usedAt sql.NullTime

	err := DB.QueryRow(
		"SELECT user_id, expired_at, used_at FROM password_resets WHERE token = $1",
		input.Token,
	).Scan(&userID, &expiredAt, &usedAt)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token tidak valid"})
		return
	}

	// Cek apakah token sudah dipakai
	if usedAt.Valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token sudah pernah dipakai"})
		return
	}

	//DEBUG
	fmt.Println("Waktu sekarang (server):", time.Now())
	fmt.Println("Token expired_at:", expiredAt)
	fmt.Println("Sudah expired?", time.Now().After(expiredAt))

	// Cek apakah token sudah expired
	if time.Now().After(expiredAt) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token sudah kedaluwarsa"})
		return
	}

	// Hash password baru
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), 10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal hash password"})
		return
	}

	// Update password di tabel users
	_, err = DB.Exec(
		"UPDATE users SET password_hash = $1 WHERE id = $2",
		string(hash), userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal update password"})
		return
	}

	// Tandai token sudah dipakai
	_, err = DB.Exec(
		"UPDATE password_resets SET used_at = NOW() WHERE token = $1",
		input.Token,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal update token"})
		return
	}
	
	LogAudit(userID, "RESET_PASSWORD", "User mereset password", c)
	c.JSON(http.StatusOK, gin.H{"message": "Password berhasil direset!"})
}

func ValidateResetToken(c *gin.Context) {
    token := c.Query("token")

    if token == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Token tidak ada"})
        return
    }

    var expiredAt time.Time
    var usedAt sql.NullTime

    err := DB.QueryRow(
        "SELECT expired_at, used_at FROM password_resets WHERE token = $1",
        token,
    ).Scan(&expiredAt, &usedAt)

    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Token tidak valid"})
        return
    }

    if usedAt.Valid {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Token sudah pernah dipakai"})
        return
    }

    if time.Now().After(expiredAt) {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Token sudah kedaluwarsa"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "Token valid"})
}

func ResendVerificationEmail(c *gin.Context) {
	var input struct {
		Email string `json:"email"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input tidak valid"})
		return
	}

	// Cari user berdasarkan email
	var userID int
	var isVerified bool
	err := DB.QueryRow(
		"SELECT id, is_verified FROM users WHERE email = $1", input.Email,
	).Scan(&userID, &isVerified)

	if err != nil {
		// Sengaja return OK supaya tidak bocorkan info email terdaftar atau tidak
		c.JSON(http.StatusOK, gin.H{"message": "Kalau email terdaftar dan belum diverifikasi, email akan dikirim ulang."})
		return
	}

	// Kalau sudah terverifikasi
	if isVerified {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email sudah terverifikasi, silakan login."})
		return
	}

	// Hapus token lama
	DB.Exec("DELETE FROM email_verifications WHERE user_id = $1", userID)

	// Generate token baru
	token, err := generateToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal generate token"})
		return
	}

	// Simpan token baru
	if err := saveVerificationToken(userID, token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal simpan token"})
		return
	}

	// Kirim ulang email
	if err := sendVerificationEmail(input.Email, token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal kirim email"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Email verifikasi sudah dikirim ulang."})
}