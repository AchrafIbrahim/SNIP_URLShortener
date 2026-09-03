package handlers

import (
	"net/http"
	"os"
	"time"
	"fmt"
	"strings"
	"context"
	"encoding/json"
	"database/sql"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

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

	// Generate JWT token (Update only to 15 minutes)
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": 	id,
		"type": 	"access",
		"exp":     	time.Now().Add(15 * time.Minute).Unix(), //token duration
	})

	accessTokenString, err := accessToken.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal generate access token"})
		return
	}

	// Generate Refresh Token (Up to 7 days)
	refreshTokenValue, err := generateToken() // function on email.go
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal generate refresh token"})
		return
	}

	// Save Refresh token to DB 
	expiredAt := time.Now().Add(7 * 24 * time.Hour)
	_, err = DB.Exec (
		"INSERT INTO refresh_tokens (user_id, token, expired_at) VALUES ($1, $2, $3)",
		id, refreshTokenValue, expiredAt,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal simpan refresh token"})
		return
	}

	LogAudit(id, "LOGIN", "User berhasil login", c)
	c.JSON(http.StatusOK, gin.H{
		"access_token": accessTokenString,
		"refresh_token": refreshTokenValue,
		"expires_in": 900, // 15 minutes in seconds
	})
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

func RefreshToken(c *gin.Context) {
	var input struct {
		RefreshToken string `json:"refresh_token"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input tidak valid"})
		return 
	}

	// find refresh token
	var userID int
	var expiredAt time.Time

	err := DB.QueryRow(
		"SELECT user_id, expired_at FROM refresh_tokens WHERE token = $1",
		input.RefreshToken,
	).Scan(&userID, &expiredAt)

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Refresh token tidak valid"})
		return
	}

	// expired check
	if time.Now().After(expiredAt) {
		DB.Exec("DELETE FROM refresh_tokens WHERE token = $1", input.RefreshToken)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Refresh token sudah kedaluwarsa, silakan login ulang"})
		return 
	}

	// delete old refresh token (rotation here)
	DB.Exec("DELETE FROM refresh_tokens WHERE token = $1", input.RefreshToken)

	//Generate new access token
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": 	userID,
		"type":		"access",
		"exp":		time.Now().Add(15 * time.Minute).Unix(),
	})

	accessTokenString, err := accessToken.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal generate access token baru"})
		return 
	}

	// Generate new refresh token
	newRefreshToken, err := generateToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal generate refresh token"})
		return
	}

	// Save new refresh token to DB
	newExpiredAt := time.Now().Add(7 * 24 * time.Hour)
	DB.Exec(
		"INSERT INTO refresh_tokens (user_id, token, expired_at) VALUES ($1, $2, $3)",
		userID, newRefreshToken, newExpiredAt,
	)

	c.JSON(http.StatusOK, gin.H{
		"access_token": accessTokenString,
		"refresh_token": newRefreshToken,
		"expires_in": 900,
	})
}

func Logout(c *gin.Context) {
	var input struct {
		RefreshToken string `json:"refresh_token"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input tidak valid"})
		return
	}

	// Hapus refresh token dari DB
	DB.Exec("DELETE FROM refresh_tokens WHERE token = $1", input.RefreshToken)

	userID := c.MustGet("user_id")
	LogAudit(userID, "LOGOUT", "User logout", c)

	c.JSON(http.StatusOK, gin.H{"message": "Berhasil logout"})
}

func ChangePassword(c *gin.Context) {
	userID := c.MustGet("user_id")

	var input struct {
		CurrentPassword string `json:"current_password"`
		NewPassword 	string `json:"new_password"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input tidak valid"})
		return
	}

	//Ambil current password dari DB
	var PasswordHash string
	err := DB.QueryRow(
		"SELECT password_hash FROM users WHERE id = $1", userID,
	).Scan(&PasswordHash)

	fmt.Println("userID:", userID)
	fmt.Println("currentPassword:", input.CurrentPassword)
	fmt.Println("passwordHash dari DB:", PasswordHash)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Terjadi kesalahan"})
		return
	}

	//Cek Password Lama
	err = bcrypt.CompareHashAndPassword([]byte(PasswordHash), []byte(input.CurrentPassword))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Password saat ini salah"})
		return
	}

	//Validasi password baru
	if valid, msg := IsValidPassword(input.NewPassword); !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	//Hash password baru
	newHash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), 10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal hash password"})
		return
	}

	//Updae password di DB
	_, err = DB.Exec(
		"UPDATE users SET password_hash = $1 WHERE id = $2",
		string(newHash), userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal update password"})
		return
	}

	//Hapus semua refresh token (paksa logout)
	DB.Exec("DELETE FROM refresh_tokens WHERE user_id = $1", userID)

	LogAudit(userID, "CHANGE_PASSWORD", "User mengganti password", c)

	c.JSON(http.StatusOK, gin.H{"message": "Password berhasil diubah. Silahkan login ulang"})
}

func DeleteAccount(c *gin.Context) {
	userID := c.MustGet("user_id")

	var input struct {
		Password string `json:"password"`
	}

	if err:= c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input tidak valid"})
		return
	}

	//verifikasi password sebelum hapus akun
	var passwordHash string
	err := DB.QueryRow(
		"SELECT password_hash FROM users WHERE id = $1", userID,
	).Scan(&passwordHash)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Terjadi kesalahan"})
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(input.Password))
	if err != nil  {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Password salah"})
		return
	}

	//Hapus semua data dari DB berurutan
	tx, err := DB.Begin()
	if err != nil {
		c.JSON(http. StatusInternalServerError, gin.H{"error": "Gagal memulai transaksi",})
		return
	}
	defer tx.Rollback()

	_, err = tx.Exec("DELETE FROM click_logs WHERE link_id IN (SELECT id FROM links WHERE user_id = $1)", userID)
	if err != nil {
		fmt.Println("Delete Click logs:", err)
		return
	}

	_, err = tx.Exec("DELETE FROM links WHERE user_id = $1", userID)
	if err != nil {
		fmt.Println("Delete links:", err)
		return
	}

	_, err = tx.Exec("DELETE FROM refresh_tokens WHERE user_id = $1", userID)
	if err != nil {
		fmt.Println("Delete refresh token:", err)
		return
	}

	_, err = tx.Exec("DELETE FROM email_verifications WHERE user_id = $1", userID)
	if err != nil {
		fmt.Println("Delete email verified:", err)
		return
	}

	_, err = tx.Exec("DELETE FROM password_resets WHERE user_id = $1", userID)
	if err != nil {
		fmt.Println("Delete password:", err)
		return
	}

	_, err = tx.Exec("DELETE FROM audit_logs WHERE user_id = $1", userID)
	if err != nil {
		fmt.Println("Delete audit log:", err)
		return
	}

	_, err = tx.Exec("DELETE FROM users WHERE id = $1", userID)
	if err != nil {
		fmt.Println("Delete user:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus akun"})
		return
	}

	err = tx.Commit()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal commit transaksi"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Akun berhasil dihapus"})
}

func GetGoogleAuthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID: os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL: os.Getenv("GOOGLE_REDIRECT_URL"),
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}
}

func GoogleLoginHandler(c *gin.Context) {
	config := GetGoogleAuthConfig()
	state, err := generateOAuthState()

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat OAuth state",})
		return
	}

	c.SetCookie(
		"oauth_state",
		state,
		300,
		"/",
		"",
		false,
		true,
	)

	c.SetCookie(
		"oauth_mode",
		"login",
		300,
		"/",
		"",
		false,
		true,
	)

	url := config.AuthCodeURL(state, oauth2.AccessTypeOffline,)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

func GoogleRegisterHandler(c *gin.Context) {
	config := GetGoogleAuthConfig()
	state, err := generateOAuthState()

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat OAuth state",})
		return
	}

	c.SetCookie(
		"oauth_state",
		state,
		300,
		"/",
		"",
		false,
		true,
	)

	c.SetCookie(
		"oauth_mode",
		"register",
		300,
		"/",
		"",
		false,
		true,
	)

	url := config.AuthCodeURL(state, oauth2.AccessTypeOffline)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

func GoogleCallBackHandler(c *gin.Context) {
	config := GetGoogleAuthConfig()
	code := c.Query("code")
	state := c.Query("state")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Authorization code tidak ditemukan"})
		return
	}

	if state == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "OAuth state tidak ditemukan",})
		return
	}

	savedState, err := c.Cookie("oauth_state")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "OAuth state tidak ditemukan atau sudah kedaluwarsa",})
		return
	}

	if subtle.ConstantTimeCompare([]byte(state),[]byte(savedState),) != 1 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "OAuth state tidak valid",})
		return
	}

	mode, err := c.Cookie("oauth_mode")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "OAuth tidak ditemukan atau sudah kedaluwarsa",})
		return
	}

	if mode != "login" && mode != "register" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "OAuth mode tidak valid",})
		return
	}

	c.SetCookie(
		"oauth_state",
		"",
		-1,
		"/",
		"",
		false,
		true,
	)

	c.SetCookie(
		"oauth_mode",
		"",
		-1,
		"/",
		"",
		false,
		true,
	)

	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menukar token dari google"})
		return
	}

	client := config.Client(context.Background(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data user dari google"})
		return
	}
	defer resp.Body.Close()

	var googleUser struct {
		ID string `json:"id"`
		Email string `json:"email"`
		Name string `json:"name"`
		VerifiedEmail bool `json:"verified_email"`
		Picture string `json:"picture"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&googleUser); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membaca response profil Google",})
		return
	}

	if !googleUser.VerifiedEmail {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Email Google belum terverifikasi",
		})
		return
	}

	var userID int
	var isNewUser bool

	err = DB.QueryRow(
		"SELECT id FROM users WHERE email = $1",
		googleUser.Email,
	).Scan(&userID)

	if err == sql.ErrNoRows {
		if mode == "login" {
			c.Redirect(http.StatusSeeOther, "/oauth-error?type=not_registered&from=login",)
			return
		}

		isNewUser = true
		randomPassword, err := generateToken()

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat akun google",})
			return
		}

		passwordHash, err := bcrypt.GenerateFromPassword([]byte(randomPassword), bcrypt.DefaultCost,)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat password akun google",})
			return
		}

		err = DB.QueryRow(
			`INSERT INTO users(email, password_hash, name, is_verified) VALUES ($1, $2, $3, $4) RETURNING id`,
			googleUser.Email, string(passwordHash), googleUser.Name, true,
		).Scan(&userID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat akun google",})
			return
		}
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memeriksa akun google"})
		return
	} else {
		if mode == "register" {
			c.JSON(http.StatusConflict, gin.H{"error": "Akun google sudah terdaftar. Silahkan langsung login"})
			return
		}
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"type":    "access",
		"exp":     time.Now().Add(15 * time.Minute).Unix(),
	})

	accessTokenString, err := accessToken.SignedString(
		[]byte(os.Getenv("JWT_SECRET")),
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal generate access token",
		})
		return
	}

	// Generate refresh token
	refreshTokenValue, err := generateToken()

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal generate refresh token",
		})
		return
	}

	expiredAt := time.Now().Add(7 * 24 * time.Hour)

	_, err = DB.Exec(
		"INSERT INTO refresh_tokens (user_id, token, expired_at) VALUES ($1, $2, $3)",
		userID,
		refreshTokenValue,
		expiredAt,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal simpan refresh token",
		})
		return
	}

	if isNewUser {
		LogAudit(
			userID,
			"REGISTER",
			"User berhasil mendaftar dengan Google",
			c,
		)
	} else {
		LogAudit(
			userID,
			"LOGIN",
			"User berhasil login dengan Google",
			c,
		)
	}
	

	accessTokenJSON, _ := json.Marshal(accessTokenString)
	refreshTokenJSON, _ := json.Marshal(refreshTokenValue)

	c.Header("Content-Type", "text/html; charset=utf-8")

	c.String(http.StatusOK,`
	<!DOCTYPE html>
	<html>
	<head>
		<title>Login Google</title>
	</head>
	<body>
		<p>Login berhasil. Mengarahkan ke dashboard...</p>

		<script>
			sessionStorage.setItem("access_token", %s);
			sessionStorage.setItem("refresh_token", %s);
			sessionStorage.setItem("token_expires", Date.now() + 900 * 1000);

			window.location.href = "/main";
		</script>
	</body>
	</html>
	`, string(accessTokenJSON), string(refreshTokenJSON))
	//c.Redirect(http.StatusSeeOther, "/main")
}

func generateOAuthState() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

