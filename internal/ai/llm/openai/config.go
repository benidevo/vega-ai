package openai

import (
	"time"

	"github.com/benidevo/vega/internal/ai/models"
	"github.com/benidevo/vega/internal/config"
)

// Config holds configuration for the OpenAI-compatible LLM client.
type Config struct {
	APIKey  string
	BaseURL string // Empty uses the OpenAI default. Set to http://localhost:11434/v1 for Ollama.

	Model            string
	ModelCVParsing   string
	ModelJobAnalysis string
	ModelCoverLetter string

	MaxRetries     int
	BaseRetryDelay int
	MaxRetryDelay  int

	DefaultWordRange string

	MinMatchScore int
	MaxMatchScore int

	DefaultStrengthsMsg string
	DefaultWeaknessMsg  string
	DefaultHighlightMsg string
	DefaultFeedbackMsg  string

	Temperature       float32
	TopP              float32
	SystemInstruction string

	CacheMaxEntries int
	CacheTTL        time.Duration

	OperationTimeout time.Duration
}

// NewConfig creates a Config from application settings.
func NewConfig(cfg *config.Settings) *Config {
	return &Config{
		APIKey:  cfg.OpenAIAPIKey,
		BaseURL: cfg.OpenAIBaseURL,

		Model:            cfg.OpenAIModel,
		ModelCVParsing:   cfg.OpenAIModelCVParsing,
		ModelJobAnalysis: cfg.OpenAIModelJobAnalysis,
		ModelCoverLetter: cfg.OpenAIModelCoverLetter,

		MaxRetries:     3,
		BaseRetryDelay: 1,
		MaxRetryDelay:  30,

		DefaultWordRange: "150-250",

		MinMatchScore: 0,
		MaxMatchScore: 100,

		DefaultStrengthsMsg: "No specific strengths identified",
		DefaultWeaknessMsg:  "No specific weaknesses identified",
		DefaultHighlightMsg: "No specific highlights identified",
		DefaultFeedbackMsg:  "Unable to provide detailed feedback at this time.",

		Temperature: 0.4,
		TopP:        0.9,
		SystemInstruction: "You are a professional career advisor and expert writer. Always provide helpful, accurate, and constructive feedback. " +
			"IMPORTANT: For job matching, use experience-based evaluation - candidates with 2+ years experience should be evaluated primarily on work history and practical skills, with education as secondary. " +
			"Entry-level candidates (<2 years) should be evaluated with education carrying more weight. " +
			"BE MODERATELY LENIENT: Value similar and transferable skills, not just exact matches. Award modest bonuses for related technologies and cross-domain experience. " +
			"When responding with JSON, output ONLY valid JSON without any preamble, explanation, or additional text. " +
			"Do not include phrases like 'Here is the JSON' or any other text before or after the JSON object.",

		CacheMaxEntries: 100,
		CacheTTL:        60 * time.Second,

		OperationTimeout: cfg.AIOperationTimeout,
	}
}

// GetModelForTask returns the appropriate model name for the given task type.
// Falls back to the default model when no task-specific model is configured.
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
	case models.TaskTypeCoverLetter, models.TaskTypeCVGeneration:
		if c.ModelCoverLetter != "" {
			return c.ModelCoverLetter
		}
	}

	return c.Model
}
