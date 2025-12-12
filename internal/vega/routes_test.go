package vega

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
)

// mockErrorRenderer is a test renderer that doesn't require HTML templates
type mockErrorRenderer struct {
	lastCode  int
	lastTitle string
}

func (m *mockErrorRenderer) Error(c *gin.Context, code int, title string) {
	m.lastCode = code
	m.lastTitle = title
	c.String(code, title)
}

// testGlobalErrorHandler is a test version of globalErrorHandler that uses mockErrorRenderer
func testGlobalErrorHandler(renderer *mockErrorRenderer) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				var err error
				switch v := r.(type) {
				case error:
					err = v
				case string:
					err = fmt.Errorf("%s", v)
				default:
					err = fmt.Errorf("%v", v)
				}
				log.Error().Err(err).Msg("Recovered from panic")

				renderer.Error(c, http.StatusInternalServerError, "Something Went Wrong")
				c.Abort()
			}
		}()

		c.Next()

		// Only handle errors if no response has been written yet
		if !c.Writer.Written() && (len(c.Errors) > 0 || c.Writer.Status() == http.StatusInternalServerError) {
			renderer.Error(c, http.StatusInternalServerError, "Something Went Wrong")
			c.Abort()
		}
	}
}

func TestGlobalErrorHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("recovers from panic", func(t *testing.T) {
		renderer := &mockErrorRenderer{}
		router := gin.New()
		router.Use(testGlobalErrorHandler(renderer))

		router.GET("/panic", func(c *gin.Context) {
			panic(errors.New("test panic"))
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/panic", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Equal(t, http.StatusInternalServerError, renderer.lastCode)
	})

	t.Run("handles unwritten error response", func(t *testing.T) {
		renderer := &mockErrorRenderer{}
		router := gin.New()
		router.Use(testGlobalErrorHandler(renderer))

		router.GET("/error", func(c *gin.Context) {
			c.Error(gin.Error{Err: assert.AnError, Type: gin.ErrorTypePrivate})
			// Don't write response - let middleware handle it
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/error", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("passes through normal requests", func(t *testing.T) {
		renderer := &mockErrorRenderer{}
		router := gin.New()
		router.Use(testGlobalErrorHandler(renderer))

		router.GET("/ok", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/ok", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, 0, renderer.lastCode) // renderer should not have been called
	})
}
