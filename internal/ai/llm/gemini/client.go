package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"google.golang.org/genai"

	"github.com/benidevo/vega/internal/ai/llm"
	"github.com/benidevo/vega/internal/ai/models"
)

// Gemini represents a client for interacting with the Gemini AI service.
//
// It holds a reference to the underlying genai.Client and configuration settings.
// Gemini is the client for interacting with Google's Gemini AI service.
type Gemini struct {
	client *genai.Client
	cfg    *Config

	cache        *ResponseCache
	deduplicator *RequestDeduplicator
}

// New creates and initializes a new Gemini client using the provided context and configuration.
// It returns a pointer to the Gemini instance or an error if the client initialization fails.
// New creates a new Gemini client with the provided configuration.
func New(ctx context.Context, cfg *Config) (*Gemini, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  cfg.APIKey,
		Backend: genai.BackendGeminiAPI})

	if err != nil {
		return nil, WrapError(ErrClientInitFailed, err)
	}

	cache := NewResponseCache(cfg.CacheMaxEntries, cfg.CacheTTL)
	deduplicator := NewRequestDeduplicator()

	return &Gemini{
		client:       client,
		cfg:          cfg,
		cache:        cache,
		deduplicator: deduplicator,
	}, nil
}

// Generate implements the Provider interface for the Gemini client.
// It processes requests based on the ResponseType and returns appropriate data.
// Generate processes AI requests and returns responses based on the request type.
func (g *Gemini) Generate(ctx context.Context, request llm.GenerateRequest) (llm.GenerateResponse, error) {
	start := time.Now()

	if ShouldCache(request.ResponseType) {
		if cached, found := g.cache.Get(request); found {
			// Create a new metadata map to avoid modifying the cached entry
			newMetadata := make(map[string]interface{})
			for k, v := range cached.Metadata {
				newMetadata[k] = v
			}
			newMetadata["cache_hit"] = true
			newMetadata["original_duration"] = cached.Duration

			// Return a response with new metadata and updated duration
			return llm.GenerateResponse{
				Data:     cached.Data,
				Tokens:   cached.Tokens,
				Duration: time.Since(start),
				Metadata: newMetadata,
			}, nil
		}
	}

	cacheKey := g.cache.generateCacheKey(request)

	response, err := g.deduplicator.Do(ctx, cacheKey, func() (llm.GenerateResponse, error) {
		var resp llm.GenerateResponse
		var genErr error

		switch request.ResponseType {
		case llm.ResponseTypeCoverLetter:
			resp, genErr = g.generateCoverLetter(ctx, request.Prompt, start)
		case llm.ResponseTypeMatchResult:
			resp, genErr = g.generateMatchResult(ctx, request.Prompt, start)
		case llm.ResponseTypeCVParsing:
			resp, genErr = g.parseCVContent(ctx, request.Prompt, start)
		case llm.ResponseTypeCV:
			resp, genErr = g.generateCV(ctx, request.Prompt, start)
		default:
			genErr = fmt.Errorf("unsupported response type: %s", request.ResponseType)
		}

		if genErr != nil {
			return llm.GenerateResponse{}, genErr
		}

		return resp, nil
	})

	if err != nil {
		return llm.GenerateResponse{}, err
	}

	if ShouldCache(request.ResponseType) && err == nil {
		g.cache.Set(request, response)
	}

	return response, nil
}

// generateCoverLetter generates a cover letter based on the provided prompt.
func (g *Gemini) generateCoverLetter(ctx context.Context, prompt models.Prompt, start time.Time) (llm.GenerateResponse, error) {
	coverLetterPrompt := prompt.ToCoverLetterPrompt(g.cfg.DefaultWordRange)

	temperature := prompt.GetOptimalTemperature(models.TaskTypeCoverLetter.String())

	result, err := g.executeWithRetry(ctx, func() (string, error) {
		model := g.cfg.GetModelForTask(models.TaskTypeCoverLetter.String())
		resp, err := g.client.Models.GenerateContent(ctx, model, genai.Text(coverLetterPrompt), &genai.GenerateContentConfig{
			Temperature:       &temperature,
			ResponseMIMEType:  g.cfg.ResponseMIMEType,
			ResponseSchema:    g.getCoverLetterSchema(),
			MaxOutputTokens:   g.cfg.MaxOutputTokens,
			TopP:              g.cfg.TopP,
			TopK:              g.cfg.TopK,
			SystemInstruction: g.buildSystemInstruction(),
		})
		if err != nil {
			return "", fmt.Errorf("generate content error: %w", err)
		}

		if len(resp.Candidates) == 0 {
			return "", fmt.Errorf("no candidates in response")
		}

		candidate := resp.Candidates[0]
		if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
			return "", fmt.Errorf("no content in response candidate")
		}

		var responseText string
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				responseText += part.Text
			}
		}

		return responseText, nil
	})

	if err != nil {
		return llm.GenerateResponse{}, WrapError(ErrCoverLetterGenFailed, err)
	}

	coverLetter, err := g.parseCoverLetterJSON(result)
	if err != nil {
		return llm.GenerateResponse{}, err
	}

	return llm.GenerateResponse{
		Data:     coverLetter,
		Duration: time.Since(start),
		Tokens:   0,
		Metadata: map[string]interface{}{
			"temperature": temperature,
			"enhanced":    prompt.UseEnhancedTemplates,
			"model":       g.cfg.GetModelForTask(models.TaskTypeCoverLetter.String()),
			"task_type":   models.TaskTypeCoverLetter.String(),
		},
	}, nil
}

// generateMatchResult analyzes the given prompt using the Gemini model and returns a match result.
func (g *Gemini) generateMatchResult(ctx context.Context, prompt models.Prompt, start time.Time) (llm.GenerateResponse, error) {
	matchPrompt := prompt.ToMatchAnalysisPrompt(g.cfg.MinMatchScore, g.cfg.MaxMatchScore)

	temperature := prompt.GetOptimalTemperature(models.TaskTypeJobAnalysis.String())

	result, err := g.executeWithRetry(ctx, func() (string, error) {
		model := g.cfg.GetModelForTask(models.TaskTypeJobAnalysis.String())
		resp, err := g.client.Models.GenerateContent(ctx, model, genai.Text(matchPrompt), &genai.GenerateContentConfig{
			Temperature:       &temperature,
			ResponseMIMEType:  g.cfg.ResponseMIMEType,
			ResponseSchema:    g.getMatchAnalysisSchema(),
			MaxOutputTokens:   g.cfg.MaxOutputTokens,
			TopP:              g.cfg.TopP,
			TopK:              g.cfg.TopK,
			SystemInstruction: g.buildSystemInstruction(),
		})
		if err != nil {
			return "", fmt.Errorf("generate content error: %w", err)
		}

		if len(resp.Candidates) == 0 {
			return "", fmt.Errorf("no candidates in response")
		}

		candidate := resp.Candidates[0]
		if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
			return "", fmt.Errorf("no content in response candidate")
		}

		var responseText string
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				responseText += part.Text
			}
		}

		return responseText, nil
	})

	if err != nil {
		return llm.GenerateResponse{}, WrapError(ErrMatchAnalysisFailed, err)
	}

	matchResult, err := g.parseMatchResultJSON(result)
	if err != nil {
		return llm.GenerateResponse{}, err
	}

	return llm.GenerateResponse{
		Data:     matchResult,
		Duration: time.Since(start),
		Tokens:   0,
		Metadata: map[string]interface{}{
			"temperature": temperature,
			"enhanced":    prompt.UseEnhancedTemplates,
			"model":       g.cfg.GetModelForTask(models.TaskTypeJobAnalysis.String()),
			"task_type":   models.TaskTypeJobAnalysis.String(),
		},
	}, nil
}

// parseCVContent parses CV text and extracts structured information
func (g *Gemini) parseCVContent(ctx context.Context, prompt models.Prompt, start time.Time) (llm.GenerateResponse, error) {
	cvPrompt := g.buildCVParsingPrompt(prompt)

	temperature := float32(0.1) // low temperature for consistent parsing

	result, err := g.executeWithRetry(ctx, func() (string, error) {
		model := g.cfg.GetModelForTask(models.TaskTypeCVParsing.String())
		resp, err := g.client.Models.GenerateContent(ctx, model, genai.Text(cvPrompt), &genai.GenerateContentConfig{
			Temperature:       &temperature,
			ResponseMIMEType:  g.cfg.ResponseMIMEType,
			ResponseSchema:    g.getCVParsingSchema(),
			MaxOutputTokens:   g.cfg.MaxOutputTokens,
			TopP:              g.cfg.TopP,
			TopK:              g.cfg.TopK,
			SystemInstruction: g.buildCVParsingSystemInstruction(),
		})
		if err != nil {
			return "", fmt.Errorf("generate content error: %w", err)
		}

		if len(resp.Candidates) == 0 {
			return "", fmt.Errorf("no candidates in response")
		}

		candidate := resp.Candidates[0]
		if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
			return "", fmt.Errorf("no content in response candidate")
		}

		var responseText string
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				responseText += part.Text
			}
		}

		return responseText, nil
	})

	if err != nil {
		return llm.GenerateResponse{}, WrapError(ErrCoverLetterGenFailed, err)
	}

	cvResult, err := g.parseCVJSON(result)
	if err != nil {
		return llm.GenerateResponse{}, err
	}

	return llm.GenerateResponse{
		Data:     cvResult,
		Duration: time.Since(start),
		Tokens:   0,
		Metadata: map[string]any{
			"temperature": temperature,
			"model":       g.cfg.GetModelForTask(models.TaskTypeCVParsing.String()),
			"task_type":   models.TaskTypeCVParsing.String(),
			"method":      "gemini_cv_parsing",
		},
	}, nil
}

func (g *Gemini) executeWithRetry(ctx context.Context, operation func() (string, error)) (string, error) {
	maxRetries := g.cfg.MaxRetries
	baseDelay := time.Duration(g.cfg.BaseRetryDelay) * time.Second

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseDelay
			maxDelay := time.Duration(g.cfg.MaxRetryDelay) * time.Second
			if delay > maxDelay {
				delay = maxDelay
			}

			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(delay):
			}
		}

		result, err := operation()
		if err == nil {
			return result, nil
		}

		lastErr = err
		if !IsRetryableError(err) || attempt == maxRetries {
			break
		}
	}

	if lastErr != nil {
		return "", WrapError(ErrMaxRetriesExceeded, lastErr)
	}
	return "", lastErr
}

func (g *Gemini) getMatchAnalysisSchema() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"matchScore": {
				Type:        genai.TypeInteger,
				Description: fmt.Sprintf("Match score from %d-%d", g.cfg.MinMatchScore, g.cfg.MaxMatchScore),
			},
			"strengths": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeString,
				},
				Description: "List of candidate strengths",
			},
			"weaknesses": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeString,
				},
				Description: "List of areas for improvement",
			},
			"highlights": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeString,
				},
				Description: "List of standout qualifications",
			},
			"feedback": {
				Type:        genai.TypeString,
				Description: "Overall assessment and recommendations",
			},
		},
		PropertyOrdering: []string{"matchScore", "strengths", "weaknesses", "highlights", "feedback"},
		Required:         []string{"matchScore", "strengths", "weaknesses", "highlights", "feedback"},
	}
}

func (g *Gemini) parseMatchResultJSON(jsonResponse string) (models.MatchResult, error) {
	cleanJSON := g.extractJSON(jsonResponse)

	var result models.MatchResult
	if err := json.Unmarshal([]byte(cleanJSON), &result); err != nil {
		return models.MatchResult{}, WrapError(ErrResponseParseFailed, err)
	}

	if result.MatchScore < g.cfg.MinMatchScore || result.MatchScore > g.cfg.MaxMatchScore {
		result.MatchScore = g.cfg.MinMatchScore
	}

	if len(result.Strengths) == 0 {
		result.Strengths = []string{g.cfg.DefaultStrengthsMsg}
	}

	if len(result.Weaknesses) == 0 {
		result.Weaknesses = []string{g.cfg.DefaultWeaknessMsg}
	}

	if len(result.Highlights) == 0 {
		result.Highlights = []string{g.cfg.DefaultHighlightMsg}
	}

	if result.Feedback == "" {
		result.Feedback = g.cfg.DefaultFeedbackMsg
	}

	return result, nil
}

func (g *Gemini) getCoverLetterSchema() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"content": {
				Type:        genai.TypeString,
				Description: "The complete cover letter content",
			},
		},
		PropertyOrdering: []string{"content"},
		Required:         []string{"content"},
	}
}

func (g *Gemini) parseCoverLetterJSON(jsonResponse string) (models.CoverLetter, error) {
	cleanJSON := g.extractJSON(jsonResponse)

	var result models.CoverLetter
	if err := json.Unmarshal([]byte(cleanJSON), &result); err != nil {
		return models.CoverLetter{}, WrapError(ErrResponseParseFailed, err)
	}

	if result.Content == "" {
		return models.CoverLetter{}, ErrEmptyResponse
	}

	result.Format = models.CoverLetterTypePlainText

	return result, nil
}

func (g *Gemini) buildSystemInstruction() *genai.Content {
	if g.cfg.SystemInstruction == "" {
		return nil
	}

	contents := genai.Text(g.cfg.SystemInstruction)
	if len(contents) > 0 {
		return contents[0]
	}

	return nil
}

// extractJSON attempts to extract JSON content from a response that may contain extra text
func (g *Gemini) extractJSON(response string) string {
	response = strings.TrimSpace(response)

	// Look for JSON object boundaries
	startIdx := strings.Index(response, "{")
	if startIdx == -1 {
		return response // No JSON found, return as is
	}

	// Find the matching closing brace
	braceCount := 0
	endIdx := -1

	for i := startIdx; i < len(response) && endIdx == -1; i++ {
		switch response[i] {
		case '{':
			braceCount++
		case '}':
			braceCount--
			if braceCount == 0 {
				endIdx = i
			}
		}
	}

	if endIdx == -1 {
		return response // No matching brace found, return as is
	}

	return response[startIdx : endIdx+1]
}

func (g *Gemini) buildCVParsingPrompt(prompt models.Prompt) string {
	cvText := prompt.CVText

	return fmt.Sprintf(`You are an expert CV/Resume parser and validator. First, determine if the provided text is actually a CV/Resume document. Then extract structured information if valid.

VALIDATION RULES:
- The document MUST be a CV, Resume, or professional profile
- It should contain career-related information (work experience, education, or skills)
- Reject documents that are: police reports, medical records, legal documents, news articles, fiction, academic papers, manuals, or any non-career documents
- If the document is NOT a valid CV/Resume, return: {"isValid": false, "reason": "explanation"}

PARSING INSTRUCTIONS (only if document is valid):
- Extract personal information (name, contact details, location, professional title)
- Parse work experience entries with company, title, dates, and descriptions
- Parse education entries with institution, degree, field of study, and dates
- Parse certifications with name, issuing organization, issue/expiry dates, and credential details
- Extract skills as a list
- For dates, use formats: "YYYY-MM" for month precision, "YYYY" for year precision, "Present" for current positions
- Be precise and don't make up information that's not clearly stated
- If information is ambiguous or missing, use empty strings rather than guessing
- Always include: {"isValid": true} in your response for valid CVs

Document Text:
%s

Please return the information in the exact JSON schema format specified.`, cvText)
}

func (g *Gemini) buildCVParsingSystemInstruction() *genai.Content {
	instruction := `You are a precise CV/Resume parsing and validation system. Your primary task is to first validate that the document is actually a CV/Resume, then extract structured information if valid. Always include an "isValid" field in your response. Reject any documents that are not career-related (police reports, medical records, etc.). For valid CVs, focus on accuracy and completeness. Extract all certifications with their issuing organizations and dates. When dates are unclear, prefer broader ranges (year-only) over specific months. Do not hallucinate or guess information that is not explicitly stated.`

	contents := genai.Text(instruction)
	if len(contents) > 0 {
		return contents[0]
	}
	return nil
}

func (g *Gemini) buildCVGenerationSystemInstruction() *genai.Content {
	instruction := `You are an expert professional CV/Resume writer. Your task is to generate a comprehensive, tailored CV from the provided user profile data and job description.

CRITICAL RULES:
1. You MUST use ONLY the information provided in the USER PROFILE section
2. NEVER fabricate names, companies, job titles, education, or any other information
3. If the user's name is provided, use it. If not, leave it blank
4. Only include work experiences, education, and skills that are explicitly mentioned in the profile
5. Transform and enhance the presentation of existing information while maintaining complete honesty

SKILLS FILTERING:
- ONLY include skills that are DIRECTLY RELEVANT to the job posting
- If the job is for Python development, DO NOT include Java, Spring Boot, or unrelated technologies
- Order skills by relevance to the specific job requirements
- Focus on skills that match the job description's technology stack

WORK EXPERIENCE FORMATTING:
- Include company location if provided in the profile data
- CONTEXTUAL BULLET POINT CREATION: Generate 3-5 relevant bullet points per role based on the original description
- Extract and synthesize multiple achievements from single experience descriptions where applicable
- Create additional contextually relevant bullet points that align with the original role and job requirements
- Start each bullet with "• " (bullet character + space)
- TRANSFORM and ENHANCE: Convert basic responsibilities into achievement-focused, impactful statements
- Prioritize the most relevant achievements and responsibilities for the target job
- Quantify impact and results where possible (e.g., "Increased X by Y%", "Managed team of Z")
- Separate each bullet point with a newline

DATE FORMATTING:
- Use "Month Year" format (e.g., "August 2023", "Jan 2021")
- For current positions use "Present"
- Always include both month and year for clarity

Key guidelines:
- Always set "isValid" to true when generating a CV
- Use professional language and active voice
- Intelligently synthesize and expand existing experience descriptions into multiple focused achievements
- Extract implicit accomplishments and responsibilities from basic job descriptions
- Highlight and amplify relevant achievements that align with job requirements
- If information is missing (e.g., email, phone), leave those fields empty rather than inventing data
- Focus on presenting the user's actual experience in the best possible light`

	contents := genai.Text(instruction)
	if len(contents) > 0 {
		return contents[0]
	}
	return nil
}

func (g *Gemini) getCVParsingSchema() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"isValid": {
				Type:        genai.TypeBoolean,
				Description: "Whether the document is a valid CV/Resume",
			},
			"reason": {
				Type:        genai.TypeString,
				Description: "Reason for rejection if document is not valid (only required when isValid is false)",
			},
			"personalInfo": {
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"firstName": {Type: genai.TypeString, Description: "First name"},
					"lastName":  {Type: genai.TypeString, Description: "Last name"},
					"email":     {Type: genai.TypeString, Description: "Email address"},
					"phone":     {Type: genai.TypeString, Description: "Phone number"},
					"location":  {Type: genai.TypeString, Description: "Location/Address"},
					"title":     {Type: genai.TypeString, Description: "Professional title or role"},
				},
				Required: []string{"firstName", "lastName"},
			},
			"workExperience": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"company":     {Type: genai.TypeString, Description: "Company name"},
						"title":       {Type: genai.TypeString, Description: "Job title"},
						"location":    {Type: genai.TypeString, Description: "Job location"},
						"startDate":   {Type: genai.TypeString, Description: "Start date (YYYY-MM or YYYY format)"},
						"endDate":     {Type: genai.TypeString, Description: "End date (YYYY-MM, YYYY, or 'Present')"},
						"description": {Type: genai.TypeString, Description: "Job description/responsibilities"},
					},
					Required: []string{"company", "title", "startDate"},
				},
			},
			"education": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"institution":  {Type: genai.TypeString, Description: "Educational institution name"},
						"degree":       {Type: genai.TypeString, Description: "Degree type (e.g., BA, BS, Master)"},
						"fieldOfStudy": {Type: genai.TypeString, Description: "Field of study/major"},
						"startDate":    {Type: genai.TypeString, Description: "Start date (YYYY-MM or YYYY format)"},
						"endDate":      {Type: genai.TypeString, Description: "End date (YYYY-MM or YYYY format)"},
					},
					Required: []string{"institution", "degree", "startDate"},
				},
			},
			"certifications": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"name":          {Type: genai.TypeString, Description: "Certification name"},
						"issuingOrg":    {Type: genai.TypeString, Description: "Issuing organization"},
						"issueDate":     {Type: genai.TypeString, Description: "Issue date (YYYY-MM or YYYY format)"},
						"expiryDate":    {Type: genai.TypeString, Description: "Expiry date (YYYY-MM or YYYY format)"},
						"credentialId":  {Type: genai.TypeString, Description: "Credential ID or certificate number"},
						"credentialUrl": {Type: genai.TypeString, Description: "URL to verify credential"},
					},
					Required: []string{"name", "issuingOrg", "issueDate"},
				},
			},
			"skills": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeString,
				},
				Description: "List of skills and technologies",
			},
		},
		PropertyOrdering: []string{"isValid", "reason", "personalInfo", "workExperience", "education", "certifications", "skills"},
		Required:         []string{"isValid"},
	}
}

func (g *Gemini) getCVGenerationSchema() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"isValid": {
				Type:        genai.TypeBoolean,
				Description: "Always true for generated CVs",
			},
			"reason": {
				Type:        genai.TypeString,
				Description: "Not used for generation, leave empty",
			},
			"personalInfo": {
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"firstName": {Type: genai.TypeString, Description: "First name EXACTLY as provided in USER PROFILE - do not fabricate"},
					"lastName":  {Type: genai.TypeString, Description: "Last name EXACTLY as provided in USER PROFILE - do not fabricate"},
					"email":     {Type: genai.TypeString, Description: "Email EXACTLY as provided in USER PROFILE - leave empty if not provided"},
					"phone":     {Type: genai.TypeString, Description: "Phone EXACTLY as provided in USER PROFILE - leave empty if not provided"},
					"location":  {Type: genai.TypeString, Description: "Location EXACTLY as provided in USER PROFILE - leave empty if not provided"},
					"title":     {Type: genai.TypeString, Description: "Professional title based on profile, can be tailored to match job"},
				},
				Required: []string{},
			},
			"workExperience": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"company":     {Type: genai.TypeString, Description: "Company name EXACTLY as in USER PROFILE - do not fabricate"},
						"title":       {Type: genai.TypeString, Description: "Job title EXACTLY as in USER PROFILE - do not fabricate"},
						"location":    {Type: genai.TypeString, Description: "Job location (city, country) from USER PROFILE - MUST include if available in profile"},
						"startDate":   {Type: genai.TypeString, Description: "Start date from USER PROFILE (Month Year format, e.g., 'August 2023')"},
						"endDate":     {Type: genai.TypeString, Description: "End date from USER PROFILE (Month Year format, e.g., 'June 2024' or 'Present')"},
						"description": {Type: genai.TypeString, Description: "Multiple bullet points (4-5 for recent, 2-3 for older) starting with '• '. Each on new line. From USER PROFILE but tailored."},
					},
					Required: []string{"company", "title", "startDate", "description"},
				},
				Description: "ONLY work experiences from USER PROFILE - do not add fictional jobs",
			},
			"education": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"institution":  {Type: genai.TypeString, Description: "School/University EXACTLY as in USER PROFILE - do not fabricate"},
						"degree":       {Type: genai.TypeString, Description: "Degree EXACTLY as in USER PROFILE - do not fabricate"},
						"fieldOfStudy": {Type: genai.TypeString, Description: "Field of study from USER PROFILE"},
						"startDate":    {Type: genai.TypeString, Description: "Start date from USER PROFILE (Month Year format, e.g., 'Sep 2014')"},
						"endDate":      {Type: genai.TypeString, Description: "End date from USER PROFILE (Month Year format, e.g., 'Jun 2018')"},
					},
					Required: []string{"institution", "degree"},
				},
				Description: "ONLY education from USER PROFILE - do not add fictional schools",
			},
			"skills": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeString,
				},
				Description: "ONLY skills from USER PROFILE that are DIRECTLY RELEVANT to the job. Filter out unrelated technologies (e.g., no Java/Spring for Python jobs). Order by relevance.",
			},
		},
		PropertyOrdering: []string{"isValid", "personalInfo", "workExperience", "education", "skills"},
		Required:         []string{"isValid"},
	}
}

func (g *Gemini) parseCVJSON(jsonResponse string) (models.CVParsingResult, error) {
	cleanJSON := g.extractJSON(jsonResponse)

	var result models.CVParsingResult
	if err := json.Unmarshal([]byte(cleanJSON), &result); err != nil {
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

// parseGeneratedCVJSON parses JSON response from CV generation without strict validation
func (g *Gemini) parseGeneratedCVJSON(jsonResponse string) (models.CVParsingResult, error) {
	cleanJSON := g.extractJSON(jsonResponse)

	var result models.CVParsingResult
	if err := json.Unmarshal([]byte(cleanJSON), &result); err != nil {
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

// generateCV generates a CV based on the provided prompt.
func (g *Gemini) generateCV(ctx context.Context, prompt models.Prompt, start time.Time) (llm.GenerateResponse, error) {
	cvPrompt := g.buildCVGenerationPrompt(prompt)

	temperature := prompt.GetOptimalTemperature(models.TaskTypeCVGeneration.String())

	result, err := g.executeWithRetry(ctx, func() (string, error) {
		model := g.cfg.GetModelForTask(models.TaskTypeCVGeneration.String())
		resp, err := g.client.Models.GenerateContent(ctx, model, genai.Text(cvPrompt), &genai.GenerateContentConfig{
			Temperature:       &temperature,
			ResponseMIMEType:  g.cfg.ResponseMIMEType,
			ResponseSchema:    g.getCVGenerationSchema(),
			MaxOutputTokens:   g.cfg.MaxOutputTokens,
			TopP:              g.cfg.TopP,
			TopK:              g.cfg.TopK,
			SystemInstruction: g.buildCVGenerationSystemInstruction(),
		})
		if err != nil {
			return "", fmt.Errorf("generate content error: %w", err)
		}

		if len(resp.Candidates) == 0 {
			return "", fmt.Errorf("no candidates in response")
		}

		candidate := resp.Candidates[0]
		if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
			return "", fmt.Errorf("no content in response candidate")
		}

		var responseText string
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				responseText += part.Text
			}
		}

		return responseText, nil
	})

	if err != nil {
		return llm.GenerateResponse{}, WrapError(ErrCVGenFailed, err)
	}

	cvResult, err := g.parseGeneratedCVJSON(result)
	if err != nil {
		return llm.GenerateResponse{}, err
	}

	return llm.GenerateResponse{
		Data:     cvResult,
		Duration: time.Since(start),
		Tokens:   0,
		Metadata: map[string]any{
			"temperature": temperature,
			"enhanced":    prompt.UseEnhancedTemplates,
			"model":       g.cfg.GetModelForTask(models.TaskTypeCVGeneration.String()),
			"task_type":   models.TaskTypeCVGeneration.String(),
			"method":      "gemini_cv_generation",
		},
	}, nil
}

func (g *Gemini) buildCVGenerationPrompt(prompt models.Prompt) string {
	return prompt.ToCVGenerationPrompt()
}

// GetOptimizationStats returns statistics from all optimization components
// GetOptimizationStats returns statistics from all optimization components.
func (g *Gemini) GetOptimizationStats() OptimizationStats {
	return OptimizationStats{
		Cache:        g.cache.GetStats(),
		Deduplicator: g.deduplicator.GetStats(),
	}
}

// OptimizationStats aggregates metrics from all optimization components.
type OptimizationStats struct {
	Cache        CacheStats
	Deduplicator DeduplicatorStats
}

// ResetStats resets all optimization statistics
// ResetStats clears all optimization statistics.
func (g *Gemini) ResetStats() {
	g.cache.Clear()
	g.deduplicator.Clear()
}

// GetCacheStats returns cache statistics
// GetCacheStats returns cache performance metrics.
func (g *Gemini) GetCacheStats() CacheStats {
	return g.cache.GetStats()
}
