package job

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/benidevo/vega/internal/ai"
	"github.com/benidevo/vega/internal/cache"
	"github.com/benidevo/vega/internal/config"
	"github.com/benidevo/vega/internal/quota"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetup(t *testing.T) {
	tests := []struct {
		name  string
		cfg   *config.Settings
		cache cache.Cache
	}{
		{
			name: "should_initialize_job_handler_with_dependencies",
			cfg: &config.Settings{
				IsCloudMode:  false,
				IsTest:       true,
				TokenSecret:  "test-secret",
				GeminiAPIKey: "test-api-key",
			},
			cache: cache.NewNoOpCache(),
		},
		{
			name: "should_initialize_job_handler_in_cloud_mode",
			cfg: &config.Settings{
				IsCloudMode:  true,
				IsTest:       true,
				TokenSecret:  "test-secret",
				GeminiAPIKey: "test-api-key",
			},
			cache: cache.NewNoOpCache(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, _, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			handler, err := Setup(db, tt.cfg, tt.cache)

			require.NoError(t, err)
			assert.NotNil(t, handler)
			assert.NotNil(t, handler.service)
			assert.Equal(t, tt.cfg, handler.cfg)
		})
	}
}

func TestSetupService(t *testing.T) {
	tests := []struct {
		name  string
		cfg   *config.Settings
		cache cache.Cache
	}{
		{
			name: "should_initialize_job_service_with_all_dependencies",
			cfg: &config.Settings{
				IsCloudMode:  false,
				TokenSecret:  "test-secret",
				GeminiAPIKey: "test-api-key",
			},
			cache: cache.NewNoOpCache(),
		},
		{
			name: "should_initialize_job_service_without_ai_when_no_api_key",
			cfg: &config.Settings{
				IsCloudMode: false,
				TokenSecret: "test-secret",
			},
			cache: cache.NewNoOpCache(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, _, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			service := SetupService(db, tt.cfg, tt.cache)

			assert.NotNil(t, service)
		})
	}
}

func TestSetupJobRepository(t *testing.T) {
	tests := []struct {
		name  string
		cache cache.Cache
	}{
		{
			name:  "should_initialize_job_repository_with_cache",
			cache: cache.NewNoOpCache(),
		},
		{
			name:  "should_initialize_job_repository_with_nil_cache",
			cache: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, _, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			repo := SetupJobRepository(db, tt.cache)

			assert.NotNil(t, repo)
		})
	}
}

func TestSetupAIService(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *config.Settings
		expectError bool
	}{
		{
			name: "should_initialize_ai_service_when_api_key_provided",
			cfg: &config.Settings{
				GeminiAPIKey: "test-api-key",
				AIProvider:   "gemini",
			},
			expectError: false,
		},
		{
			name: "should_return_error_when_no_api_key",
			cfg: &config.Settings{
				AIProvider: "gemini",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aiService, err := SetupAIService(tt.cfg)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, aiService)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, aiService)
			}
		})
	}
}

func TestSetupSettingsService(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Settings
	}{
		{
			name: "should_initialize_settings_service",
			cfg: &config.Settings{
				TokenSecret: "test-secret",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, _, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			service := SetupSettingsService(db, tt.cfg)

			assert.NotNil(t, service)
			assert.Equal(t, tt.cfg, service.GetConfig())
		})
	}
}

func TestSetupJobService(t *testing.T) {
	tests := []struct {
		name             string
		cfg              *config.Settings
		withAI           bool
		withQuotaService bool
	}{
		{
			name: "should_initialize_job_service_with_all_dependencies",
			cfg: &config.Settings{
				IsCloudMode: false,
			},
			withAI:           true,
			withQuotaService: true,
		},
		{
			name: "should_initialize_job_service_without_ai",
			cfg: &config.Settings{
				IsCloudMode: false,
			},
			withAI:           false,
			withQuotaService: true,
		},
		{
			name: "should_initialize_job_service_in_cloud_mode",
			cfg: &config.Settings{
				IsCloudMode: true,
			},
			withAI:           true,
			withQuotaService: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, _, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			cache := cache.NewNoOpCache()
			repo := SetupJobRepository(db, cache)

			var aiService *ai.AIService
			if tt.withAI {
				aiService = nil
			}

			settingsService := SetupSettingsService(db, tt.cfg)

			var quotaService *quota.Service
			if tt.withQuotaService {
				quotaAdapter := quota.NewJobRepositoryAdapter(repo)
				quotaService = quota.NewService(db, quotaAdapter, tt.cfg.IsCloudMode)
			}

			service := SetupJobService(repo, aiService, settingsService, quotaService, tt.cfg)

			assert.NotNil(t, service)
		})
	}
}
