package auth

import (
	"github.com/benidevo/vega/internal/middleware"
	"github.com/gin-gonic/gin"
)

// RegisterPublicRoutes registers public authentication routes (login page and login action)
// to the provided Gin router group using the specified AuthHandler.
// Rate limiting is applied to login and refresh endpoints to prevent brute force attacks.
func RegisterPublicRoutes(router *gin.RouterGroup, handler *AuthHandler, authLimiter *middleware.RateLimiter) {
	router.GET("/login", handler.GetLoginPage)
	router.POST("/login", authLimiter.Middleware(), handler.Login)
	router.POST("/refresh", authLimiter.Middleware(), handler.RefreshToken)
}

// RegisterPrivateRoutes registers private authentication-related routes to the provided router group.
// It attaches handler functions for endpoints that require authentication, such as logout.
func RegisterPrivateRoutes(router *gin.RouterGroup, handler *AuthHandler) {
	router.POST("/logout", handler.Logout)
}

// RegisterGoogleAuthRoutes registers Google authentication routes to the provided router group.
// It attaches handler functions for Google login and callback endpoints.
func RegisterGoogleAuthRoutes(router *gin.RouterGroup, handler *GoogleAuthHandler) {
	router.GET("/google/login", handler.HandleLogin)
	router.GET("/google/callback", handler.HandleCallback)
}
