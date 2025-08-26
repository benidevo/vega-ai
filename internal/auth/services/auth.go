package services

import (
	"context"
	"fmt"
	"time"

	"github.com/benidevo/vega/internal/auth/models"
	"github.com/benidevo/vega/internal/auth/repository"
	"github.com/benidevo/vega/internal/common/logger"
	"github.com/benidevo/vega/internal/config"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Claims struct {
	UserID    int    `json:"user_id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	TokenType string `json:"token_type,omitempty"` // "access" or "refresh"
	jwt.RegisteredClaims
}

type AuthService struct {
	repo   repository.UserRepository
	config *config.Settings
	log    *logger.PrivacyLogger
}

func NewAuthService(repo repository.UserRepository, config *config.Settings) *AuthService {
	return &AuthService{repo: repo, config: config, log: logger.GetPrivacyLogger("auth")}
}

func (s *AuthService) LogError(err error) {
	s.log.Error().Err(err).Msg("Authentication error")
}

// Register creates a new user with the provided username, password, and role.
// It returns the user and models.ErrUserAlreadyExists if a user with that username already exists.
func (s *AuthService) Register(ctx context.Context, username, password, role string) (*models.User, error) {
	hashedPassword, err := hashPassword(password)
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to hash password")
		return nil, models.ErrUserCreationFailed
	}

	user, err := s.repo.CreateUser(ctx, username, hashedPassword, role)
	if err != nil {
		sentinelErr := models.GetSentinelError(err)

		if sentinelErr == models.ErrUserAlreadyExists {
			s.log.Warn().
				Str("event", "user_registration_duplicate").
				Str("hashed_id", logger.HashIdentifier(username)).
				Msg("User already exists")
			return user, models.ErrUserAlreadyExists
		}

		s.log.Error().Err(err).
			Str("event", "user_registration_failed").
			Str("hashed_id", logger.HashIdentifier(username)).
			Msg("Failed to create user")
		return nil, sentinelErr
	}

	s.log.LogRegistrationEvent("user_registered", logger.HashIdentifier(username), true)
	return user, nil
}

func (s *AuthService) Login(ctx context.Context, username, password string) (string, string, int64, error) {
	user, err := s.repo.FindByUsername(ctx, username)
	if err != nil {
		sentinelErr := models.GetSentinelError(err)
		if sentinelErr == models.ErrUserNotFound {
			s.log.Info().
				Str("event", "login_attempt_unknown_user").
				Str("hashed_id", logger.HashIdentifier(username)).
				Msg("Login attempt for non-existent user")
		} else {
			s.log.Error().Err(err).
				Str("event", "login_user_retrieval_error").
				Str("hashed_id", logger.HashIdentifier(username)).
				Msg("Error retrieving user during login")
		}

		return "", "", 0, models.ErrInvalidCredentials
	}

	if user.Password == "" {
		s.log.Error().
			Str("event", "login_oauth_account_password_attempt").
			Str("user_ref", fmt.Sprintf("user_%d", user.ID)).
			Msg("User password is empty. Account was created using Google authentication")
		return "", "", 0, models.ErrInvalidCredentials
	}

	if !verifyPassword(user.Password, password) {
		s.log.LogAuthEvent("login_invalid_password", user.ID, false)
		return "", "", 0, models.ErrInvalidCredentials
	}

	accessToken, err := GenerateAccessToken(user, s.config)
	if err != nil {
		s.log.Error().Err(err).
			Str("event", "access_token_generation_failed").
			Str("user_ref", fmt.Sprintf("user_%d", user.ID)).
			Msg("Failed to generate access token")
		return "", "", 0, models.ErrInvalidCredentials
	}

	refreshToken, err := GenerateRefreshToken(user, s.config)
	if err != nil {
		s.log.Error().Err(err).
			Str("event", "refresh_token_generation_failed").
			Str("user_ref", fmt.Sprintf("user_%d", user.ID)).
			Msg("Failed to generate refresh token")
		return "", "", 0, models.ErrInvalidCredentials
	}
	expiresAt := time.Now().UTC().Add(s.config.AccessTokenExpiry).Unix()

	user.LastLogin = time.Now().UTC()
	_, err = s.repo.UpdateUser(ctx, user)
	if err != nil {
		sentinelErr := models.GetSentinelError(err)
		s.log.Warn().Err(err).
			Str("event", "last_login_update_failed").
			Str("user_ref", fmt.Sprintf("user_%d", user.ID)).
			Str("error_type", sentinelErr.Error()).
			Msg("Failed to update user last login")
	}

	s.log.LogAuthEvent("login_success", user.ID, true)
	return accessToken, refreshToken, expiresAt, nil
}

func (s *AuthService) RefreshAccessToken(ctx context.Context, refreshToken string) (string, string, int64, error) {
	claims, err := s.VerifyToken(refreshToken)
	if err != nil {
		s.log.Error().Err(err).Msg("Invalid refresh token")
		return "", "", 0, models.ErrInvalidToken
	}

	if claims.TokenType != "refresh" {
		s.log.Error().Msg("Token provided is not a refresh token")
		return "", "", 0, models.ErrInvalidToken
	}

	user, err := s.repo.FindByID(ctx, claims.UserID)
	if err != nil {
		sentinelErr := models.GetSentinelError(err)
		s.log.Error().Err(err).
			Str("event", "token_refresh_user_not_found").
			Str("user_ref", fmt.Sprintf("user_%d", claims.UserID)).
			Str("error_type", sentinelErr.Error()).
			Msg("Failed to find user for token refresh")
		return "", "", 0, models.ErrInvalidToken
	}

	if user == nil {
		s.log.Error().
			Str("event", "token_refresh_user_nil").
			Str("user_ref", fmt.Sprintf("user_%d", claims.UserID)).
			Msg("User not found for token refresh")
		return "", "", 0, models.ErrInvalidToken
	}

	accessToken, err := GenerateAccessToken(user, s.config)
	if err != nil {
		s.log.Error().Err(err).
			Str("event", "token_refresh_failed").
			Str("user_ref", fmt.Sprintf("user_%d", user.ID)).
			Msg("Failed to generate new access token")
		return "", "", 0, models.ErrTokenCreationFailed
	}
	newRefreshToken, err := GenerateRefreshToken(user, s.config)
	if err != nil {
		s.log.Error().Err(err).
			Str("event", "refresh_token_rotation_failed").
			Str("user_ref", fmt.Sprintf("user_%d", user.ID)).
			Msg("Failed to generate new refresh token")
		newRefreshToken = refreshToken
	}
	expiresAt := time.Now().UTC().Add(s.config.AccessTokenExpiry).Unix()

	s.log.Info().
		Str("event", "token_refreshed").
		Str("user_ref", fmt.Sprintf("user_%d", user.ID)).
		Msg("Access token refreshed successfully")
	return accessToken, newRefreshToken, expiresAt, nil
}

func (s *AuthService) GetUserByID(ctx context.Context, userID int) (*models.User, error) {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		sentinelErr := models.GetSentinelError(err)
		s.log.Error().Err(err).
			Str("event", "user_lookup_failed").
			Str("user_ref", fmt.Sprintf("user_%d", userID)).
			Str("error_type", sentinelErr.Error()).
			Msg("Failed to find user by ID")

		if sentinelErr == models.ErrUserNotFound {
			return nil, models.ErrUserNotFound
		}
		return nil, models.ErrUserRetrievalFailed
	}
	return user, nil
}

func (s *AuthService) VerifyToken(token string) (*Claims, error) {
	claims := &Claims{}
	parsedToken, err := jwt.ParseWithClaims(token, claims, func(jwtToken *jwt.Token) (interface{}, error) {
		if jwtToken.Method != jwt.SigningMethodHS256 {
			s.log.Error().Msg("Unexpected signing method")
			return nil, models.ErrInvalidToken
		}
		return []byte(s.config.TokenSecret), nil
	})

	if err != nil {
		s.log.Error().Err(err).Msg("Failed to parse token")
		return nil, models.ErrInvalidToken
	}

	if !parsedToken.Valid {
		s.log.Error().Msg("Token is not valid")
		return nil, models.ErrInvalidToken
	}

	return claims, nil
}

func (s *AuthService) ChangePassword(ctx context.Context, userID int, newPassword string) error {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		sentinelErr := models.GetSentinelError(err)
		s.log.Error().Err(err).
			Str("event", "password_change_user_not_found").
			Str("user_ref", fmt.Sprintf("user_%d", userID)).
			Str("error_type", sentinelErr.Error()).
			Msg("Failed to find user for password change")
		if sentinelErr == models.ErrUserNotFound {
			return models.ErrUserNotFound
		}
		return models.ErrUserPasswordChangeFailed
	}

	hashedPassword, err := hashPassword(newPassword)
	if err != nil {
		s.log.Error().Err(err).
			Str("event", "password_hash_failed").
			Str("user_ref", fmt.Sprintf("user_%d", userID)).
			Msg("Failed to hash password")
		return models.ErrUserPasswordChangeFailed
	}

	user.Password = hashedPassword
	_, err = s.repo.UpdateUser(ctx, user)
	if err != nil {
		sentinelErr := models.GetSentinelError(err)
		s.log.Error().Err(err).
			Str("event", "password_update_failed").
			Str("user_ref", fmt.Sprintf("user_%d", userID)).
			Str("error_type", sentinelErr.Error()).
			Msg("Failed to update user password")
		return models.ErrUserPasswordChangeFailed
	}

	s.log.Info().
		Str("event", "password_changed").
		Str("user_ref", fmt.Sprintf("user_%d", userID)).
		Msg("User password changed successfully")
	return nil
}

func (s *AuthService) VerifyPassword(hashedPassword, password string) bool {
	return verifyPassword(hashedPassword, password)
}

func (s *AuthService) DeleteAccount(ctx context.Context, userID int) error {
	s.log.Info().
		Str("event", "account_deletion_requested").
		Str("user_ref", fmt.Sprintf("user_%d", userID)).
		Msg("Account deletion requested")

	if err := s.repo.DeleteUser(ctx, userID); err != nil {
		s.log.Error().Err(err).
			Str("event", "account_deletion_failed").
			Str("user_ref", fmt.Sprintf("user_%d", userID)).
			Msg("Failed to delete user account")
		return models.ErrUserDeletionFailed
	}

	s.log.Info().
		Str("event", "account_deleted").
		Str("user_ref", fmt.Sprintf("user_%d", userID)).
		Msg("User account deleted successfully")

	return nil
}

type TokenType string

const (
	AccessToken  TokenType = "access"
	RefreshToken TokenType = "refresh"
)

func GenerateToken(user *models.User, cfg *config.Settings, tokenType TokenType, expiry time.Duration) (string, error) {
	expirationTime := time.Now().UTC().Add(expiry)
	role := user.Role.String()

	claims := &Claims{
		UserID:    user.ID,
		Username:  user.Username,
		Role:      role,
		TokenType: string(tokenType),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    cfg.AppName,
			Subject:   user.Username,
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.TokenSecret))
}

func GenerateAccessToken(user *models.User, cfg *config.Settings) (string, error) {
	return GenerateToken(user, cfg, AccessToken, cfg.AccessTokenExpiry)
}

func GenerateRefreshToken(user *models.User, cfg *config.Settings) (string, error) {
	return GenerateToken(user, cfg, RefreshToken, cfg.RefreshTokenExpiry)
}

func hashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedPassword), nil
}

func verifyPassword(hashedPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}
