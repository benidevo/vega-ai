package auth

import "github.com/gin-gonic/gin"

func RegisterRoutes(router *gin.RouterGroup, handler *AuthAPIHandler) {
	router.POST("/refresh", handler.RefreshToken)
	router.POST("/login", handler.Login)
	router.GET("/verify", handler.VerifyToken)
}
