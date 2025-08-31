package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("generates_request_id_when_missing", func(t *testing.T) {
		router := gin.New()
		router.Use(RequestID())

		var capturedID string
		router.GET("/test", func(c *gin.Context) {
			capturedID = GetRequestID(c)
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		assert.NotEmpty(t, capturedID)
		assert.Equal(t, capturedID, w.Header().Get(RequestIDHeader))
	})

	t.Run("uses_existing_request_id_from_header", func(t *testing.T) {
		router := gin.New()
		router.Use(RequestID())

		expectedID := "test-request-id-123"
		var capturedID string
		router.GET("/test", func(c *gin.Context) {
			capturedID = GetRequestID(c)
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.Header.Set(RequestIDHeader, expectedID)
		router.ServeHTTP(w, req)

		assert.Equal(t, expectedID, capturedID)
		assert.Equal(t, expectedID, w.Header().Get(RequestIDHeader))
	})

	t.Run("request_id_available_in_context", func(t *testing.T) {
		router := gin.New()
		router.Use(RequestID())

		router.GET("/test", func(c *gin.Context) {
			id, exists := c.Get(RequestIDKey)
			assert.True(t, exists)
			assert.NotEmpty(t, id)
			c.JSON(http.StatusOK, gin.H{"request_id": id})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestGetRequestID(t *testing.T) {
	t.Run("returns_empty_string_when_not_set", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		id := GetRequestID(c)
		assert.Empty(t, id)
	})

	t.Run("returns_request_id_when_set", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		expectedID := "test-id-456"
		c.Set(RequestIDKey, expectedID)

		id := GetRequestID(c)
		assert.Equal(t, expectedID, id)
	})

	t.Run("handles_wrong_type_in_context", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		c.Set(RequestIDKey, 12345)

		id := GetRequestID(c)
		assert.Empty(t, id)
	})
}

func TestWithRequestID(t *testing.T) {
	t.Run("returns_logger_with_request_id", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		expectedID := "test-logger-id"
		c.Set(RequestIDKey, expectedID)

		logger := WithRequestID(c)
		assert.NotNil(t, logger)
	})

	t.Run("returns_logger_without_request_id_when_missing", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		logger := WithRequestID(c)
		assert.NotNil(t, logger)
	})
}
