package gemini

import (
	"context"
	"testing"
	"time"

	"github.com/benidevo/vega/internal/ai/llm"
	"github.com/benidevo/vega/internal/ai/models"
	"github.com/stretchr/testify/assert"
	"google.golang.org/genai"
)

func TestGemini_extractJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "clean JSON object",
			input:    `{"content": "test"}`,
			expected: `{"content": "test"}`,
		},
		{
			name:     "JSON with extra text before",
			input:    `Here is the JSON: {"content": "test"}`,
			expected: `{"content": "test"}`,
		},
		{
			name:     "JSON with extra text after",
			input:    `{"content": "test"} - This is the result`,
			expected: `{"content": "test"}`,
		},
		{
			name:     "nested JSON object",
			input:    `{"outer": {"inner": "value"}, "count": 42}`,
			expected: `{"outer": {"inner": "value"}, "count": 42}`,
		},
		{
			name:     "JSON with text before and after",
			input:    `Response: {"status": "success", "data": {"value": 123}} End of response`,
			expected: `{"status": "success", "data": {"value": 123}}`,
		},
		{
			name:     "no JSON content",
			input:    `This is just plain text without JSON`,
			expected: `This is just plain text without JSON`,
		},
		{
			name:     "invalid JSON structures",
			input:    `{"content": "test"`,
			expected: `{"content": "test"`,
		},
		{
			name:     "empty input",
			input:    ``,
			expected: ``,
		},
		{
			name:     "multiple JSON objects - returns first",
			input:    `{"first": "object"} {"second": "object"}`,
			expected: `{"first": "object"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &Gemini{}
			result := g.extractJSON(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGemini_parseMatchResultJSON(t *testing.T) {
	cfg := &Config{
		MinMatchScore:       0,
		MaxMatchScore:       100,
		DefaultStrengthsMsg: "No strengths identified",
		DefaultWeaknessMsg:  "No weaknesses identified",
		DefaultHighlightMsg: "No highlights identified",
		DefaultFeedbackMsg:  "No feedback available",
	}
	g := &Gemini{cfg: cfg}

	tests := []struct {
		name          string
		input         string
		expected      models.MatchResult
		expectedError bool
	}{
		{
			name: "valid JSON response",
			input: `{
				"matchScore": 85,
				"strengths": ["Strong Go skills", "Good communication"],
				"weaknesses": ["Limited Docker experience"],
				"highlights": ["5 years experience", "Team lead"],
				"feedback": "Great candidate overall"
			}`,
			expected: models.MatchResult{
				MatchScore: 85,
				Strengths:  []string{"Strong Go skills", "Good communication"},
				Weaknesses: []string{"Limited Docker experience"},
				Highlights: []string{"5 years experience", "Team lead"},
				Feedback:   "Great candidate overall",
			},
			expectedError: false,
		},
		{
			name: "score out of range - too high",
			input: `{
				"matchScore": 150,
				"strengths": ["Good skills"],
				"weaknesses": ["Some gaps"],
				"highlights": ["Experience"],
				"feedback": "Good candidate"
			}`,
			expected: models.MatchResult{
				MatchScore: 0, // Should be corrected to min score
				Strengths:  []string{"Good skills"},
				Weaknesses: []string{"Some gaps"},
				Highlights: []string{"Experience"},
				Feedback:   "Good candidate",
			},
			expectedError: false,
		},
		{
			name: "score out of range - too low",
			input: `{
				"matchScore": -10,
				"strengths": ["Good skills"],
				"weaknesses": ["Some gaps"],
				"highlights": ["Experience"],
				"feedback": "Good candidate"
			}`,
			expected: models.MatchResult{
				MatchScore: 0, // Should be corrected to min score
				Strengths:  []string{"Good skills"},
				Weaknesses: []string{"Some gaps"},
				Highlights: []string{"Experience"},
				Feedback:   "Good candidate",
			},
			expectedError: false,
		},
		{
			name: "empty arrays get defaults",
			input: `{
				"matchScore": 75,
				"strengths": [],
				"weaknesses": [],
				"highlights": [],
				"feedback": ""
			}`,
			expected: models.MatchResult{
				MatchScore: 75,
				Strengths:  []string{"No strengths identified"},
				Weaknesses: []string{"No weaknesses identified"},
				Highlights: []string{"No highlights identified"},
				Feedback:   "No feedback available",
			},
			expectedError: false,
		},
		{
			name:          "invalid JSON",
			input:         `{"matchScore": 85, "strengths": [}`,
			expected:      models.MatchResult{},
			expectedError: true,
		},
		{
			name: "JSON with extra text",
			input: `Here is the analysis: {
				"matchScore": 90,
				"strengths": ["Excellent skills"],
				"weaknesses": ["Minor gaps"],
				"highlights": ["Leadership"],
				"feedback": "Top candidate"
			} End of analysis`,
			expected: models.MatchResult{
				MatchScore: 90,
				Strengths:  []string{"Excellent skills"},
				Weaknesses: []string{"Minor gaps"},
				Highlights: []string{"Leadership"},
				Feedback:   "Top candidate",
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := g.parseMatchResultJSON(tt.input)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected.MatchScore, result.MatchScore)
				assert.Equal(t, tt.expected.Strengths, result.Strengths)
				assert.Equal(t, tt.expected.Weaknesses, result.Weaknesses)
				assert.Equal(t, tt.expected.Highlights, result.Highlights)
				assert.Equal(t, tt.expected.Feedback, result.Feedback)
			}
		})
	}
}

func TestGemini_parseCoverLetterJSON(t *testing.T) {
	g := &Gemini{}

	tests := []struct {
		name          string
		input         string
		expected      models.CoverLetter
		expectedError bool
	}{
		{
			name:  "valid cover letter JSON",
			input: `{"content": "Dear Hiring Manager,\n\nI am writing to express my interest..."}`,
			expected: models.CoverLetter{
				Content: "Dear Hiring Manager,\n\nI am writing to express my interest...",
				Format:  models.CoverLetterTypePlainText,
			},
			expectedError: false,
		},
		{
			name:  "cover letter with extra text",
			input: `Here is your cover letter: {"content": "Dear Sir/Madam,\n\nApplication for the position..."} Hope this helps!`,
			expected: models.CoverLetter{
				Content: "Dear Sir/Madam,\n\nApplication for the position...",
				Format:  models.CoverLetterTypePlainText,
			},
			expectedError: false,
		},
		{
			name:          "empty content",
			input:         `{"content": ""}`,
			expected:      models.CoverLetter{},
			expectedError: true,
		},
		{
			name:          "missing content field",
			input:         `{"title": "test"}`,
			expected:      models.CoverLetter{},
			expectedError: true,
		},
		{
			name:          "malformed JSON",
			input:         `{"content": "test"`,
			expected:      models.CoverLetter{},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := g.parseCoverLetterJSON(tt.input)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected.Content, result.Content)
				assert.Equal(t, tt.expected.Format, result.Format)
			}
		})
	}
}

func TestGemini_executeWithRetry(t *testing.T) {
	tests := []struct {
		name          string
		config        *Config
		operation     func() (string, error)
		expectedError bool
		expectRetries bool
	}{
		{
			name: "successful operation on first try",
			config: &Config{
				MaxRetries:     3,
				BaseRetryDelay: 1,
				MaxRetryDelay:  10,
			},
			operation: func() (string, error) {
				return "success", nil
			},
			expectedError: false,
			expectRetries: false,
		},
		{
			name: "retryable error eventually succeeds",
			config: &Config{
				MaxRetries:     2,
				BaseRetryDelay: 1,
				MaxRetryDelay:  10,
			},
			operation: func() func() (string, error) {
				attemptCount := 0
				return func() (string, error) {
					attemptCount++
					if attemptCount < 2 {
						return "", NewGeminiError(503, "service unavailable", nil)
					}
					return "success after retries", nil
				}
			}(),
			expectedError: false,
			expectRetries: true,
		},
		{
			name: "non-retryable error fails immediately",
			config: &Config{
				MaxRetries:     3,
				BaseRetryDelay: 1,
				MaxRetryDelay:  10,
			},
			operation: func() (string, error) {
				return "", NewGeminiError(400, "bad request", nil)
			},
			expectedError: true,
			expectRetries: false,
		},
		{
			name: "max retries exceeded",
			config: &Config{
				MaxRetries:     2,
				BaseRetryDelay: 1,
				MaxRetryDelay:  10,
			},
			operation: func() (string, error) {
				return "", NewGeminiError(503, "service unavailable", nil)
			},
			expectedError: true,
			expectRetries: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &Gemini{cfg: tt.config}
			ctx := context.Background()

			start := time.Now()
			result, err := g.executeWithRetry(ctx, tt.operation)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, result)
			}

			if tt.expectRetries {
				// If retries were expected, operation should have taken some time
				duration := time.Since(start)
				if !tt.expectedError {
					// Only check duration if operation eventually succeeded
					assert.True(t, duration > time.Millisecond*500, "Expected retry delays")
				}
			}
		})
	}
}

func TestGemini_Generate_UnsupportedResponseType(t *testing.T) {
	cfg := &Config{
		APIKey: "test-key",
		Model:  "gemini-1.5-flash",
	}
	g := &Gemini{
		cfg:            cfg,
		cache:          NewResponseCache(10, 5*time.Minute),
		circuitBreaker: NewCircuitBreaker(DefaultCircuitBreakerConfig()),
		deduplicator:   NewRequestDeduplicator(),
	}

	req := llm.GenerateRequest{
		ResponseType: "unsupported_type",
		Prompt: models.Prompt{
			Instructions: "Test",
			Request: models.Request{
				ApplicantName:    "Test User",
				ApplicantProfile: "Test Profile",
				JobDescription:   "Test Job",
			},
		},
	}

	_, err := g.Generate(context.Background(), req)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported response type")
}

func TestGemini_TaskSpecificModelIntegration(t *testing.T) {
	tests := []struct {
		name          string
		config        *Config
		responseType  llm.ResponseType
		expectedModel string
	}{
		{
			name: "CV parsing response type uses CV parsing model",
			config: &Config{
				Model:            "gemini-2.5-flash",
				ModelCVParsing:   "gemini-1.5-flash",
				ModelJobAnalysis: "gemini-2.5-flash",
				ModelCoverLetter: "gemini-2.5-flash",
			},
			responseType:  llm.ResponseTypeCVParsing,
			expectedModel: "gemini-1.5-flash",
		},
		{
			name: "Match result response type uses job analysis model",
			config: &Config{
				Model:            "gemini-1.5-flash",
				ModelCVParsing:   "gemini-1.5-flash",
				ModelJobAnalysis: "gemini-2.5-flash",
				ModelCoverLetter: "gemini-2.5-flash",
			},
			responseType:  llm.ResponseTypeMatchResult,
			expectedModel: "gemini-2.5-flash",
		},
		{
			name: "Cover letter response type uses cover letter model",
			config: &Config{
				Model:            "gemini-1.5-flash",
				ModelCVParsing:   "gemini-1.5-flash",
				ModelJobAnalysis: "gemini-2.5-flash",
				ModelCoverLetter: "gemini-2.5-flash",
			},
			responseType:  llm.ResponseTypeCoverLetter,
			expectedModel: "gemini-2.5-flash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify the configuration correctly selects the expected model for the task
			var taskType string
			switch tt.responseType {
			case llm.ResponseTypeCVParsing:
				taskType = "cv_parsing"
			case llm.ResponseTypeMatchResult:
				taskType = "job_analysis"
			case llm.ResponseTypeCoverLetter:
				taskType = "cover_letter"
			}

			result := tt.config.GetModelForTask(taskType)
			assert.Equal(t, tt.expectedModel, result)
		})
	}
}

func TestGemini_parseCVJSON(t *testing.T) {
	g := &Gemini{}

	tests := []struct {
		name          string
		input         string
		expected      models.CVParsingResult
		expectedError string
	}{
		{
			name: "valid CV JSON with complete information",
			input: `{
				"isValid": true,
				"personalInfo": {
					"firstName": "John",
					"lastName": "Doe",
					"email": "john.doe@email.com",
					"phone": "+1-555-123-4567",
					"location": "San Francisco, CA",
					"title": "Senior Software Engineer"
				},
				"workExperience": [
					{
						"company": "Tech Corp",
						"title": "Senior Engineer",
						"startDate": "2020-01",
						"endDate": "Present",
						"description": "Led development of microservices"
					}
				],
				"education": [
					{
						"institution": "UC Berkeley",
						"degree": "BS",
						"fieldOfStudy": "Computer Science",
						"startDate": "2014",
						"endDate": "2018"
					}
				],
				"skills": ["Go", "Python", "JavaScript"]
			}`,
			expected: models.CVParsingResult{
				IsValid: true,
				PersonalInfo: models.PersonalInfo{
					FirstName: "John",
					LastName:  "Doe",
					Email:     "john.doe@email.com",
					Phone:     "+1-555-123-4567",
					Location:  "San Francisco, CA",
					Title:     "Senior Software Engineer",
				},
				WorkExperience: []models.WorkExperience{
					{
						Company:     "Tech Corp",
						Title:       "Senior Engineer",
						StartDate:   "2020-01",
						EndDate:     "Present",
						Description: "Led development of microservices",
					},
				},
				Education: []models.Education{
					{
						Institution:  "UC Berkeley",
						Degree:       "BS",
						FieldOfStudy: "Computer Science",
						StartDate:    "2014",
						EndDate:      "2018",
					},
				},
				Skills: []string{"Go", "Python", "JavaScript"},
			},
			expectedError: "",
		},
		{
			name: "invalid document gets rejected",
			input: `{
				"isValid": false,
				"reason": "Document appears to be a police report, not a CV/Resume"
			}`,
			expected:      models.CVParsingResult{},
			expectedError: "invalid document: Document appears to be a police report, not a CV/Resume",
		},
		{
			name: "invalid document with missing reason",
			input: `{
				"isValid": false
			}`,
			expected:      models.CVParsingResult{},
			expectedError: "invalid document: Document is not a valid CV/Resume",
		},
		{
			name: "valid CV but missing name",
			input: `{
				"isValid": true,
				"personalInfo": {
					"firstName": "",
					"lastName": "",
					"email": "test@email.com"
				},
				"skills": ["Python"]
			}`,
			expected:      models.CVParsingResult{},
			expectedError: "no name found in CV",
		},
		{
			name: "valid CV ensures arrays are not nil",
			input: `{
				"isValid": true,
				"personalInfo": {
					"firstName": "Jane",
					"lastName": "Smith"
				}
			}`,
			expected: models.CVParsingResult{
				IsValid: true,
				PersonalInfo: models.PersonalInfo{
					FirstName: "Jane",
					LastName:  "Smith",
				},
				WorkExperience: []models.WorkExperience{},
				Education:      []models.Education{},
				Skills:         []string{},
			},
			expectedError: "",
		},
		{
			name:          "malformed JSON",
			input:         `{"isValid": true, "personalInfo": {`,
			expected:      models.CVParsingResult{},
			expectedError: "failed to parse Gemini response",
		},
		{
			name: "CV with extra text around JSON",
			input: `Here is the parsed CV data: {
				"isValid": true,
				"personalInfo": {
					"firstName": "Bob",
					"lastName": "Wilson"
				},
				"skills": ["Java", "SQL"]
			} End of parsing`,
			expected: models.CVParsingResult{
				IsValid: true,
				PersonalInfo: models.PersonalInfo{
					FirstName: "Bob",
					LastName:  "Wilson",
				},
				WorkExperience: []models.WorkExperience{},
				Education:      []models.Education{},
				Skills:         []string{"Java", "SQL"},
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := g.parseCVJSON(tt.input)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected.IsValid, result.IsValid)
				assert.Equal(t, tt.expected.PersonalInfo.FirstName, result.PersonalInfo.FirstName)
				assert.Equal(t, tt.expected.PersonalInfo.LastName, result.PersonalInfo.LastName)
				assert.Equal(t, tt.expected.PersonalInfo.Email, result.PersonalInfo.Email)
				assert.Equal(t, tt.expected.PersonalInfo.Phone, result.PersonalInfo.Phone)
				assert.Equal(t, tt.expected.PersonalInfo.Location, result.PersonalInfo.Location)
				assert.Equal(t, tt.expected.PersonalInfo.Title, result.PersonalInfo.Title)

				// Verify arrays are correctly handled
				assert.NotNil(t, result.WorkExperience)
				assert.NotNil(t, result.Education)
				assert.NotNil(t, result.Skills)

				assert.Equal(t, len(tt.expected.WorkExperience), len(result.WorkExperience))
				assert.Equal(t, len(tt.expected.Education), len(result.Education))
				assert.Equal(t, tt.expected.Skills, result.Skills)
			}
		})
	}
}

func TestGemini_parseGeneratedCVJSON(t *testing.T) {
	g := &Gemini{}

	tests := []struct {
		name     string
		input    string
		expected models.CVParsingResult
	}{
		{
			name: "valid generated CV JSON",
			input: `{
				"isValid": true,
				"personalInfo": {
					"firstName": "John",
					"lastName": "Doe",
					"email": "john@example.com",
					"title": "Software Engineer"
				},
				"workExperience": [{
					"company": "Tech Corp",
					"title": "Developer",
					"location": "San Francisco, CA",
					"startDate": "January 2020",
					"endDate": "Present",
					"description": "• Built web applications\n• Led team projects"
				}],
				"education": [{
					"institution": "University",
					"degree": "BS",
					"fieldOfStudy": "Computer Science",
					"endDate": "May 2019"
				}],
				"skills": ["Go", "Python", "React"]
			}`,
			expected: models.CVParsingResult{
				IsValid: true,
				PersonalInfo: models.PersonalInfo{
					FirstName: "John",
					LastName:  "Doe",
					Email:     "john@example.com",
					Title:     "Software Engineer",
				},
				WorkExperience: []models.WorkExperience{{
					Company:     "Tech Corp",
					Title:       "Developer",
					Location:    "San Francisco, CA",
					StartDate:   "January 2020",
					EndDate:     "Present",
					Description: "• Built web applications\n• Led team projects",
				}},
				Education: []models.Education{{
					Institution:  "University",
					Degree:       "BS",
					FieldOfStudy: "Computer Science",
					EndDate:      "May 2019",
				}},
				Skills: []string{"Go", "Python", "React"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := g.parseGeneratedCVJSON(tt.input)

			assert.NoError(t, err)
			assert.True(t, result.IsValid)
			assert.Equal(t, tt.expected.PersonalInfo.FirstName, result.PersonalInfo.FirstName)
			assert.Equal(t, tt.expected.PersonalInfo.LastName, result.PersonalInfo.LastName)
			assert.Equal(t, len(tt.expected.WorkExperience), len(result.WorkExperience))
			assert.Equal(t, len(tt.expected.Education), len(result.Education))
			assert.Equal(t, tt.expected.Skills, result.Skills)
		})
	}
}

func TestGemini_buildCVParsingPrompt(t *testing.T) {
	g := &Gemini{}

	prompt := models.Prompt{
		Instructions: "test instructions",
		CVText:       "John Doe\nSoftware Engineer\n5 years experience",
	}

	result := g.buildCVParsingPrompt(prompt)

	assert.Contains(t, result, "expert CV/Resume parser")
	assert.Contains(t, result, prompt.CVText)
	assert.Contains(t, result, "Document Text:")
}

func TestGemini_buildSystemInstruction(t *testing.T) {
	g := &Gemini{
		cfg: &Config{
			SystemInstruction: "You are a professional AI assistant specialized in professional career services and providing structured JSON responses.",
		},
	}

	instruction := g.buildSystemInstruction()

	assert.NotNil(t, instruction)
	assert.NotNil(t, instruction.Parts)
	assert.Greater(t, len(instruction.Parts), 0)
}

func TestGemini_buildCVParsingSystemInstruction(t *testing.T) {
	g := &Gemini{}

	instruction := g.buildCVParsingSystemInstruction()

	assert.NotNil(t, instruction)
	assert.NotNil(t, instruction.Parts)
	assert.Greater(t, len(instruction.Parts), 0)
}

func TestGemini_buildCVGenerationSystemInstruction(t *testing.T) {
	g := &Gemini{}

	instruction := g.buildCVGenerationSystemInstruction()

	assert.NotNil(t, instruction)
	assert.NotNil(t, instruction.Parts)
	assert.Greater(t, len(instruction.Parts), 0)
}

func TestGemini_getMatchAnalysisSchema(t *testing.T) {
	g := &Gemini{
		cfg: &Config{
			MinMatchScore: 0,
			MaxMatchScore: 100,
		},
	}

	schema := g.getMatchAnalysisSchema()

	assert.NotNil(t, schema)
	assert.Equal(t, genai.TypeObject, schema.Type)
	assert.NotNil(t, schema.Properties)

	// Check for required properties
	assert.Contains(t, schema.Properties, "matchScore")
	assert.Contains(t, schema.Properties, "strengths")
	assert.Contains(t, schema.Properties, "weaknesses")
	assert.Contains(t, schema.Properties, "highlights")
	assert.Contains(t, schema.Properties, "feedback")

	// Check matchScore property details
	matchScore := schema.Properties["matchScore"]
	assert.Equal(t, genai.TypeInteger, matchScore.Type)
}

func TestGemini_getCoverLetterSchema(t *testing.T) {
	g := &Gemini{}

	schema := g.getCoverLetterSchema()

	assert.NotNil(t, schema)
	assert.Equal(t, genai.TypeObject, schema.Type)
	assert.NotNil(t, schema.Properties)

	// Check for required property
	assert.Contains(t, schema.Properties, "content")
	assert.Contains(t, schema.Required, "content")

	// Check content property details
	content := schema.Properties["content"]
	assert.Equal(t, genai.TypeString, content.Type)
	assert.Equal(t, "The complete cover letter content", content.Description)
}

func TestGemini_getCVParsingSchema(t *testing.T) {
	g := &Gemini{}

	schema := g.getCVParsingSchema()

	assert.NotNil(t, schema)
	assert.Equal(t, genai.TypeObject, schema.Type)
	assert.NotNil(t, schema.Properties)

	// Check for required properties
	assert.Contains(t, schema.Properties, "isValid")
	assert.Contains(t, schema.Properties, "personalInfo")
	assert.Contains(t, schema.Properties, "workExperience")
	assert.Contains(t, schema.Properties, "education")
	assert.Contains(t, schema.Properties, "skills")
	assert.Contains(t, schema.Properties, "reason")

	// Check isValid property
	isValid := schema.Properties["isValid"]
	assert.Equal(t, genai.TypeBoolean, isValid.Type)

	// Check personalInfo structure
	personalInfo := schema.Properties["personalInfo"]
	assert.Equal(t, genai.TypeObject, personalInfo.Type)
	assert.Contains(t, personalInfo.Properties, "firstName")
	assert.Contains(t, personalInfo.Properties, "lastName")
	assert.Contains(t, personalInfo.Properties, "email")
}

func TestGemini_getCVGenerationSchema(t *testing.T) {
	g := &Gemini{}

	schema := g.getCVGenerationSchema()

	assert.NotNil(t, schema)
	assert.Equal(t, genai.TypeObject, schema.Type)
	assert.NotNil(t, schema.Properties)

	// Check structure is similar to parsing schema
	assert.Contains(t, schema.Properties, "isValid")
	assert.Contains(t, schema.Properties, "personalInfo")
	assert.Contains(t, schema.Properties, "workExperience")
	assert.Contains(t, schema.Properties, "education")
	assert.Contains(t, schema.Properties, "skills")

	// Check workExperience array structure
	workExp := schema.Properties["workExperience"]
	assert.Equal(t, genai.TypeArray, workExp.Type)
	assert.NotNil(t, workExp.Items)
	assert.Equal(t, genai.TypeObject, workExp.Items.Type)
	assert.Contains(t, workExp.Items.Properties, "company")
	assert.Contains(t, workExp.Items.Properties, "title")
	assert.Contains(t, workExp.Items.Properties, "location")
}
