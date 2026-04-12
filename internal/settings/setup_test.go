package settings

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/benidevo/vega/internal/ai"
	"github.com/benidevo/vega/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock for quota service
type mockQuotaService struct {
	mock.Mock
}

func (m *mockQuotaService) GetAllQuotaStatus(ctx context.Context, userID int) (interface{}, error) {
	args := m.Called(ctx, userID)
	return args.Get(0), args.Error(1)
}

// Mock for auth service
type mockAuthService struct {
	mock.Mock
}

func (m *mockAuthService) ChangePassword(ctx context.Context, userID int, newPassword string) error {
	args := m.Called(ctx, userID, newPassword)
	return args.Error(0)
}

func (m *mockAuthService) VerifyPassword(hashedPassword, password string) bool {
	args := m.Called(hashedPassword, password)
	return args.Bool(0)
}

func (m *mockAuthService) DeleteAccount(ctx context.Context, userID int) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func TestSetup(t *testing.T) {
	t.Run("should_create_settings_handler_when_valid_dependencies", func(t *testing.T) {
		db, _, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		cfg := &config.Settings{
			DBConnectionString: "test.db",
			OpenAIAPIKey:       "test-key",
		}

		aiService := &ai.AIService{}
		quotaService := &mockQuotaService{}
		authService := &mockAuthService{}

		handler := Setup(cfg, db, aiService, quotaService, authService)

		assert.NotNil(t, handler)
		assert.NotNil(t, handler.service)
		assert.Equal(t, aiService, handler.aiService)
		assert.Equal(t, quotaService, handler.quotaService)
	})

	t.Run("should_create_handler_with_nil_services", func(t *testing.T) {
		db, _, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		cfg := &config.Settings{
			DBConnectionString: "test.db",
		}

		handler := Setup(cfg, db, nil, nil, nil)

		assert.NotNil(t, handler)
		assert.NotNil(t, handler.service)
		assert.Nil(t, handler.aiService)
		assert.Nil(t, handler.quotaService)
	})
}

func TestSetupWithService(t *testing.T) {
	t.Run("should_return_handler_and_service_when_called", func(t *testing.T) {
		db, _, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		cfg := &config.Settings{
			DBConnectionString: "test.db",
			OpenAIAPIKey:       "test-key",
		}

		aiService := &ai.AIService{}
		quotaService := &mockQuotaService{}
		authService := &mockAuthService{}

		handler, service := SetupWithService(cfg, db, aiService, quotaService, authService)

		assert.NotNil(t, handler)
		assert.NotNil(t, service)
		assert.Equal(t, handler.service, service)
		assert.Equal(t, aiService, handler.aiService)
		assert.Equal(t, quotaService, handler.quotaService)
		assert.Equal(t, cfg, service.cfg)
	})

	t.Run("should_create_same_repositories_for_handler_and_service", func(t *testing.T) {
		db, _, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		cfg := &config.Settings{
			DBConnectionString: "test.db",
		}

		handler, service := SetupWithService(cfg, db, nil, nil, nil)

		assert.NotNil(t, handler)
		assert.NotNil(t, service)
		assert.Equal(t, handler.service, service)
	})
}
