package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	_"github.com/lib/pq"
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
		log.Fatal("Cannot reach database:", err)
	}

	fmt.Println("Database connected successfully!")
}

func main() {
	initDB()

	r := gin.Default()

	r.GET("/:slug", func(c *gin.Context) {
		slug := c.Param("slug")

		var originalURL string
		err := db.QueryRow("SELECT original_url FROM links WHERE slug = $1", slug).Scan(&originalURL)
		if err != nil {
			c.JSON(404, gin.H{"error": "Link tidak ditemukan"})
			return
		}

		c.Redirect(302, originalURL)
	})

	r.Run(":9090")
}