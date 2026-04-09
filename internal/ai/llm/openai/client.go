package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	openaisdk "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"

	"github.com/benidevo/vega/internal/ai/llm"
	"github.com/benidevo/vega/internal/ai/models"
)

// Client implements llm.Provider using the OpenAI-compatible chat completions API.
// It works with OpenAI, Ollama, LM Studio, vLLM, and any OpenAI-spec-compatible provider.
type Client struct {
	client *openaisdk.Client
	cfg    *Config

	cache        *ResponseCache
	deduplicator *RequestDeduplicator
}

// New creates an OpenAI-compatible client from cfg.
// When cfg.BaseURL is empty the official OpenAI endpoint is used.
func New(cfg *Config) (*Client, error) {
	opts := []option.RequestOption{
		option.WithAPIKey(cfg.APIKey),
	}

	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}

	client := openaisdk.NewClient(opts...)

	return &Client{
		client:       &client,
		cfg:          cfg,
		cache:        NewResponseCache(cfg.CacheMaxEntries, cfg.CacheTTL),
		deduplicator: NewRequestDeduplicator(),
	}, nil
}

// Generate implements llm.Provider.
func (c *Client) Generate(ctx context.Context, request llm.GenerateRequest) (llm.GenerateResponse, error) {
	start := time.Now()

	if ShouldCache(request.ResponseType) {
		if cached, found := c.cache.Get(request); found {
			newMeta := make(map[string]interface{})
			for k, v := range cached.Metadata {
				newMeta[k] = v
			}
			newMeta["cache_hit"] = true
			newMeta["original_duration"] = cached.Duration

			return llm.GenerateResponse{
				Data:     cached.Data,
				Tokens:   cached.Tokens,
				Duration: time.Since(start),
				Metadata: newMeta,
			}, nil
		}
	}

	cacheKey := c.cache.generateCacheKey(request)

	response, err := c.deduplicator.Do(ctx, cacheKey, func() (llm.GenerateResponse, error) {
		switch request.ResponseType {
		case llm.ResponseTypeCoverLetter:
			return c.generateCoverLetter(ctx, request.Prompt, start)
		case llm.ResponseTypeMatchResult:
			return c.generateMatchResult(ctx, request.Prompt, start)
		case llm.ResponseTypeCVParsing:
			return c.parseCVContent(ctx, request.Prompt, start)
		case llm.ResponseTypeCV:
			return c.generateCV(ctx, request.Prompt, start)
		default:
			return llm.GenerateResponse{}, fmt.Errorf("unsupported response type: %s", request.ResponseType)
		}
	})

	if err != nil {
		return llm.GenerateResponse{}, err
	}

	if ShouldCache(request.ResponseType) {
		c.cache.Set(request, response)
	}

	return response, nil
}

// callChat sends a chat completion request and returns the raw response text.
func (c *Client) callChat(ctx context.Context, systemPrompt, userPrompt string, temperature float32, model string) (string, int, error) {
	result, tokens, err := c.executeWithRetry(ctx, func() (string, int, error) {
		rfParam := shared.NewResponseFormatJSONObjectParam()
		params := openaisdk.ChatCompletionNewParams{
			Model: openaisdk.ChatModel(model),
			Messages: []openaisdk.ChatCompletionMessageParamUnion{
				openaisdk.SystemMessage(systemPrompt),
				openaisdk.UserMessage(userPrompt),
			},
			Temperature: openaisdk.Float(float64(temperature)),
			MaxTokens:   openaisdk.Int(int64(c.cfg.MaxOutputTokens)),
			ResponseFormat: openaisdk.ChatCompletionNewParamsResponseFormatUnion{
				OfJSONObject: &rfParam,
			},
		}

		if c.cfg.TopP > 0 {
			params.TopP = openaisdk.Float(float64(c.cfg.TopP))
		}

		resp, err := c.client.Chat.Completions.New(ctx, params)
		if err != nil {
			return "", 0, fmt.Errorf("chat completion error: %w", err)
		}

		if len(resp.Choices) == 0 {
			return "", 0, fmt.Errorf("no choices in response")
		}

		tokens := int(resp.Usage.TotalTokens)
		return resp.Choices[0].Message.Content, tokens, nil
	})

	return result, tokens, err
}

func (c *Client) generateCoverLetter(ctx context.Context, prompt models.Prompt, start time.Time) (llm.GenerateResponse, error) {
	temperature := prompt.GetOptimalTemperature(models.TaskTypeCoverLetter.String())
	model := c.cfg.GetModelForTask(models.TaskTypeCoverLetter.String())

	systemPrompt := c.cfg.SystemInstruction + `

Respond ONLY with a JSON object in this exact format:
{"content": "the complete cover letter text here"}`

	userPrompt := prompt.ToCoverLetterPrompt(c.cfg.DefaultWordRange)

	raw, tokens, err := c.callChat(ctx, systemPrompt, userPrompt, temperature, model)
	if err != nil {
		return llm.GenerateResponse{}, WrapError(ErrCoverLetterGenFailed, err)
	}

	coverLetter, err := c.parseCoverLetterJSON(raw)
	if err != nil {
		return llm.GenerateResponse{}, err
	}

	return llm.GenerateResponse{
		Data:     coverLetter,
		Tokens:   tokens,
		Duration: time.Since(start),
		Metadata: map[string]interface{}{
			"temperature": temperature,
			"enhanced":    prompt.UseEnhancedTemplates,
			"model":       model,
			"task_type":   models.TaskTypeCoverLetter.String(),
		},
	}, nil
}

func (c *Client) generateMatchResult(ctx context.Context, prompt models.Prompt, start time.Time) (llm.GenerateResponse, error) {
	temperature := prompt.GetOptimalTemperature(models.TaskTypeJobAnalysis.String())
	model := c.cfg.GetModelForTask(models.TaskTypeJobAnalysis.String())

	systemPrompt := c.cfg.SystemInstruction + fmt.Sprintf(`

Respond ONLY with a JSON object in this exact format:
{"matchScore": <integer %d-%d>, "strengths": ["..."], "weaknesses": ["..."], "highlights": ["..."], "feedback": "..."}`,
		c.cfg.MinMatchScore, c.cfg.MaxMatchScore)

	userPrompt := prompt.ToMatchAnalysisPrompt(c.cfg.MinMatchScore, c.cfg.MaxMatchScore)

	raw, tokens, err := c.callChat(ctx, systemPrompt, userPrompt, temperature, model)
	if err != nil {
		return llm.GenerateResponse{}, WrapError(ErrMatchAnalysisFailed, err)
	}

	matchResult, err := c.parseMatchResultJSON(raw)
	if err != nil {
		return llm.GenerateResponse{}, err
	}

	return llm.GenerateResponse{
		Data:     matchResult,
		Tokens:   tokens,
		Duration: time.Since(start),
		Metadata: map[string]interface{}{
			"temperature": temperature,
			"enhanced":    prompt.UseEnhancedTemplates,
			"model":       model,
			"task_type":   models.TaskTypeJobAnalysis.String(),
		},
	}, nil
}

func (c *Client) parseCVContent(ctx context.Context, prompt models.Prompt, start time.Time) (llm.GenerateResponse, error) {
	temperature := float32(0.1)
	model := c.cfg.GetModelForTask(models.TaskTypeCVParsing.String())

	systemPrompt := `You are a precise CV/Resume parsing and validation system. First validate that the document is a CV/Resume, then extract structured information.

Always include an "isValid" field. Reject non-career documents (police reports, medical records, etc.).
For valid CVs, be accurate and complete. Do not hallucinate or guess missing information.

Respond ONLY with a JSON object in this exact format:
{
  "isValid": true,
  "reason": "",
  "personalInfo": {"firstName": "", "lastName": "", "email": "", "phone": "", "location": "", "title": ""},
  "workExperience": [{"company": "", "title": "", "location": "", "startDate": "", "endDate": "", "description": ""}],
  "education": [{"institution": "", "degree": "", "fieldOfStudy": "", "startDate": "", "endDate": ""}],
  "certifications": [{"name": "", "issuingOrg": "", "issueDate": "", "expiryDate": "", "credentialId": "", "credentialUrl": ""}],
  "skills": [""]
}
If the document is NOT a valid CV set isValid to false and explain in reason.`

	userPrompt := c.buildCVParsingPrompt(prompt)

	raw, tokens, err := c.callChat(ctx, systemPrompt, userPrompt, temperature, model)
	if err != nil {
		return llm.GenerateResponse{}, WrapError(ErrCVParsingFailed, err)
	}

	cvResult, err := c.parseCVJSON(raw)
	if err != nil {
		return llm.GenerateResponse{}, err
	}

	return llm.GenerateResponse{
		Data:     cvResult,
		Tokens:   tokens,
		Duration: time.Since(start),
		Metadata: map[string]any{
			"temperature": temperature,
			"model":       model,
			"task_type":   models.TaskTypeCVParsing.String(),
			"method":      "openai_cv_parsing",
		},
	}, nil
}

func (c *Client) generateCV(ctx context.Context, prompt models.Prompt, start time.Time) (llm.GenerateResponse, error) {
	temperature := prompt.GetOptimalTemperature(models.TaskTypeCVGeneration.String())
	model := c.cfg.GetModelForTask(models.TaskTypeCVGeneration.String())

	systemPrompt := `You are an expert professional CV/Resume writer. Generate a comprehensive, tailored CV from the provided user profile data and job description.

CRITICAL RULES:
1. Use ONLY information from the USER PROFILE section — never fabricate names, companies, jobs, or education
2. If the user's name is provided, use it exactly; if not, leave blank
3. Transform and enhance the presentation of existing information while maintaining honesty
4. Filter skills to those DIRECTLY RELEVANT to the job posting
5. Always set "isValid" to true

WORK EXPERIENCE FORMATTING:
- Generate 3-5 bullet points per role based on the original description, starting with "• "
- Convert responsibilities into achievement-focused statements
- Quantify impact where possible (e.g., "Increased X by Y%")
- Use "Month Year" date format (e.g., "August 2023") or "Present"

Respond ONLY with a JSON object in this exact format:
{
  "isValid": true,
  "personalInfo": {"firstName": "", "lastName": "", "email": "", "phone": "", "location": "", "title": ""},
  "workExperience": [{"company": "", "title": "", "location": "", "startDate": "", "endDate": "", "description": ""}],
  "education": [{"institution": "", "degree": "", "fieldOfStudy": "", "startDate": "", "endDate": ""}],
  "skills": [""]
}`

	userPrompt := prompt.ToCVGenerationPrompt()

	raw, tokens, err := c.callChat(ctx, systemPrompt, userPrompt, temperature, model)
	if err != nil {
		return llm.GenerateResponse{}, WrapError(ErrCVGenFailed, err)
	}

	cvResult, err := c.parseGeneratedCVJSON(raw)
	if err != nil {
		return llm.GenerateResponse{}, err
	}

	return llm.GenerateResponse{
		Data:     cvResult,
		Tokens:   tokens,
		Duration: time.Since(start),
		Metadata: map[string]any{
			"temperature": temperature,
			"enhanced":    prompt.UseEnhancedTemplates,
			"model":       model,
			"task_type":   models.TaskTypeCVGeneration.String(),
			"method":      "openai_cv_generation",
		},
	}, nil
}

func (c *Client) executeWithRetry(ctx context.Context, operation func() (string, int, error)) (string, int, error) {
	baseDelay := time.Duration(c.cfg.BaseRetryDelay) * time.Second

	var lastErr error
	for attempt := 0; attempt <= c.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseDelay
			maxDelay := time.Duration(c.cfg.MaxRetryDelay) * time.Second
			if delay > maxDelay {
				delay = maxDelay
			}

			select {
			case <-ctx.Done():
				return "", 0, ctx.Err()
			case <-time.After(delay):
			}
		}

		result, tokens, err := operation()
		if err == nil {
			return result, tokens, nil
		}

		lastErr = err
		if !IsRetryableError(err) || attempt == c.cfg.MaxRetries {
			break
		}
	}

	return "", 0, WrapError(ErrMaxRetriesExceeded, lastErr)
}

// extractJSON extracts the outermost JSON object from a response string.
// Uses LastIndex so that lone `}` characters inside string values don't
// truncate the result prematurely.
func (c *Client) extractJSON(response string) string {
	response = strings.TrimSpace(response)

	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start == -1 || end == -1 || end <= start {
		return response
	}

	candidate := response[start : end+1]
	if json.Valid([]byte(candidate)) {
		return candidate
	}

	return response
}

func (c *Client) parseCoverLetterJSON(raw string) (models.CoverLetter, error) {
	clean := c.extractJSON(raw)

	var result models.CoverLetter
	if err := json.Unmarshal([]byte(clean), &result); err != nil {
		return models.CoverLetter{}, WrapError(ErrResponseParseFailed, err)
	}

	if result.Content == "" {
		return models.CoverLetter{}, ErrEmptyResponse
	}

	result.Format = models.CoverLetterTypePlainText
	return result, nil
}

func (c *Client) parseMatchResultJSON(raw string) (models.MatchResult, error) {
	clean := c.extractJSON(raw)

	var result models.MatchResult
	if err := json.Unmarshal([]byte(clean), &result); err != nil {
		return models.MatchResult{}, WrapError(ErrResponseParseFailed, err)
	}

	if result.MatchScore < c.cfg.MinMatchScore || result.MatchScore > c.cfg.MaxMatchScore {
		result.MatchScore = c.cfg.MinMatchScore
	}
	if len(result.Strengths) == 0 {
		result.Strengths = []string{c.cfg.DefaultStrengthsMsg}
	}
	if len(result.Weaknesses) == 0 {
		result.Weaknesses = []string{c.cfg.DefaultWeaknessMsg}
	}
	if len(result.Highlights) == 0 {
		result.Highlights = []string{c.cfg.DefaultHighlightMsg}
	}
	if result.Feedback == "" {
		result.Feedback = c.cfg.DefaultFeedbackMsg
	}

	return result, nil
}

func (c *Client) parseCVJSON(raw string) (models.CVParsingResult, error) {
	clean := c.extractJSON(raw)

	var result models.CVParsingResult
	if err := json.Unmarshal([]byte(clean), &result); err != nil {
		return models.CVParsingResult{}, WrapError(ErrResponseParseFailed, err)
	}

	if !result.IsValid {
		reason := result.Reason
		if reason == "" {
			reason = "Document is not a valid CV/Resume"
		}
		return models.CVParsingResult{}, fmt.Errorf("invalid document: %s", reason)
	}

	if result.PersonalInfo.FirstName == "" && result.PersonalInfo.LastName == "" {
		return models.CVParsingResult{}, fmt.Errorf("no name found in CV")
	}

	if result.WorkExperience == nil {
		result.WorkExperience = []models.WorkExperience{}
	}
	if result.Education == nil {
		result.Education = []models.Education{}
	}
	if result.Skills == nil {
		result.Skills = []string{}
	}

	return result, nil
}

func (c *Client) parseGeneratedCVJSON(raw string) (models.CVParsingResult, error) {
	clean := c.extractJSON(raw)

	var result models.CVParsingResult
	if err := json.Unmarshal([]byte(clean), &result); err != nil {
		return models.CVParsingResult{}, WrapError(ErrResponseParseFailed, err)
	}

	result.IsValid = true

	if result.WorkExperience == nil {
		result.WorkExperience = []models.WorkExperience{}
	}
	if result.Education == nil {
		result.Education = []models.Education{}
	}
	if result.Skills == nil {
		result.Skills = []string{}
	}

	return result, nil
}

func (c *Client) buildCVParsingPrompt(prompt models.Prompt) string {
	return fmt.Sprintf(`Parse the following CV/Resume document and extract structured information.

VALIDATION RULES:
- The document MUST be a CV, Resume, or professional profile containing career-related information
- Reject documents that are: police reports, medical records, legal documents, news articles, fiction, academic papers, manuals, or any non-career documents
- If NOT a valid CV/Resume, return: {"isValid": false, "reason": "explanation"}

PARSING INSTRUCTIONS (only if valid):
- Extract personal information (name, contact details, location, professional title)
- Parse work experience with company, title, dates, and descriptions
- Parse education with institution, degree, field of study, and dates
- Parse certifications with name, issuing organization, issue/expiry dates, and credential details
- Extract skills as a list
- For dates use: "YYYY-MM" for month precision, "YYYY" for year precision, "Present" for current positions
- Use empty strings for missing information — do not guess

Document Text:
%s`, prompt.CVText)
}

// GetCacheStats returns cache performance metrics.
func (c *Client) GetCacheStats() CacheStats {
	return c.cache.GetStats()
}

// GetDeduplicatorStats returns deduplication performance metrics.
func (c *Client) GetDeduplicatorStats() DeduplicatorStats {
	return c.deduplicator.GetStats()
}

// ResetStats clears all performance statistics.
func (c *Client) ResetStats() {
	c.cache.Clear()
	c.deduplicator.Clear()
}
