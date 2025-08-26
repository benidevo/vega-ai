package auth

import (
	"context"
	"net/http"

	"github.com/benidevo/vega/internal/auth/services"
	"github.com/benidevo/vega/internal/common/logger"
	"github.com/gin-gonic/gin"
)

type authService interface {
	Login(ctx context.Context, username, password string) (string, string, int64, error)
	RefreshAccessToken(ctx context.Context, refreshToken string) (string, string, int64, error)
	VerifyToken(token string) (*services.Claims, error)
}

type AuthAPIHandler struct {
	authService authService
	log         *logger.PrivacyLogger
}

func NewAuthAPIHandler(authService authService) *AuthAPIHandler {
	return &AuthAPIHandler{
		authService: authService,
		log:         logger.GetPrivacyLogger("api_auth"),
	}
}

func (h *AuthAPIHandler) RefreshToken(ctx *gin.Context) {
	var request struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := ctx.ShouldBindJSON(&request); err != nil {
		h.log.Error().Err(err).Msg("Failed to bind refresh token request body")
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	accessToken, refreshToken, expiresAt, err := h.authService.RefreshAccessToken(ctx.Request.Context(), request.RefreshToken)
	if err != nil {
		h.log.Debug().Err(err).Msg("Failed to refresh access token")
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "failed to refresh access token"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"token": accessToken, "refresh_token": refreshToken, "expires_at": expiresAt})
}

func (h *AuthAPIHandler) VerifyToken(ctx *gin.Context) {
	authHeader := ctx.GetHeader("Authorization")
	if authHeader == "" {
		h.log.Debug().Msg("Missing Authorization header")
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
		return
	}

	const bearerPrefix = "Bearer "
	if len(authHeader) < len(bearerPrefix) || authHeader[:len(bearerPrefix)] != bearerPrefix {
		h.log.Debug().Msg("Invalid Authorization header format")
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
		return
	}

	token := authHeader[len(bearerPrefix):]
	claims, err := h.authService.VerifyToken(token)
	if err != nil {
		h.log.Debug().Err(err).Msg("Token verification failed")
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
		return
	}
	h.log.Debug().
		Int("user_id", claims.UserID).
		Str("username", claims.Username).
		Msg("Token verified successfully")

	ctx.JSON(http.StatusOK, gin.H{
		"valid":    true,
		"user_id":  claims.UserID,
		"username": claims.Username,
		"role":     claims.Role,
	})
}

func (h *AuthAPIHandler) Login(ctx *gin.Context) {
	var request struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := ctx.ShouldBindJSON(&request); err != nil {
		h.log.Error().Err(err).Msg("Failed to bind login request body")
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	accessToken, refreshToken, expiresAt, err := h.authService.Login(ctx.Request.Context(), request.Username, request.Password)
	if err != nil {
		h.log.Debug().
			Err(err).
			Str("username", request.Username).
			Msg("Login attempt failed")
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"token": accessToken, "refresh_token": refreshToken, "expires_at": expiresAt})
}
