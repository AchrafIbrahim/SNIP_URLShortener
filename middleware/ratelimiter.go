package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type visitor struct {
	limiter *rate.Limiter
	lastSeen time.Time
}

var (
	visitors = make(map[string]*visitor)
	mu       sync.Mutex
)

func getVisitor(ip string) *rate.Limiter {
	mu.Lock()
	defer mu.Unlock()

	v, exists := visitors[ip]
	if !exists {
		limiter := rate.NewLimiter(rate.Every(1*time.Minute), 5)
		visitors[ip] = &visitor{
			limiter: limiter,
			lastSeen: time.Now(),
		}
		return limiter
	}

	v.lastSeen = time.Now()
	return v.limiter
}

func cleanupVisitors() {
	for {
		time.Sleep(time.Minute)
		
		mu.Lock()
		for ip, v := range visitors {
			if time.Since(v.lastSeen) > 15*time.Minute {
				delete(visitors, ip)
			}
		}
		mu.Unlock()
	}
}

func RateLimitMiddleware() gin.HandlerFunc {
	go cleanupVisitors()

	return func(c *gin.Context) {
		ip := c.ClientIP()

		limiter := getVisitor(ip)

		if !limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Terlalu banyak percobaan login. Coba lagi dalam bebrapa menit.",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}