package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/benidevo/vega/internal/ai/constants"
	"github.com/benidevo/vega/internal/ai/helpers"
	"github.com/benidevo/vega/internal/ai/llm"
	"github.com/benidevo/vega/internal/ai/models"
	"github.com/benidevo/vega/internal/ai/validation"
	"github.com/benidevo/vega/internal/common/logger"
)

// CVGeneratorService provides methods to generate CVs using a specified LLM provider.
type CVGeneratorService struct {
	model     llm.Provider
	log       *logger.PrivacyLogger
	validator *validation.AIRequestValidator
	helper    *helpers.ServiceHelper
	timeout   time.Duration
}

// NewCVGeneratorService creates and returns a new instance of CVGeneratorService
// using the provided llm.Provider as the underlying model.
func NewCVGeneratorService(model llm.Provider, timeout time.Duration) *CVGeneratorService {
	log := logger.GetPrivacyLogger("ai_cv_generator")
	return &CVGeneratorService{
		model:     model,
		log:       log,
		validator: validation.NewAIRequestValidator(),
		helper:    helpers.NewServiceHelper(log),
		timeout:   timeout,
	}
}

// GenerateCV generates a CV based on the provided request.
func (c *CVGeneratorService) GenerateCV(ctx context.Context, req models.Request, jobID int, jobTitle string) (*models.GeneratedCV, error) {
	start := time.Now()

	c.helper.LogOperationStart("cv_generation", req.ApplicantName)

	if err := c.validator.ValidateRequest(req); err != nil {
		return nil, c.helper.LogValidationError("cv_generation", req.ApplicantName, err)
	}

	// Use basic prompting for CV generation to reduce processing time
	// Enhanced templates are too complex for the large JSON output required
	prompt := models.NewPrompt(
		"You are a professional career advisor and expert CV writer.",
		req,
		false,
	)

	optimalTemp := prompt.GetOptimalTemperature(string(models.TaskTypeCVGeneration))
	prompt.SetTemperature(optimalTemp)

	aiCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	response, err := c.model.Generate(aiCtx, llm.GenerateRequest{
		Prompt:       *prompt,
		ResponseType: llm.ResponseTypeCV,
	})
	if err != nil {
		return nil, c.helper.LogOperationError("cv_generation", req.ApplicantName, constants.ErrorTypeAIGenerationFailed, time.Since(start), err)
	}

	result, ok := response.Data.(models.CVParsingResult)
	if !ok {
		err := fmt.Errorf("unexpected response type: expected CVParsingResult, got %T", response.Data)
		return nil, c.helper.LogOperationError("cv_generation", req.ApplicantName, constants.ErrorTypeResponseParseFailed, time.Since(start), err)
	}

	// Fallback: if the model omitted the name, populate it from the known applicant name.
	if result.PersonalInfo.FirstName == "" && result.PersonalInfo.LastName == "" && req.ApplicantName != "" {
		parts := strings.SplitN(strings.TrimSpace(req.ApplicantName), " ", 2)
		result.PersonalInfo.FirstName = parts[0]
		if len(parts) == 2 {
			result.PersonalInfo.LastName = parts[1]
		}
	}

	c.log.Debug().
		Str("applicant", req.ApplicantName).
		Bool("is_valid", result.IsValid).
		Str("first_name", result.PersonalInfo.FirstName).
		Str("last_name", result.PersonalInfo.LastName).
		Str("email", result.PersonalInfo.Email).
		Str("phone", result.PersonalInfo.Phone).
		Int("work_exp_count", len(result.WorkExperience)).
		Int("edu_count", len(result.Education)).
		Int("skills_count", len(result.Skills)).
		Msg("Generated CV result before validation")

	if err := c.validateGeneratedCV(&result); err != nil {
		return nil, c.helper.LogOperationError("cv_generation", req.ApplicantName, constants.ErrorTypeValidationFailed, time.Since(start), err)
	}

	generatedCV := &models.GeneratedCV{
		CVParsingResult: result,
		GeneratedAt:     time.Now().Unix(),
		JobID:           jobID,
		JobTitle:        jobTitle,
	}

	metadata := c.helper.CreateOperationMetadata(optimalTemp, prompt.UseEnhancedTemplates, map[string]interface{}{
		"job_id":                jobID,
		"job_title":             jobTitle,
		"work_experience_count": len(result.WorkExperience),
		"education_count":       len(result.Education),
		"skills_count":          len(result.Skills),
	})

	c.helper.LogOperationSuccess("cv_generation", req.ApplicantName, time.Since(start), prompt.UseEnhancedTemplates, metadata)

	return generatedCV, nil
}

func (c *CVGeneratorService) validateGeneratedCV(cv *models.CVParsingResult) error {
	if !cv.IsValid {
		return models.WrapError(models.ErrValidationFailed, fmt.Errorf("generated CV is not valid: %s", cv.Reason))
	}

	if cv.PersonalInfo.FirstName == "" && cv.PersonalInfo.LastName == "" {
		return models.WrapError(models.ErrValidationFailed, fmt.Errorf("generated CV missing required personal information"))
	}

	if len(cv.WorkExperience) == 0 && len(cv.Education) == 0 {
		return models.WrapError(models.ErrValidationFailed, fmt.Errorf("generated CV must have at least work experience or education"))
	}

	return nil
}
