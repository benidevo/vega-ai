package ai

import (
	"context"
	"testing"
	"time"

	"github.com/benidevo/vega/internal/ai/llm"
	"github.com/benidevo/vega/internal/ai/models"
	"github.com/benidevo/vega/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetup(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.Settings
		expectError bool
		errorType   error
	}{
		{
			name: "successful openai setup",
			config: &config.Settings{
				AIProvider:   ProviderOpenAI,
				OpenAIAPIKey: "valid-api-key",
				OpenAIModel:  "gpt-4o-mini",
			},
			expectError: false,
		},
		{
			name: "successful gemini setup via openai-compat",
			config: &config.Settings{
				AIProvider:    ProviderGemini,
				OpenAIAPIKey:  "valid-api-key",
				OpenAIBaseURL: "https://generativelanguage.googleapis.com/v1beta/openai/",
				OpenAIModel:   "gemini-2.5-flash",
			},
			expectError: false,
		},
		{
			name: "missing API key",
			config: &config.Settings{
				AIProvider:   ProviderOpenAI,
				OpenAIAPIKey: "",
			},
			expectError: true,
			errorType:   models.ErrMissingAPIKey,
		},
		{
			name: "unsupported provider",
			config: &config.Settings{
				AIProvider: "anthropic",
			},
			expectError: true,
			errorType:   models.ErrUnsupportedProvider,
		},
		{
			name: "empty provider",
			config: &config.Settings{
				AIProvider: "",
			},
			expectError: true,
			errorType:   models.ErrUnsupportedProvider,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, err := Setup(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, service)
				if tt.errorType != nil {
					assert.Contains(t, err.Error(), tt.errorType.Error())
				}
			} else {
				assert.NoError(t, err)
				require.NotNil(t, service)
				assert.NotNil(t, service.JobMatcher)
				assert.NotNil(t, service.CoverLetterGenerator)
			}
		})
	}
}

func TestCreateProvider(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.Settings
		expectError bool
		errorType   error
	}{
		{
			name: "valid openai provider",
			config: &config.Settings{
				AIProvider:   ProviderOpenAI,
				OpenAIAPIKey: "test-key",
				OpenAIModel:  "gpt-4o-mini",
			},
			expectError: false,
		},
		{
			name: "valid gemini via openai-compat",
			config: &config.Settings{
				AIProvider:   ProviderGemini,
				OpenAIAPIKey: "test-key",
				OpenAIModel:  "gemini-2.5-flash",
			},
			expectError: false,
		},
		{
			name: "missing API key",
			config: &config.Settings{
				AIProvider:   ProviderOpenAI,
				OpenAIAPIKey: "",
			},
			expectError: true,
			errorType:   models.ErrMissingAPIKey,
		},
		{
			name: "unsupported provider type",
			config: &config.Settings{
				AIProvider: "claude",
			},
			expectError: true,
			errorType:   models.ErrUnsupportedProvider,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := createProvider(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, provider)
				if tt.errorType != nil {
					assert.Contains(t, err.Error(), tt.errorType.Error())
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, provider)
			}
		})
	}
}

func TestNewAIService(t *testing.T) {
	t.Run("creates service with all components", func(t *testing.T) {
		mockProvider := &MockProvider{}

		service := NewAIService(mockProvider, 30*time.Second)

		require.NotNil(t, service)
		assert.NotNil(t, service.JobMatcher)
		assert.NotNil(t, service.CoverLetterGenerator)
	})
}

type MockProvider struct{}

func (m *MockProvider) Generate(ctx context.Context, req llm.GenerateRequest) (llm.GenerateResponse, error) {
	return llm.GenerateResponse{}, nil
}
