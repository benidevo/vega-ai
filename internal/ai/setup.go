package ai

import (
	"fmt"

	"github.com/benidevo/vega/internal/ai/llm"
	openaiProvider "github.com/benidevo/vega/internal/ai/llm/openai"
	"github.com/benidevo/vega/internal/ai/models"
	"github.com/benidevo/vega/internal/ai/services"
	"github.com/benidevo/vega/internal/config"
)

const (
	// ProviderGemini uses Gemini via its OpenAI-compatible endpoint.
	ProviderGemini = "gemini"
	// ProviderOpenAI uses OpenAI or any OpenAI-spec-compatible provider (Ollama, vLLM, etc.).
	ProviderOpenAI = "openai"
)

type AIService struct {
	JobMatcher           *services.JobMatcherService
	CoverLetterGenerator *services.CoverLetterGeneratorService
	CVParser             *services.CVParserService
	CVGenerator          *services.CVGeneratorService
}

// Setup initializes the complete AI service with all dependencies.
// It configures the LLM provider and creates all AI services.
func Setup(cfg *config.Settings) (*AIService, error) {
	provider, err := createProvider(cfg)
	if err != nil {
		return nil, models.WrapError(models.ErrProviderInitFailed, err)
	}

	return NewAIService(provider), nil
}

func createProvider(cfg *config.Settings) (llm.Provider, error) {
	switch cfg.AIProvider {
	case ProviderGemini, ProviderOpenAI:
		if cfg.OpenAIAPIKey == "" {
			return nil, models.WrapError(models.ErrMissingAPIKey, fmt.Errorf("OPENAI_API_KEY (or GEMINI_API_KEY) is required"))
		}
		providerCfg := openaiProvider.NewConfig(cfg)
		provider, err := openaiProvider.New(providerCfg)
		if err != nil {
			return nil, models.WrapError(models.ErrProviderInitFailed, err)
		}
		return provider, nil
	default:
		return nil, models.WrapError(models.ErrUnsupportedProvider, fmt.Errorf("provider '%s' is not supported", cfg.AIProvider))
	}
}

// NewAIService initializes the AI service with the provided LLM provider.
func NewAIService(provider llm.Provider) *AIService {
	return &AIService{
		JobMatcher:           services.NewJobMatcherService(provider),
		CoverLetterGenerator: services.NewCoverLetterGeneratorService(provider),
		CVParser:             services.NewCVParserService(provider),
		CVGenerator:          services.NewCVGeneratorService(provider),
	}
}
