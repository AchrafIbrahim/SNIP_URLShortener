package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()

	r.GET("/:slug", func(c *gin.Context) {
		slug := c.Param("slug")
		c.JSON(http.StatusOK, gin.H{
			"slug": slug,
		})
	})

	r.Run(":9090")
}