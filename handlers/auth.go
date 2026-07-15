package handlers

import (
	"net/http"
	"os"
	"time"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func Register(c *gin.Context) {
	var input struct {
		Name 	 string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input tidak valid"})
		return
	}

	//Validate password
	if valid, msg := IsValidPassword(input.Password); !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), 10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal hash password"})
		return
	}

	// Simpan ke database
	var userID int
	input.Name = SanitizeText(input.Name)
	input.Email = strings.TrimSpace(input.Email)
	err = DB.QueryRow(
		"INSERT INTO users (name, email, password_hash) VALUES ($1, $2, $3) RETURNING id",
		input.Name, input.Email, string(hash),
	).Scan(&userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email sudah terdaftar"})
		return
	}

	//Generate token verifikasi
	token, err := generateToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal generate token"})
		return
	}

	//Simpan token ke DB
	if err := saveVerificationToken(userID, token); err != nil{
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal simpan token"})
		return
	}

	//Kirim email verifikasi
	if err := sendVerificationEmail(input.Email, token); err != nil {
		DB.Exec("DELETE FROM email_verifications WHERE user_id = $1", userID)
    	DB.Exec("DELETE FROM users WHERE id = $1", userID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal kirim email verifikasi"})
		return
	}
	LogAudit(userID, "REGISTER", "User baru mendaftar: " +input.Email, c)	
	c.JSON(http.StatusOK, gin.H{"message": "Registrasi berhasil"})
}

func Login(c *gin.Context) {
	var input struct {
		Name 	 string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input tidak valid"})
		return
	}

	// Cari user di database
	var id int
	var passwordHash string
	err := DB.QueryRow(
		"SELECT id, password_hash FROM users WHERE email = $1", input.Email,
	).Scan(&id, &passwordHash)

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Email atau password salah"})
		return
	}

	// Cek password
	err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(input.Password))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Email atau password salah"})
		return
	}

	// Cek apakah email sudah terverifikasi
	var isVerified bool
	err = DB.QueryRow(
		"SELECT is_verified FROM users WHERE id = $1", id,
	).Scan(&isVerified)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Terjadi kesalahan"})
		return
	}

	if !isVerified {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Email belum diverifikasi. Cek inbox/spam kamu"})
		return
	}

	// Generate JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": id,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal generate token"})
		return
	}

	LogAudit(id, "LOGIN", "User berhasil login", c)
	c.JSON(http.StatusOK, gin.H{"token": tokenString})

	
}

func UpdateProfile(c *gin.Context) {
	userID := c.MustGet("user_id").(int)

	var input struct {
		Name 	 string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input tidak valid"})
		return
	}

	//Sanitize name
	input.Name = SanitizeText(input.Name)

	var userEmail string
	err := DB.QueryRow("SELECT email FROM users WHERE id = $1", userID).Scan(&userEmail)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data user"})
		return
	}

	if input.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Nama tidak boleh kosong"})
		return
	}

	_, err = DB.Exec(
		"UPDATE users SET name = $1 WHERE id = $2",
		input.Name, userID,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal update profil"})
		return
	}
	LogAudit(userID, "UPDATE_PROFILE", fmt.Sprintf("User %s mengubah nama profil", userEmail), c)
	c.JSON(http.StatusOK, gin.H{"message": "Profil berhasil diperbarui"})
}