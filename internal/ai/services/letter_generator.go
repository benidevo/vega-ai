package services

import (
	"context"
	"fmt"
	"time"

	"github.com/benidevo/vega/internal/ai/constants"
	"github.com/benidevo/vega/internal/ai/helpers"
	"github.com/benidevo/vega/internal/ai/llm"
	"github.com/benidevo/vega/internal/ai/models"
	"github.com/benidevo/vega/internal/ai/validation"
	"github.com/benidevo/vega/internal/common/logger"
)

// CoverLetterService provides methods to generate cover letters using a specified LLM provider.
type CoverLetterGeneratorService struct {
	model     llm.Provider
	log       *logger.PrivacyLogger
	validator *validation.AIRequestValidator
	helper    *helpers.ServiceHelper
}

// NewCoverLetterGeneratorService creates and returns a new instance of CoverLetterGeneratorService
// using the provided llm.Provider as the underlying model.
func NewCoverLetterGeneratorService(model llm.Provider) *CoverLetterGeneratorService {
	log := logger.GetPrivacyLogger("ai_cover_letter")
	return &CoverLetterGeneratorService{
		model:     model,
		log:       log,
		validator: validation.NewAIRequestValidator(),
		helper:    helpers.NewServiceHelper(log),
	}
}

// GenerateCoverLetter generates a cover letter based on the provided request.
func (c *CoverLetterGeneratorService) GenerateCoverLetter(ctx context.Context, req models.Request) (*models.CoverLetter, error) {
	start := time.Now()

	c.helper.LogOperationStart(constants.OperationCoverLetter, req.ApplicantName)

	if err := c.validator.ValidateRequest(req); err != nil {
		return nil, c.helper.LogValidationError(constants.OperationCoverLetter, req.ApplicantName, err)
	}

	// Use enhanced prompting by default
	prompt := models.NewPrompt(
		"You are a professional career advisor and expert cover letter writer.",
		req,
		true,
	)

	// Add timeout to prevent indefinite waiting on AI operations
	// Cover letter generation should complete within 30 seconds
	aiCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	response, err := c.model.Generate(aiCtx, llm.GenerateRequest{
		Prompt:       *prompt,
		ResponseType: llm.ResponseTypeCoverLetter,
	})
	if err != nil {
		return nil, c.helper.LogOperationError(constants.OperationCoverLetter, req.ApplicantName, constants.ErrorTypeAIGenerationFailed, time.Since(start), err)
	}

	result, ok := response.Data.(models.CoverLetter)
	if !ok {
		err := fmt.Errorf("unexpected response type: expected CoverLetter, got %T", response.Data)
		return nil, c.helper.LogOperationError(constants.OperationCoverLetter, req.ApplicantName, constants.ErrorTypeResponseParseFailed, time.Since(start), err)
	}

	if err := c.validateCoverLetter(&result); err != nil {
		return nil, c.helper.LogOperationError(constants.OperationCoverLetter, req.ApplicantName, constants.ErrorTypeValidationFailed, time.Since(start), err)
	}

	metadata := c.helper.CreateOperationMetadata(prompt.GetOptimalTemperature("cover_letter"), prompt.UseEnhancedTemplates, map[string]interface{}{
		"content_length": len(result.Content),
		"format":         string(result.Format),
	})

	c.helper.LogOperationSuccess(constants.OperationCoverLetter, req.ApplicantName, time.Since(start), prompt.UseEnhancedTemplates, metadata)

	return &result, nil
}

func (c *CoverLetterGeneratorService) validateCoverLetter(letter *models.CoverLetter) error {
	if letter.Content == "" {
		return models.WrapError(models.ErrValidationFailed, fmt.Errorf("generated cover letter is empty"))
	}

	if letter.Format == "" {
		letter.Format = models.CoverLetterTypePlainText
	}

	return nil
}
