package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRequestTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("completes_before_timeout", func(t *testing.T) {
		router := gin.New()
		router.Use(RequestTimeout(100 * time.Millisecond))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "ok")
	})

	t.Run("context_timeout_is_set", func(t *testing.T) {
		router := gin.New()
		router.Use(RequestTimeout(50 * time.Millisecond))

		var ctxDeadline time.Time
		router.GET("/test", func(c *gin.Context) {
			deadline, ok := c.Request.Context().Deadline()
			assert.True(t, ok, "context should have deadline")
			ctxDeadline = deadline
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.False(t, ctxDeadline.IsZero())
	})

	t.Run("handlers_can_check_context_cancellation", func(t *testing.T) {
		router := gin.New()
		router.Use(RequestTimeout(50 * time.Millisecond))

		router.GET("/test", func(c *gin.Context) {
			select {
			case <-time.After(100 * time.Millisecond):
				c.JSON(http.StatusOK, gin.H{"status": "should_not_reach"})
			case <-c.Request.Context().Done():
				return
			}
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		time.Sleep(60 * time.Millisecond)
	})
}

func TestDatabaseTimeout(t *testing.T) {
	t.Run("creates_timeout_context", func(t *testing.T) {
		ctx := context.Background()
		timeoutCtx, cancel := DatabaseTimeout(ctx, 100*time.Millisecond)
		defer cancel()

		deadline, ok := timeoutCtx.Deadline()
		assert.True(t, ok, "context should have deadline")
		assert.WithinDuration(t, time.Now().Add(100*time.Millisecond), deadline, 10*time.Millisecond)
	})

	t.Run("context_cancels_after_timeout", func(t *testing.T) {
		ctx := context.Background()
		timeoutCtx, cancel := DatabaseTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		time.Sleep(60 * time.Millisecond)
		assert.Error(t, timeoutCtx.Err())
		assert.Equal(t, context.DeadlineExceeded, timeoutCtx.Err())
	})
}
