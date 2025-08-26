package services

import (
	"context"
	"testing"
	"time"

	"github.com/benidevo/vega/internal/auth/models"
	"github.com/benidevo/vega/internal/config"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestTokenTypes(t *testing.T) {
	cfg := &config.Settings{
		TokenSecret:        "test-secret-key",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
		AppName:            "TestApp",
	}

	user := &models.User{
		ID:       1,
		Username: "testuser",
		Role:     models.ADMIN,
	}

	t.Run("access_token_should_have_correct_type_and_expiry", func(t *testing.T) {
		accessToken, err := GenerateAccessToken(user, cfg)
		require.NoError(t, err)
		require.NotEmpty(t, accessToken)

		repo := new(MockUserRepository)
		authService := NewAuthService(repo, cfg)

		claims, err := authService.VerifyToken(accessToken)
		require.NoError(t, err)
		require.NotNil(t, claims)

		require.Equal(t, "access", claims.TokenType)

		// Check expiration is around 15 minutes
		now := time.Now().UTC()
		expiry := claims.ExpiresAt.Time
		diff := expiry.Sub(now)
		require.True(t, diff > 14*time.Minute, "Token expiry should be at least 14 minutes")
		require.True(t, diff < 16*time.Minute, "Token expiry should be at most 16 minutes")
	})

	t.Run("refresh_token_should_have_correct_type_and_expiry", func(t *testing.T) {
		refreshToken, err := GenerateRefreshToken(user, cfg)
		require.NoError(t, err)
		require.NotEmpty(t, refreshToken)

		repo := new(MockUserRepository)
		authService := NewAuthService(repo, cfg)

		claims, err := authService.VerifyToken(refreshToken)
		require.NoError(t, err)
		require.NotNil(t, claims)

		require.Equal(t, "refresh", claims.TokenType)

		// Check expiration is around 7 days
		now := time.Now().UTC()
		expiry := claims.ExpiresAt.Time
		diff := expiry.Sub(now)
		require.True(t, diff > 6*24*time.Hour, "Token expiry should be at least 6 days")
		require.True(t, diff < 8*24*time.Hour, "Token expiry should be at most 8 days")
	})

	t.Run("login_should_generate_both_token_types", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		ctx := context.Background()

		hashedPassword, err := hashPassword("password123")
		require.NoError(t, err)

		mockRepo.On("FindByUsername", ctx, "testuser").Return(&models.User{
			ID:       1,
			Username: "testuser",
			Password: hashedPassword,
			Role:     models.ADMIN,
		}, nil)

		mockRepo.On("UpdateUser", ctx, mock.AnythingOfType("*models.User")).Return(&models.User{}, nil)

		authService := NewAuthService(mockRepo, cfg)

		accessToken, refreshToken, _, err := authService.Login(ctx, "testuser", "password123")
		require.NoError(t, err)
		require.NotEmpty(t, accessToken)
		require.NotEmpty(t, refreshToken)

		accessClaims, err := authService.VerifyToken(accessToken)
		require.NoError(t, err)
		require.Equal(t, "access", accessClaims.TokenType)
		require.Equal(t, 1, accessClaims.UserID)

		refreshClaims, err := authService.VerifyToken(refreshToken)
		require.NoError(t, err)
		require.Equal(t, "refresh", refreshClaims.TokenType)
		require.Equal(t, 1, refreshClaims.UserID)

		mockRepo.AssertExpectations(t)
	})
}
