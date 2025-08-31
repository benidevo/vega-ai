package gemini

import (
	"time"

	"github.com/benidevo/vega/internal/ai/models"
	"github.com/benidevo/vega/internal/config"
)

// Config holds configuration for the Gemini LLM client.
type Config struct {
	APIKey           string
	MaxTokens        int
	Model            string
	ModelCVParsing   string
	ModelJobAnalysis string
	ModelCoverLetter string
	Temperature      *float32

	MaxRetries     int
	BaseRetryDelay int
	MaxRetryDelay  int

	ResponseMIMEType string

	DefaultWordRange string

	MinMatchScore int
	MaxMatchScore int

	DefaultStrengthsMsg string
	DefaultWeaknessMsg  string
	DefaultHighlightMsg string
	DefaultFeedbackMsg  string

	MaxOutputTokens   int32
	TopP              *float32
	TopK              *float32
	SystemInstruction string

	CacheMaxEntries int
	CacheTTL        time.Duration

	CircuitBreakerMaxFailures      int
	CircuitBreakerResetTimeout     time.Duration
	CircuitBreakerHalfOpenRequests int
}

// NewConfig creates a new Config from application settings.
func NewConfig(cfg *config.Settings) *Config {
	defaultTemp := float32(0.4)

	return &Config{
		APIKey:           cfg.GeminiAPIKey,
		MaxTokens:        8192,
		Model:            cfg.GeminiModel,
		ModelCVParsing:   cfg.GeminiModelCVParsing,
		ModelJobAnalysis: cfg.GeminiModelJobAnalysis,
		ModelCoverLetter: cfg.GeminiModelCoverLetter,
		Temperature:      &defaultTemp,

		MaxRetries:     3,
		BaseRetryDelay: 1,
		MaxRetryDelay:  30,

		ResponseMIMEType: "application/json",

		DefaultWordRange: "150-250",

		MinMatchScore: 0,
		MaxMatchScore: 100,

		DefaultStrengthsMsg: "No specific strengths identified",
		DefaultWeaknessMsg:  "No specific weaknesses identified",
		DefaultHighlightMsg: "No specific highlights identified",
		DefaultFeedbackMsg:  "Unable to provide detailed feedback at this time.",

		MaxOutputTokens:   6000,
		TopP:              floatPtr(0.9),
		TopK:              floatPtr(40),
		SystemInstruction: "You are a professional career advisor and expert writer. Always provide helpful, accurate, and constructive feedback. IMPORTANT: For job matching, use experience-based evaluation - candidates with 2+ years experience should be evaluated primarily on work history and practical skills, with education as secondary. Entry-level candidates (<2 years) should be evaluated with education carrying more weight. BE MODERATELY LENIENT: Value similar and transferable skills, not just exact matches. Award modest bonuses for related technologies and cross-domain experience. When responding with JSON, output ONLY valid JSON without any preamble, explanation, or additional text. Do not include phrases like 'Here is the JSON' or any other text before or after the JSON object.",

		CacheMaxEntries: 100,
		CacheTTL:        60 * time.Second,

		CircuitBreakerMaxFailures:      5,
		CircuitBreakerResetTimeout:     30 * time.Second,
		CircuitBreakerHalfOpenRequests: 3,
	}
}

// GetModelForTask returns the appropriate model for the given task type.
func (c *Config) GetModelForTask(taskType string) string {
	switch models.AITaskType(taskType) {
	case models.TaskTypeCVParsing:
		if c.ModelCVParsing != "" {
			return c.ModelCVParsing
		}
	case models.TaskTypeJobAnalysis, models.TaskTypeMatchResult:
		if c.ModelJobAnalysis != "" {
			return c.ModelJobAnalysis
		}
	case models.TaskTypeCoverLetter:
		if c.ModelCoverLetter != "" {
			return c.ModelCoverLetter
		}
	case models.TaskTypeCVGeneration:
		if c.ModelCoverLetter != "" {
			return c.ModelCoverLetter
		}
	}

	return c.Model
}

func floatPtr(f float32) *float32 {
	return &f
}
