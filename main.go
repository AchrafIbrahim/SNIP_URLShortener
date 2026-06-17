package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

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

func main() {
	initDB()

	//pass DB to handlers
	handlers.DB = db

	r := gin.Default()

	//Load template
	r.LoadHTMLGlob("templates/*")

	//Public Routes
	r.GET("/:slug", handlers.Redirect)
	r.POST("/register", handlers.Register)
	r.POST("/login", handlers.Login)
	r.POST("/shorten", handlers.Shorten)

	//Protected Routes
	auth := r.Group("/api")
	auth.Use(middleware.AuthRequired)
	{
		//Route dashboard here
	}

	r.Run(":9090")
}