package ai

import (
	"context"
	"testing"

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
			name: "successful Gemini setup",
			config: &config.Settings{
				AIProvider:   ProviderGemini,
				GeminiAPIKey: "valid-api-key",
				GeminiModel:  "gemini-2.5-flash",
			},
			expectError: false,
		},
		{
			name: "missing Gemini API key",
			config: &config.Settings{
				AIProvider:   ProviderGemini,
				GeminiAPIKey: "",
				GeminiModel:  "gemini-2.5-flash",
			},
			expectError: true,
			errorType:   models.ErrMissingAPIKey,
		},
		{
			name: "unsupported provider",
			config: &config.Settings{
				AIProvider: "openai",
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
			name: "valid Gemini provider",
			config: &config.Settings{
				AIProvider:   ProviderGemini,
				GeminiAPIKey: "test-key",
				GeminiModel:  "gemini-2.5-flash",
			},
			expectError: false,
		},
		{
			name: "Gemini missing API key",
			config: &config.Settings{
				AIProvider:   ProviderGemini,
				GeminiAPIKey: "",
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

		service := NewAIService(mockProvider)

		require.NotNil(t, service)
		assert.NotNil(t, service.JobMatcher)
		assert.NotNil(t, service.CoverLetterGenerator)
	})
}

type MockProvider struct{}

func (m *MockProvider) Generate(ctx context.Context, req llm.GenerateRequest) (llm.GenerateResponse, error) {
	return llm.GenerateResponse{}, nil
}
