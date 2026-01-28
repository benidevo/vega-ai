package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// rateLimitEntry tracks request counts for a single IP
type rateLimitEntry struct {
	count     int
	firstSeen time.Time
}

// RateLimiter provides IP-based rate limiting
type RateLimiter struct {
	mu       sync.RWMutex
	requests map[string]*rateLimitEntry
	limit    int
	window   time.Duration
	done     chan struct{}
}

// NewRateLimiter creates a new rate limiter with the specified limit per window
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string]*rateLimitEntry),
		limit:    limit,
		window:   window,
		done:     make(chan struct{}),
	}

	go rl.cleanup()

	return rl
}

// Stop signals the cleanup goroutine to exit
func (rl *RateLimiter) Stop() {
	close(rl.done)
}

// cleanup periodically removes expired entries
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			for ip, entry := range rl.requests {
				if now.Sub(entry.firstSeen) > rl.window {
					delete(rl.requests, ip)
				}
			}
			rl.mu.Unlock()
		case <-rl.done:
			return
		}
	}
}

// Allow checks if a request from the given IP should be allowed
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, exists := rl.requests[ip]

	if !exists || now.Sub(entry.firstSeen) > rl.window {
		rl.requests[ip] = &rateLimitEntry{
			count:     1,
			firstSeen: now,
		}
		return true
	}

	if entry.count >= rl.limit {
		return false
	}

	entry.count++
	return true
}

// Middleware returns a Gin middleware handler for this rate limiter
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		if !rl.Allow(ip) {
			c.Header("Retry-After", "60")
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many requests. Please try again later.",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// NewAuthRateLimiter creates a rate limiter configured for authentication endpoints.
// Returns the limiter so callers can share one instance across multiple routes
// and call Stop() on shutdown.
func NewAuthRateLimiter() *RateLimiter {
	return NewRateLimiter(5, time.Minute)
}
