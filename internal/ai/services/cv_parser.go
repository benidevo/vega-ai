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
	"github.com/benidevo/vega/internal/common/logger"
)

// CVParserService provides services for parsing CV/resume content using a language model provider.
type CVParserService struct {
	model  llm.Provider
	log    *logger.PrivacyLogger
	helper *helpers.ServiceHelper
}

// NewCVParserService creates and returns a new instance of CVParserService
// using the provided llm.Provider as the model.
func NewCVParserService(model llm.Provider) *CVParserService {
	log := logger.GetPrivacyLogger("ai_cv_parser")
	return &CVParserService{
		model:  model,
		log:    log,
		helper: helpers.NewServiceHelper(log),
	}
}

// ParseCV analyzes CV text and extracts structured information
func (c *CVParserService) ParseCV(ctx context.Context, cvText string) (*models.CVParsingResult, error) {
	start := time.Now()

	c.helper.LogOperationStart("cv_parsing", "anonymous")

	if strings.TrimSpace(cvText) == "" {
		return nil, c.helper.LogValidationError("cv_parsing", "anonymous",
			models.WrapError(models.ErrValidationFailed, fmt.Errorf("CV text cannot be empty")))
	}

	prompt := models.NewCVParsingPrompt(cvText)

	request := llm.GenerateRequest{
		Prompt:       *prompt,
		ResponseType: llm.ResponseTypeCVParsing,
	}

	response, err := c.model.Generate(ctx, request)
	if err != nil {
		return nil, c.helper.LogOperationError("cv_parsing", "anonymous", constants.ErrorTypeAIAnalysisFailed, time.Since(start), err)
	}

	result, ok := response.Data.(models.CVParsingResult)
	if !ok {
		parseErr := fmt.Errorf("unexpected response type: %T", response.Data)
		return nil, c.helper.LogOperationError("cv_parsing", "anonymous", constants.ErrorTypeResponseParseFailed, time.Since(start), parseErr)
	}

	metadata := c.helper.CreateOperationMetadata(0.1, false, map[string]interface{}{
		"method":    "openai_cv_parsing",
		"model":     response.Metadata["model"],
		"task_type": response.Metadata["task_type"],
	})

	c.helper.LogOperationSuccess("cv_parsing", "anonymous", time.Since(start), false, metadata)

	return &result, nil
}
