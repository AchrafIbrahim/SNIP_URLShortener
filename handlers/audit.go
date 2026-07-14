package handlers

import (
	"fmt"
	"net/http"
	"github.com/gin-gonic/gin"
)

func LogAudit(userID interface{}, action, description string, c *gin.Context) {
	fmt.Println("LogAudit dipanggil:", action, userID)
	ipAddress := c.ClientIP()
	userAgent := c.Request.UserAgent()

	_, err := DB.Exec(
		"INSERT INTO audit_logs (user_id, action, description, ip_address, user_agent) VALUES ($1, $2, $3, $4, $5)",
		userID, action, description, ipAddress, userAgent,
	)
	if err != nil {
        fmt.Println("Error LogAudit:", err)
    } else {
        fmt.Println("LogAudit berhasil!")
    }
}

func GetAuditLogs(c *gin.Context) {
	userID := c.MustGet("user_id")

	rows, err := DB.Query(
		"SELECT action, description, ip_address, created_at FROM audit_logs WHERE user_id = $1 ORDER BY created_at DESC LIMIT 20",
		userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil audit log"})
		return
	}
	defer rows.Close()

	var logs []gin.H
	for rows.Next() {
		var action, description, ipAddress string
		var createdAt string

		rows.Scan(&action, &description, &ipAddress, &createdAt)
		logs = append(logs, gin.H{
			"action":      action,
			"description": description,
			"ip_address":  ipAddress,
			"created_at":  createdAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{"logs": logs})
}