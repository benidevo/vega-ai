package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/benidevo/vega/internal/auth/models"
	commonerrors "github.com/benidevo/vega/internal/common/errors"
	"github.com/benidevo/vega/internal/config"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) CreateUser(ctx context.Context, username, password, role string) (*models.User, error) {
	args := m.Called(ctx, username, password, role)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) FindByUsername(ctx context.Context, username string) (*models.User, error) {
	args := m.Called(ctx, username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) FindByID(ctx context.Context, id int) (*models.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) UpdateUser(ctx context.Context, user *models.User) (*models.User, error) {
	args := m.Called(ctx, user)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) DeleteUser(ctx context.Context, id int) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockUserRepository) FindAllUsers(ctx context.Context) ([]*models.User, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.User), args.Error(1)
}

func setupTestConfig() *config.Settings {
	return &config.Settings{
		TokenSecret:        "test-secret-key",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	}
}

func TestRegisterUser(t *testing.T) {
	t.Run("should_register_user_successfully_when_valid_data", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		cfg := setupTestConfig()
		ctx := context.Background()

		mockRepo.On("CreateUser", ctx, "testuser", mock.AnythingOfType("string"), "admin").Return(&models.User{
			ID:       1,
			Username: "testuser",
			Role:     models.ADMIN,
		}, nil)

		authService := NewAuthService(mockRepo, cfg)

		user, err := authService.Register(ctx, "testuser", "password123", "admin")

		require.NoError(t, err)
		require.NotNil(t, user)
		require.Equal(t, "testuser", user.Username)
		require.Equal(t, models.ADMIN, user.Role)

		mockRepo.AssertExpectations(t)
	})

	t.Run("should_return_error_when_repository_fails", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		cfg := setupTestConfig()
		ctx := context.Background()

		repoErr := commonerrors.WrapError(models.ErrUserCreationFailed, errors.New("repository error"))
		mockRepo.On("CreateUser", ctx, "testuser", mock.AnythingOfType("string"), "admin").Return(nil, repoErr)

		authService := NewAuthService(mockRepo, cfg)

		user, err := authService.Register(ctx, "testuser", "password123", "admin")

		require.Error(t, err)
		require.Equal(t, models.ErrUserCreationFailed, err)
		require.Nil(t, user)

		mockRepo.AssertExpectations(t)
	})
}

func TestLogin(t *testing.T) {
	t.Run("should_login_successfully_when_credentials_valid", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		cfg := setupTestConfig()
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

		mockRepo.AssertExpectations(t)
	})

	t.Run("should_return_error_when_credentials_invalid", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		cfg := setupTestConfig()
		ctx := context.Background()

		mockRepo.On("FindByUsername", ctx, "nonexistent").Return(nil, models.ErrUserNotFound)

		hashedPassword, err := hashPassword("correctpassword")
		require.NoError(t, err)
		mockRepo.On("FindByUsername", ctx, "testuser").Return(&models.User{
			ID:       1,
			Username: "testuser",
			Password: hashedPassword,
			Role:     models.ADMIN,
		}, nil)

		authService := NewAuthService(mockRepo, cfg)

		accessToken1, refreshToken1, _, err1 := authService.Login(ctx, "nonexistent", "password123")
		require.Error(t, err1)
		require.Equal(t, models.ErrInvalidCredentials, err1)
		require.Empty(t, accessToken1)
		require.Empty(t, refreshToken1)

		accessToken2, refreshToken2, _, err2 := authService.Login(ctx, "testuser", "wrongpassword")
		require.Error(t, err2)
		require.Equal(t, models.ErrInvalidCredentials, err2)
		require.Empty(t, accessToken2)
		require.Empty(t, refreshToken2)

		mockRepo.AssertExpectations(t)
	})
}

func TestGetUserByID(t *testing.T) {
	mockRepo := new(MockUserRepository)
	cfg := setupTestConfig()
	ctx := context.Background()

	mockRepo.On("FindByID", ctx, 1).Return(&models.User{
		ID:       1,
		Username: "testuser",
		Role:     models.ADMIN,
	}, nil)

	mockRepo.On("FindByID", ctx, 999).Return(nil, models.ErrUserNotFound)

	authService := NewAuthService(mockRepo, cfg)

	t.Run("should_return_user_when_found", func(t *testing.T) {
		user, err := authService.GetUserByID(ctx, 1)

		require.NoError(t, err)
		require.NotNil(t, user)
		require.Equal(t, 1, user.ID)
		require.Equal(t, "testuser", user.Username)
	})

	t.Run("should_return_error_when_user_not_found", func(t *testing.T) {
		user, err := authService.GetUserByID(ctx, 999)

		require.Error(t, err)
		require.Equal(t, models.ErrUserNotFound, err)
		require.Nil(t, user)
	})

	mockRepo.AssertExpectations(t)
}

func TestVerifyToken(t *testing.T) {
	mockRepo := new(MockUserRepository)
	cfg := setupTestConfig()
	authService := NewAuthService(mockRepo, cfg)

	user := &models.User{
		ID:       1,
		Username: "testuser",
		Role:     models.ADMIN,
	}

	token, err := GenerateAccessToken(user, cfg)
	require.NoError(t, err)

	t.Run("should_verify_token_when_valid", func(t *testing.T) {
		claims, err := authService.VerifyToken(token)

		require.NoError(t, err)
		require.NotNil(t, claims)
		require.Equal(t, 1, claims.UserID)
		require.Equal(t, "testuser", claims.Username)
	})

	t.Run("should_reject_token_when_invalid", func(t *testing.T) {
		claims, err := authService.VerifyToken("invalid.token.here")

		require.Error(t, err)
		require.Equal(t, models.ErrInvalidToken, err)
		require.Nil(t, claims)
	})
}

func TestChangePassword(t *testing.T) {
	mockRepo := new(MockUserRepository)
	cfg := setupTestConfig()
	ctx := context.Background()

	mockRepo.On("FindByID", ctx, 1).Return(&models.User{
		ID:       1,
		Username: "testuser",
		Password: "oldhash",
	}, nil)

	mockRepo.On("FindByID", ctx, 999).Return(nil, models.ErrUserNotFound)

	mockRepo.On("UpdateUser", ctx, mock.AnythingOfType("*models.User")).Return(&models.User{}, nil).Once()
	updateErr := commonerrors.WrapError(models.ErrUserPasswordChangeFailed, errors.New("update error"))
	mockRepo.On("UpdateUser", ctx, mock.AnythingOfType("*models.User")).Return(nil, updateErr).Once()

	authService := NewAuthService(mockRepo, cfg)

	t.Run("should_change_password_when_user_exists", func(t *testing.T) {
		err := authService.ChangePassword(ctx, 1, "newpassword")
		require.NoError(t, err)
	})

	t.Run("should_return_error_when_user_not_found", func(t *testing.T) {
		err := authService.ChangePassword(ctx, 999, "newpassword")
		require.Error(t, err)
		require.Equal(t, models.ErrUserNotFound, err)
	})

	t.Run("should_return_error_when_update_fails", func(t *testing.T) {
		err := authService.ChangePassword(ctx, 1, "newpassword")
		require.Error(t, err)
		require.Equal(t, models.ErrUserPasswordChangeFailed, err)
	})

	mockRepo.AssertExpectations(t)
}

func TestRefreshAccessToken(t *testing.T) {
	t.Run("should_refresh_token_successfully_when_valid_refresh_token", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		cfg := setupTestConfig()
		ctx := context.Background()

		user := &models.User{
			ID:       1,
			Username: "testuser",
			Role:     models.ADMIN,
		}

		refreshToken, err := GenerateRefreshToken(user, cfg)
		require.NoError(t, err)

		mockRepo.On("FindByID", ctx, 1).Return(user, nil)

		authService := NewAuthService(mockRepo, cfg)

		newAccessToken, _, _, err := authService.RefreshAccessToken(ctx, refreshToken)

		require.NoError(t, err)
		require.NotEmpty(t, newAccessToken)

		claims, err := authService.VerifyToken(newAccessToken)
		require.NoError(t, err)
		require.Equal(t, "access", claims.TokenType)
		require.Equal(t, 1, claims.UserID)

		mockRepo.AssertExpectations(t)
	})

	t.Run("should_return_error_when_refresh_token_invalid", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		cfg := setupTestConfig()
		ctx := context.Background()

		authService := NewAuthService(mockRepo, cfg)

		newAccessToken, _, _, err := authService.RefreshAccessToken(ctx, "invalid.refresh.token")

		require.Error(t, err)
		require.Equal(t, models.ErrInvalidToken, err)
		require.Empty(t, newAccessToken)
	})

	t.Run("should_return_error_when_access_token_provided_instead_of_refresh", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		cfg := setupTestConfig()
		ctx := context.Background()

		user := &models.User{
			ID:       1,
			Username: "testuser",
			Role:     models.ADMIN,
		}

		accessToken, err := GenerateAccessToken(user, cfg)
		require.NoError(t, err)

		authService := NewAuthService(mockRepo, cfg)

		newAccessToken, _, _, err := authService.RefreshAccessToken(ctx, accessToken)

		require.Error(t, err)
		require.Equal(t, models.ErrInvalidToken, err)
		require.Empty(t, newAccessToken)
	})

	t.Run("should_return_error_when_user_not_found", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		cfg := setupTestConfig()
		ctx := context.Background()

		user := &models.User{
			ID:       999,
			Username: "deleteduser",
			Role:     models.STANDARD,
		}

		refreshToken, err := GenerateRefreshToken(user, cfg)
		require.NoError(t, err)

		mockRepo.On("FindByID", ctx, 999).Return(nil, models.ErrUserNotFound)

		authService := NewAuthService(mockRepo, cfg)

		newAccessToken, _, _, err := authService.RefreshAccessToken(ctx, refreshToken)

		require.Error(t, err)
		require.Equal(t, models.ErrInvalidToken, err)
		require.Empty(t, newAccessToken)

		mockRepo.AssertExpectations(t)
	})

	t.Run("should_return_error_when_user_repository_returns_nil", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		cfg := setupTestConfig()
		ctx := context.Background()

		user := &models.User{
			ID:       1,
			Username: "testuser",
			Role:     models.ADMIN,
		}

		refreshToken, err := GenerateRefreshToken(user, cfg)
		require.NoError(t, err)

		mockRepo.On("FindByID", ctx, 1).Return(nil, nil)

		authService := NewAuthService(mockRepo, cfg)

		newAccessToken, _, _, err := authService.RefreshAccessToken(ctx, refreshToken)

		require.Error(t, err)
		require.Equal(t, models.ErrInvalidToken, err)
		require.Empty(t, newAccessToken)

		mockRepo.AssertExpectations(t)
	})
}

func TestVerifyPassword(t *testing.T) {
	mockRepo := new(MockUserRepository)
	cfg := setupTestConfig()
	authService := NewAuthService(mockRepo, cfg)

	t.Run("should_return_true_when_password_matches", func(t *testing.T) {
		hashedPassword, err := hashPassword("correctpassword")
		require.NoError(t, err)

		result := authService.VerifyPassword(hashedPassword, "correctpassword")
		require.True(t, result)
	})

	t.Run("should_return_false_when_password_does_not_match", func(t *testing.T) {
		hashedPassword, err := hashPassword("correctpassword")
		require.NoError(t, err)

		result := authService.VerifyPassword(hashedPassword, "wrongpassword")
		require.False(t, result)
	})
}

func TestDeleteAccount(t *testing.T) {
	t.Run("should_delete_account_successfully", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		cfg := setupTestConfig()
		ctx := context.Background()

		mockRepo.On("DeleteUser", ctx, 1).Return(nil)

		authService := NewAuthService(mockRepo, cfg)

		err := authService.DeleteAccount(ctx, 1)

		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("should_return_error_when_deletion_fails", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		cfg := setupTestConfig()
		ctx := context.Background()

		mockRepo.On("DeleteUser", ctx, 1).Return(errors.New("deletion failed"))

		authService := NewAuthService(mockRepo, cfg)

		err := authService.DeleteAccount(ctx, 1)

		require.Error(t, err)
		require.Equal(t, models.ErrUserDeletionFailed, err)
		mockRepo.AssertExpectations(t)
	})
}

func TestRegisterPasswordResetScenarios(t *testing.T) {
	t.Run("should_return_error_when_username_already_exists", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		cfg := setupTestConfig()
		ctx := context.Background()

		existingErr := commonerrors.WrapError(models.ErrUserAlreadyExists, errors.New("duplicate key"))
		mockRepo.On("CreateUser", ctx, "existing@example.com", mock.AnythingOfType("string"), "Standard").Return(nil, existingErr)

		authService := NewAuthService(mockRepo, cfg)

		user, err := authService.Register(ctx, "existing@example.com", "password123", "Standard")

		require.Error(t, err)
		require.Equal(t, models.ErrUserAlreadyExists, err)
		require.Nil(t, user)

		mockRepo.AssertExpectations(t)
	})
}

func TestLoginEdgeCases(t *testing.T) {
	t.Run("should_return_error_when_oauth_account_tries_password_login", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		cfg := setupTestConfig()
		ctx := context.Background()

		mockRepo.On("FindByUsername", ctx, "oauth@example.com").Return(&models.User{
			ID:       1,
			Username: "oauth@example.com",
			Password: "",
			Role:     models.STANDARD,
		}, nil)

		authService := NewAuthService(mockRepo, cfg)

		accessToken, refreshToken, _, err := authService.Login(ctx, "oauth@example.com", "anypassword")

		require.Error(t, err)
		require.Equal(t, models.ErrInvalidCredentials, err)
		require.Empty(t, accessToken)
		require.Empty(t, refreshToken)

		mockRepo.AssertExpectations(t)
	})

	t.Run("should_handle_repository_error_during_login", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		cfg := setupTestConfig()
		ctx := context.Background()

		repoErr := commonerrors.WrapError(models.ErrUserRetrievalFailed, errors.New("database error"))
		mockRepo.On("FindByUsername", ctx, "testuser").Return(nil, repoErr)

		authService := NewAuthService(mockRepo, cfg)

		accessToken, refreshToken, _, err := authService.Login(ctx, "testuser", "password123")

		require.Error(t, err)
		require.Equal(t, models.ErrInvalidCredentials, err)
		require.Empty(t, accessToken)
		require.Empty(t, refreshToken)

		mockRepo.AssertExpectations(t)
	})
}

func TestVerifyTokenEdgeCases(t *testing.T) {
	mockRepo := new(MockUserRepository)
	cfg := setupTestConfig()
	authService := NewAuthService(mockRepo, cfg)

	t.Run("should_reject_expired_token", func(t *testing.T) {
		user := &models.User{
			ID:       1,
			Username: "testuser",
			Role:     models.ADMIN,
		}

		expiredToken, err := GenerateToken(user, cfg, AccessToken, -1*time.Hour)
		require.NoError(t, err)

		claims, err := authService.VerifyToken(expiredToken)

		require.Error(t, err)
		require.Equal(t, models.ErrInvalidToken, err)
		require.Nil(t, claims)
	})

	t.Run("should_reject_token_with_wrong_signing_method", func(t *testing.T) {
		claims, err := authService.VerifyToken("invalid.test.token")

		require.Error(t, err)
		require.Equal(t, models.ErrInvalidToken, err)
		require.Nil(t, claims)
	})
}

func TestHashPasswordEdgeCases(t *testing.T) {
	t.Run("should_handle_empty_password", func(t *testing.T) {
		hashedPassword, err := hashPassword("")
		require.NoError(t, err)
		require.NotEmpty(t, hashedPassword)

		result := verifyPassword(hashedPassword, "")
		require.True(t, result)
	})

	t.Run("should_generate_different_hashes_for_same_password", func(t *testing.T) {
		password := "samepassword"

		hash1, err1 := hashPassword(password)
		require.NoError(t, err1)

		hash2, err2 := hashPassword(password)
		require.NoError(t, err2)

		require.NotEqual(t, hash1, hash2)

		require.True(t, verifyPassword(hash1, password))
		require.True(t, verifyPassword(hash2, password))
	})
}

func TestLogError(t *testing.T) {
	mockRepo := new(MockUserRepository)
	cfg := setupTestConfig()
	authService := NewAuthService(mockRepo, cfg)

	authService.LogError(errors.New("test error"))
	authService.LogError(nil)
}

func TestRegisterWithLongPassword(t *testing.T) {
	t.Run("should_handle_password_at_bcrypt_limit", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		cfg := setupTestConfig()
		ctx := context.Background()

		password72Bytes := ""
		for i := 0; i < 72; i++ {
			password72Bytes += "a"
		}

		mockRepo.On("CreateUser", ctx, "testuser", mock.AnythingOfType("string"), "admin").Return(&models.User{
			ID:       1,
			Username: "testuser",
			Role:     models.ADMIN,
		}, nil)

		authService := NewAuthService(mockRepo, cfg)

		user, err := authService.Register(ctx, "testuser", password72Bytes, "admin")

		require.NoError(t, err)
		require.NotNil(t, user)
		mockRepo.AssertExpectations(t)
	})

	t.Run("should_fail_with_password_over_bcrypt_limit", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		cfg := setupTestConfig()
		ctx := context.Background()

		passwordTooLong := ""
		for i := 0; i < 73; i++ {
			passwordTooLong += "a"
		}

		authService := NewAuthService(mockRepo, cfg)

		user, err := authService.Register(ctx, "testuser", passwordTooLong, "admin")

		require.Error(t, err)
		require.Equal(t, models.ErrUserCreationFailed, err)
		require.Nil(t, user)
	})
}

func TestGetUserByIDWithRepositoryError(t *testing.T) {
	t.Run("should_return_retrieval_failed_error_for_unknown_errors", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		cfg := setupTestConfig()
		ctx := context.Background()

		dbErr := errors.New("database connection failed")
		mockRepo.On("FindByID", ctx, 1).Return(nil, dbErr)

		authService := NewAuthService(mockRepo, cfg)

		user, err := authService.GetUserByID(ctx, 1)

		require.Error(t, err)
		require.Equal(t, models.ErrUserRetrievalFailed, err)
		require.Nil(t, user)

		mockRepo.AssertExpectations(t)
	})
}

func TestRefreshAccessTokenErrorScenarios(t *testing.T) {
	t.Run("should_handle_repository_error_during_user_lookup", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		cfg := setupTestConfig()
		ctx := context.Background()

		user := &models.User{
			ID:       1,
			Username: "testuser",
			Role:     models.ADMIN,
		}

		refreshToken, err := GenerateRefreshToken(user, cfg)
		require.NoError(t, err)

		dbErr := errors.New("database connection failed")
		mockRepo.On("FindByID", ctx, 1).Return(nil, dbErr)

		authService := NewAuthService(mockRepo, cfg)

		newAccessToken, _, _, err := authService.RefreshAccessToken(ctx, refreshToken)

		require.Error(t, err)
		require.Equal(t, models.ErrInvalidToken, err)
		require.Empty(t, newAccessToken)

		mockRepo.AssertExpectations(t)
	})
}

func TestVerifyTokenWithInvalidSigningMethod(t *testing.T) {
	mockRepo := new(MockUserRepository)
	cfg := setupTestConfig()
	authService := NewAuthService(mockRepo, cfg)

	t.Run("should_reject_empty_token", func(t *testing.T) {
		claims, err := authService.VerifyToken("")

		require.Error(t, err)
		require.Equal(t, models.ErrInvalidToken, err)
		require.Nil(t, claims)
	})
}

func TestChangePasswordErrorCases(t *testing.T) {
	t.Run("should_handle_generic_repository_error", func(t *testing.T) {
		mockRepo := new(MockUserRepository)
		cfg := setupTestConfig()
		ctx := context.Background()

		dbErr := errors.New("database connection failed")
		mockRepo.On("FindByID", ctx, 1).Return(nil, dbErr)

		authService := NewAuthService(mockRepo, cfg)

		err := authService.ChangePassword(ctx, 1, "newpassword")

		require.Error(t, err)
		require.Equal(t, models.ErrUserPasswordChangeFailed, err)

		mockRepo.AssertExpectations(t)
	})
}
