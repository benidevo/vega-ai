package services

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/benidevo/vega/internal/auth/models"
	repomocks "github.com/benidevo/vega/internal/auth/repository/mocks"
	commonerrors "github.com/benidevo/vega/internal/common/errors"
	"github.com/benidevo/vega/internal/common/logger"
	"github.com/benidevo/vega/internal/config"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

// setupGoogleAuthTestConfig prepares common test configuration
func setupGoogleAuthTestConfig() (*config.Settings, context.Context) {
	cfg := &config.Settings{
		TokenSecret:             "test-secret-key",
		GoogleAuthUserInfoURL:   "https://www.googleapis.com/oauth2/v1/userinfo",
		GoogleAuthUserInfoScope: "https://www.googleapis.com/auth/userinfo.email",
	}
	ctx := context.Background()
	return cfg, ctx
}

func TestGetOrCreateUser(t *testing.T) {
	cfg, ctx := setupGoogleAuthTestConfig()
	log := logger.GetPrivacyLogger("test")

	userInfo := &GoogleAuthUserInfo{
		ID:            "12345",
		Email:         "test@example.com",
		VerifiedEmail: true,
	}

	t.Run("should_return_existing_user", func(t *testing.T) {
		mockRepo := repomocks.NewMockUserRepository(t)
		service := &GoogleAuthService{
			cfg:  cfg,
			repo: mockRepo,
			log:  log,
		}

		existingUser := &models.User{
			ID:       1,
			Username: "test@example.com",
			Role:     models.STANDARD,
		}
		mockRepo.On("FindByUsername", ctx, "test@example.com").Return(existingUser, nil)

		user, err := service.getOrCreateUser(ctx, userInfo)

		require.NoError(t, err)
		require.NotNil(t, user)
		require.Equal(t, existingUser.ID, user.ID)
		require.Equal(t, existingUser.Username, user.Username)
	})

	t.Run("should_create_new_user_when_not_found", func(t *testing.T) {
		mockRepo := repomocks.NewMockUserRepository(t)
		service := &GoogleAuthService{
			cfg:  cfg,
			repo: mockRepo,
			log:  log,
		}

		mockRepo.On("FindByUsername", ctx, "test@example.com").Return(nil, models.ErrUserNotFound)

		newUser := &models.User{
			ID:       2,
			Username: "test@example.com",
			Role:     models.STANDARD,
		}
		mockRepo.On("CreateUser", ctx, "test@example.com", "", "Standard").Return(newUser, nil)

		user, err := service.getOrCreateUser(ctx, userInfo)

		require.NoError(t, err)
		require.NotNil(t, user)
		require.Equal(t, newUser.ID, user.ID)
		require.Equal(t, newUser.Username, user.Username)
	})

	t.Run("should_return_error_when_user_lookup_fails", func(t *testing.T) {
		mockRepo := repomocks.NewMockUserRepository(t)
		service := &GoogleAuthService{
			cfg:  cfg,
			repo: mockRepo,
			log:  log,
		}

		repoErr := commonerrors.WrapError(models.ErrUserRetrievalFailed, errors.New("database error"))
		mockRepo.On("FindByUsername", ctx, "test@example.com").Return(nil, repoErr)

		user, err := service.getOrCreateUser(ctx, userInfo)

		require.Error(t, err)
		require.Equal(t, models.ErrGoogleUserCreationFailed, err)
		require.Nil(t, user)
	})

	t.Run("should_return_error_when_user_creation_fails", func(t *testing.T) {
		mockRepo := repomocks.NewMockUserRepository(t)
		service := &GoogleAuthService{
			cfg:  cfg,
			repo: mockRepo,
			log:  log,
		}

		mockRepo.On("FindByUsername", ctx, "test@example.com").Return(nil, models.ErrUserNotFound)

		repoErr := commonerrors.WrapError(models.ErrUserCreationFailed, errors.New("database error"))
		mockRepo.On("CreateUser", ctx, "test@example.com", "", "Standard").Return(nil, repoErr)

		user, err := service.getOrCreateUser(ctx, userInfo)

		require.Error(t, err)
		require.Equal(t, models.ErrGoogleUserCreationFailed, err)
		require.Nil(t, user)
	})
}

func TestGetAuthURL(t *testing.T) {
	cfg, _ := setupGoogleAuthTestConfig()
	log := logger.GetPrivacyLogger("test")

	t.Run("should_return_auth_url_with_state", func(t *testing.T) {
		service := &GoogleAuthService{
			cfg: cfg,
			log: log,
			oauthCfg: &oauth2.Config{
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
				RedirectURL:  "http://localhost/auth/google/callback",
				Endpoint: oauth2.Endpoint{
					AuthURL:  "https://accounts.google.com/o/oauth2/auth",
					TokenURL: "https://oauth2.googleapis.com/token",
				},
			},
		}

		url, state := service.GetAuthURL()

		require.Contains(t, url, "https://accounts.google.com/o/oauth2/auth")
		require.Contains(t, url, "client_id=test-client-id")
		require.Contains(t, url, "redirect_uri=http%3A%2F%2Flocalhost%2Fauth%2Fgoogle%2Fcallback")
		require.Contains(t, url, "access_type=offline")
		require.Contains(t, url, "state="+state)
		require.Len(t, state, 32)
		require.NotEmpty(t, state)
	})
}

func TestGenerateRandomState(t *testing.T) {
	lengths := []int{8, 16, 32, 64}

	for _, length := range lengths {
		state := generateRandomState(length)

		require.Equal(t, length, len(state))

		for _, char := range state {
			require.True(t,
				(char >= 'a' && char <= 'z') ||
					(char >= 'A' && char <= 'Z') ||
					(char >= '0' && char <= '9'),
				"Character '%c' is not alphanumeric", char)
		}

		anotherState := generateRandomState(length)
		require.NotEqual(t, state, anotherState, "Two generated states should differ")
	}
}

func TestNewGoogleAuthService(t *testing.T) {
	t.Run("should_create_service_successfully_with_valid_config", func(t *testing.T) {
		cfg := &config.Settings{
			GoogleClientID:          "test-client-id",
			GoogleClientSecret:      "test-client-secret",
			GoogleClientRedirectURL: "http://localhost/auth/google/callback",
			GoogleAuthUserInfoURL:   "https://www.googleapis.com/oauth2/v1/userinfo",
			GoogleAuthUserInfoScope: "https://www.googleapis.com/auth/userinfo.email",
		}
		mockRepo := repomocks.NewMockUserRepository(t)

		service, err := NewGoogleAuthService(cfg, mockRepo)

		require.NoError(t, err)
		require.NotNil(t, service)
		require.Equal(t, cfg, service.cfg)
		require.NotNil(t, service.oauthCfg)
		require.NotNil(t, service.log)
		require.Equal(t, mockRepo, service.repo)
	})

	t.Run("should_return_error_when_credentials_missing", func(t *testing.T) {
		cfg := &config.Settings{
			GoogleClientID:          "", // Missing client ID
			GoogleClientSecret:      "test-client-secret",
			GoogleClientRedirectURL: "http://localhost/auth/google/callback",
			GoogleAuthUserInfoURL:   "https://www.googleapis.com/oauth2/v1/userinfo",
			GoogleAuthUserInfoScope: "https://www.googleapis.com/auth/userinfo.email",
		}
		mockRepo := repomocks.NewMockUserRepository(t)

		service, err := NewGoogleAuthService(cfg, mockRepo)

		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to read Google credentials file")
		require.Nil(t, service)
	})
}

// Mock HTTP client for testing
type mockHTTPClient struct {
	response *http.Response
	err      error
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.response, m.err
}

func TestGetGoogleCredentials(t *testing.T) {
	t.Run("should_return_oauth_config_with_valid_credentials", func(t *testing.T) {
		clientID := "test-client-id"
		clientSecret := "test-client-secret"
		redirectURL := "http://localhost/auth/google/callback"
		scope := "https://www.googleapis.com/auth/userinfo.email"

		oauthCfg, err := getGoogleCredentials(clientID, clientSecret, redirectURL, scope)

		require.NoError(t, err)
		require.NotNil(t, oauthCfg)
		require.Equal(t, clientID, oauthCfg.ClientID)
		require.Equal(t, clientSecret, oauthCfg.ClientSecret)
		require.Equal(t, redirectURL, oauthCfg.RedirectURL)
		require.Contains(t, oauthCfg.Scopes, scope)
		require.Equal(t, "https://accounts.google.com/o/oauth2/auth", oauthCfg.Endpoint.AuthURL)
		require.Equal(t, "https://oauth2.googleapis.com/token", oauthCfg.Endpoint.TokenURL)
	})

	t.Run("should_return_error_when_client_id_missing", func(t *testing.T) {
		clientID := ""
		clientSecret := "test-client-secret"
		redirectURL := "http://localhost/auth/google/callback"
		scope := "https://www.googleapis.com/auth/userinfo.email"

		oauthCfg, err := getGoogleCredentials(clientID, clientSecret, redirectURL, scope)

		require.Error(t, err)
		require.Nil(t, oauthCfg)
		require.Equal(t, models.ErrGoogleCredentialsInvalid, err)
	})

	t.Run("should_return_error_when_client_secret_missing", func(t *testing.T) {
		clientID := "test-client-id"
		clientSecret := ""
		redirectURL := "http://localhost/auth/google/callback"
		scope := "https://www.googleapis.com/auth/userinfo.email"

		oauthCfg, err := getGoogleCredentials(clientID, clientSecret, redirectURL, scope)

		require.Error(t, err)
		require.Nil(t, oauthCfg)
		require.Equal(t, models.ErrGoogleCredentialsInvalid, err)
	})

	t.Run("should_allow_empty_redirect_url", func(t *testing.T) {
		clientID := "test-client-id"
		clientSecret := "test-client-secret"
		redirectURL := ""
		scope := "https://www.googleapis.com/auth/userinfo.email"

		oauthCfg, err := getGoogleCredentials(clientID, clientSecret, redirectURL, scope)

		require.NoError(t, err)
		require.NotNil(t, oauthCfg)
		require.Equal(t, "", oauthCfg.RedirectURL)
	})
}

func TestOAuthLogError(t *testing.T) {
	cfg := &config.Settings{
		GoogleClientID:          "test-client-id",
		GoogleClientSecret:      "test-client-secret",
		GoogleClientRedirectURL: "http://localhost/auth/google/callback",
		GoogleAuthUserInfoURL:   "https://www.googleapis.com/oauth2/v1/userinfo",
		GoogleAuthUserInfoScope: "https://www.googleapis.com/auth/userinfo.email",
	}
	mockRepo := repomocks.NewMockUserRepository(t)

	service, err := NewGoogleAuthService(cfg, mockRepo)
	require.NoError(t, err)

	service.LogError(errors.New("test oauth error"))
	service.LogError(nil)
}

func TestGenerateRandomStateFallback(t *testing.T) {
	// Test the fallback behavior in generateRandomState
	// This would require simulating a crypto/rand failure, which is difficult
	// We can at least ensure the function works with different lengths
	lengths := []int{1, 10, 100}

	for _, length := range lengths {
		state := generateRandomState(length)
		require.Len(t, state, length)

		// Verify all characters are alphanumeric
		for _, char := range state {
			require.True(t,
				(char >= 'a' && char <= 'z') ||
					(char >= 'A' && char <= 'Z') ||
					(char >= '0' && char <= '9'),
				"Character '%c' is not alphanumeric", char)
		}
	}
}
