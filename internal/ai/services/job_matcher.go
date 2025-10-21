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

// JobMatcherService provides services for matching jobs using a language model provider.
// The 'model' field represents the LLM provider used for job matching operations.
type JobMatcherService struct {
	model     llm.Provider
	log       *logger.PrivacyLogger
	validator *validation.AIRequestValidator
	helper    *helpers.ServiceHelper
}

// NewJobMatcherService creates and returns a new instance of JobMatcherService
// using the provided llm.Provider as the model.
// The returned JobMatcherService can be used to perform job matching operations.
func NewJobMatcherService(model llm.Provider) *JobMatcherService {
	log := logger.GetPrivacyLogger("ai_job_matcher")
	return &JobMatcherService{
		model:     model,
		log:       log,
		validator: validation.NewAIRequestValidator(),
		helper:    helpers.NewServiceHelper(log),
	}
}

// AnalyzeMatch analyzes the match between a job applicant and a job.
func (j *JobMatcherService) AnalyzeMatch(ctx context.Context, req models.Request) (*models.MatchResult, error) {
	start := time.Now()

	j.helper.LogOperationStart(constants.OperationMatchAnalysis, req.ApplicantName)

	if err := j.validator.ValidateRequest(req); err != nil {
		return nil, j.helper.LogValidationError(constants.OperationMatchAnalysis, req.ApplicantName, err)
	}

	prompt := models.NewPrompt(
		"Analyze the job match between the candidate and position",
		req,
		true,
	)

	// Add timeout to prevent indefinite waiting on AI operations
	// Job analysis is complex but should complete within 30 seconds
	// If it takes longer, the prompt is likely too complex
	aiCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	response, err := j.model.Generate(aiCtx, llm.GenerateRequest{
		Prompt:       *prompt,
		ResponseType: llm.ResponseTypeMatchResult,
	})
	if err != nil {
		return nil, j.helper.LogOperationError(constants.OperationMatchAnalysis, req.ApplicantName, constants.ErrorTypeAIAnalysisFailed, time.Since(start), err)
	}

	result, ok := response.Data.(models.MatchResult)
	if !ok {
		err := fmt.Errorf("unexpected response type: expected MatchResult, got %T", response.Data)
		return nil, j.helper.LogOperationError(constants.OperationMatchAnalysis, req.ApplicantName, constants.ErrorTypeResponseParseFailed, time.Since(start), err)
	}

	j.validateMatchResult(&result)

	metadata := j.helper.CreateOperationMetadata(prompt.GetOptimalTemperature(models.TaskTypeJobAnalysis.String()), prompt.UseEnhancedTemplates, map[string]interface{}{
		"match_score":      result.MatchScore,
		"strengths_count":  len(result.Strengths),
		"weaknesses_count": len(result.Weaknesses),
	})

	j.helper.LogOperationSuccess(constants.OperationMatchAnalysis, req.ApplicantName, time.Since(start), prompt.UseEnhancedTemplates, metadata)

	return &result, nil
}

func (j *JobMatcherService) validateMatchResult(result *models.MatchResult) {
	if result.MatchScore < 0 || result.MatchScore > 100 {
		result.MatchScore = 0
	}

	if len(result.Strengths) == 0 {
		result.Strengths = []string{"No specific strengths identified"}
	}

	if len(result.Weaknesses) == 0 {
		result.Weaknesses = []string{"No specific weaknesses identified"}
	}

	if len(result.Highlights) == 0 {
		result.Highlights = []string{"No specific highlights identified"}
	}

	if result.Feedback == "" {
		result.Feedback = "Unable to provide detailed feedback at this time."
	}
}

// GetMatchCategories returns the match category and its description based on the provided score.
// It evaluates the score and maps it to a predefined category and description:
//   - 90 and above: Excellent
//   - 80 to 89: Strong
//   - 70 to 79: Good
//   - 60 to 69: Fair
//   - 50 to 59: Partial
//   - Below 50: Poor
func (j *JobMatcherService) GetMatchCategories(score int) (string, string) {
	switch {
	case score >= constants.ScoreThresholdExcellent:
		return constants.MatchCategoryExcellent, constants.MatchDescExcellent
	case score >= constants.ScoreThresholdStrong:
		return constants.MatchCategoryStrong, constants.MatchDescStrong
	case score >= constants.ScoreThresholdGood:
		return constants.MatchCategoryGood, constants.MatchDescGood
	case score >= constants.ScoreThresholdFair:
		return constants.MatchCategoryFair, constants.MatchDescFair
	case score >= constants.ScoreThresholdPartial:
		return constants.MatchCategoryPartial, constants.MatchDescPartial
	default:
		return constants.MatchCategoryPoor, constants.MatchDescPoor
	}
}
