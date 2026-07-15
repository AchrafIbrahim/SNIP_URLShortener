package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	_"github.com/lib/pq"

	"github.com/AchrafIbrahim/SNIP_URLShortener/handlers"
	"github.com/AchrafIbrahim/SNIP_URLShortener/middleware"
)

var db *sql.DB

func initDB() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	dsn:= fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
	)
	db, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal("Error connecting to database:", err)
	}
	err = db.Ping()
	if err != nil {
		log.Fatal("Cannot reach database:", err)
	}

	fmt.Println("Database connected successfully!")
}

func noCache() gin.HandlerFunc {
	return func(c *gin.Context){
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
        c.Header("Pragma", "no-cache")
        c.Header("Expires", "0")
        c.Next()
	}
}

func main() {
	initDB()

	//pass DB to handlers
	handlers.DB = db

	r := gin.Default()

	//Load template
	r.LoadHTMLGlob("templates/*")

	//HTML Routes
	r.GET("/login", func(c *gin.Context){
		c.HTML(http.StatusOK, "login.html", nil)
	})
	r.GET("/register", func(c *gin.Context){
		c.HTML(http.StatusOK, "register.html", nil)
	})
	r.GET("/forgot-password", func(c *gin.Context){
		c.HTML(http.StatusOK, "forget_password.html", nil)
	})
	r.GET("/email-check-reset-password", func(c *gin.Context){
		c.HTML(http.StatusOK, "email_check_reset_password.html", nil)
	})
	r.GET("/expired", func(c *gin.Context){
		c.HTML(http.StatusOK, "expired.html", nil)
	})
	r.GET("/link-reset-expired", func(c *gin.Context){
		c.HTML(http.StatusOK, "link_reset_expired.html", nil)
	})
	r.GET("/new-password", func(c *gin.Context){
		c.HTML(http.StatusOK, "new_password.html", nil)
	})
	r.GET("/success-reset-password", func(c *gin.Context){
		c.HTML(http.StatusOK, "success_reset_password.html", nil)
	})
	r.GET("/verify-email", handlers.VerifyEmail)
	r.GET("/email-verified", func(c *gin.Context){
		c.HTML(http.StatusOK, "email_verified.html", nil)
	})
	r.GET("/validate-reset-token", handlers.ValidateResetToken)
	r.POST("/forgot-password", handlers.ForgotPassword)
	r.POST("/reset-password", handlers.ResetPassword)
	r.POST("/resend-verification", handlers.ResendVerificationEmail)
	r.POST("/refresh-token", handlers.RefreshToken)



	//Public Routes
	
	r.POST("/register", handlers.Register)
	r.POST(
		"/login", middleware.RateLimitMiddleware(), 
		handlers.Login,
	)

	//Protected Routes
	auth := r.Group("/api")
	auth.Use(middleware.AuthRequired)
	{
		//Route dashboard here
		auth.GET("/me", func(c *gin.Context) {
       		userID := c.MustGet("user_id")

			var name, email string
			err := db.QueryRow(
				"SELECT name, email FROM users WHERE id = $1", userID, 
			).Scan(&name, &email)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal ambil data user"})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"user_id": userID,
				"name": name,
				"email": email,
			})
   		})
		auth.POST("/shorten", handlers.Shorten)
		auth.GET("/links", handlers.GetLinks)
		auth.PUT("/profile", handlers.UpdateProfile)
		auth.GET("/audit-logs", handlers.GetAuditLogs)
		auth.POST("/logout", handlers.Logout)
	}

	protected := r.Group("/")
	protected.Use(noCache())
	{
		protected.GET("/main", func(c *gin.Context) {
			c.HTML(http.StatusOK, "main.html", nil)
		})
		protected.GET("/profile", func(c *gin.Context) {
			c.HTML(http.StatusOK, "profile.html", nil)
		})
	}


	//Redirect
	r.GET("/:slug", handlers.Redirect)

	r.Run(":9090")
}