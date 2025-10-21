package gemini

import (
	"testing"

	"github.com/benidevo/vega/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestConfig_GetModelForTask(t *testing.T) {
	tests := []struct {
		name          string
		config        *Config
		taskType      string
		expectedModel string
	}{
		{
			name: "CV parsing uses specific model",
			config: &Config{
				Model:          "default-model",
				ModelCVParsing: "gemini-1.5-flash",
			},
			taskType:      "cv_parsing",
			expectedModel: "gemini-1.5-flash",
		},
		{
			name: "job analysis uses specific model",
			config: &Config{
				Model:            "default-model",
				ModelJobAnalysis: "gemini-2.5-flash",
			},
			taskType:      "job_analysis",
			expectedModel: "gemini-2.5-flash",
		},
		{
			name: "cover letter uses specific model",
			config: &Config{
				Model:            "default-model",
				ModelCoverLetter: "gemini-2.0-flash-thinking",
			},
			taskType:      "cover_letter",
			expectedModel: "gemini-2.0-flash-thinking",
		},
		{
			name: "cv generation uses default model when cover letter model available",
			config: &Config{
				Model:            "default-model",
				ModelCoverLetter: "gemini-cv-model",
			},
			taskType:      "cv_generate",
			expectedModel: "default-model",
		},
		{
			name: "cv generation falls back to default model",
			config: &Config{
				Model: "default-model",
			},
			taskType:      "cv_generate",
			expectedModel: "default-model",
		},
		{
			name: "fallback to default when task model empty",
			config: &Config{
				Model:          "default-model",
				ModelCVParsing: "",
			},
			taskType:      "cv_parsing",
			expectedModel: "default-model",
		},
		{
			name: "unknown task uses default",
			config: &Config{
				Model: "default-model",
			},
			taskType:      "unknown_task",
			expectedModel: "default-model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetModelForTask(tt.taskType)
			assert.Equal(t, tt.expectedModel, result)
		})
	}
}

func TestNewConfig(t *testing.T) {
	cfg := &config.Settings{
		GeminiAPIKey:           "test-api-key",
		GeminiModel:            "gemini-2.5-flash",
		GeminiModelCVParsing:   "gemini-2.0-flash-lite",
		GeminiModelJobAnalysis: "gemini-2.5-flash",
		GeminiModelCoverLetter: "gemini-2.0-flash-lite",
	}

	result := NewConfig(cfg)

	assert.Equal(t, "test-api-key", result.APIKey)
	assert.Equal(t, "gemini-2.5-flash", result.Model)
	assert.Equal(t, "gemini-2.0-flash-lite", result.ModelCVParsing)
	assert.Equal(t, "gemini-2.5-flash", result.ModelJobAnalysis)
	assert.Equal(t, "gemini-2.0-flash-lite", result.ModelCoverLetter)
	assert.Equal(t, 3, result.MaxRetries)
	assert.Equal(t, 1, result.BaseRetryDelay)
	assert.Equal(t, 30, result.MaxRetryDelay)
	assert.Equal(t, 0, result.MinMatchScore)
	assert.Equal(t, 100, result.MaxMatchScore)
	assert.Equal(t, "application/json", result.ResponseMIMEType)
	assert.Equal(t, "150-250", result.DefaultWordRange)

	// Check that float pointers are set correctly
	assert.Equal(t, int32(6000), result.MaxOutputTokens)
	assert.NotNil(t, result.TopP)
	assert.Equal(t, float32(0.9), *result.TopP)
	assert.NotNil(t, result.TopK)
	assert.Equal(t, float32(40), *result.TopK)
	assert.NotNil(t, result.Temperature)
	assert.Equal(t, float32(0.4), *result.Temperature)
}

func TestFloatPtr(t *testing.T) {
	value := float32(3.14)
	ptr := floatPtr(value)

	assert.NotNil(t, ptr)
	assert.Equal(t, value, *ptr)
}
