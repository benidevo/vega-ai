package models

import (
	"testing"
	"time"

	settingsmodels "github.com/benidevo/vega/internal/settings/models"
	"github.com/stretchr/testify/assert"
)

func TestAITaskType_String(t *testing.T) {
	tests := []struct {
		name     string
		taskType AITaskType
		expected string
	}{
		{
			name:     "should_return_cv_parsing_string_when_cv_parsing_type",
			taskType: TaskTypeCVParsing,
			expected: "cv_parsing",
		},
		{
			name:     "should_return_job_analysis_string_when_job_analysis_type",
			taskType: TaskTypeJobAnalysis,
			expected: "job_analysis",
		},
		{
			name:     "should_return_match_result_string_when_match_result_type",
			taskType: TaskTypeMatchResult,
			expected: "match_result",
		},
		{
			name:     "should_return_cover_letter_string_when_cover_letter_type",
			taskType: TaskTypeCoverLetter,
			expected: "cover_letter",
		},
		{
			name:     "should_return_cv_generation_string_when_cv_generation_type",
			taskType: TaskTypeCVGeneration,
			expected: "cv_generation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.taskType.String())
		})
	}
}

func TestNewPrompt(t *testing.T) {
	tests := []struct {
		name         string
		instructions string
		request      Request
		useEnhanced  bool
	}{
		{
			name:         "should_create_prompt_with_basic_features_when_not_enhanced",
			instructions: "Generate a cover letter",
			request: Request{
				ApplicantName:    "John Doe",
				ApplicantProfile: "Software Engineer",
				JobDescription:   "Senior Go Developer",
				ExtraContext:     "Remote position",
			},
			useEnhanced: false,
		},
		{
			name:         "should_create_prompt_with_enhanced_features_when_enhanced",
			instructions: "Analyze job match",
			request: Request{
				ApplicantName:    "Jane Smith",
				ApplicantProfile: "Data Scientist",
				JobDescription:   "ML Engineer Role",
				CVText:           "Experienced in Python and ML",
			},
			useEnhanced: true,
		},
		{
			name:         "should_copy_cv_text_from_request_when_provided",
			instructions: "Parse CV",
			request: Request{
				CVText: "John Doe\nSoftware Engineer\n5 years experience",
			},
			useEnhanced: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := NewPrompt(tt.instructions, tt.request, tt.useEnhanced)

			assert.NotNil(t, prompt)
			assert.Equal(t, tt.instructions, prompt.Instructions)
			assert.Equal(t, tt.request, prompt.Request)
			assert.Equal(t, tt.useEnhanced, prompt.UseEnhancedTemplates)
			assert.Equal(t, tt.request.CVText, prompt.CVText)
			assert.NotNil(t, prompt.sanitizer) // Sanitizer should always be initialized

			if tt.useEnhanced {
				assert.NotNil(t, prompt.promptEnhancer)
			} else {
				assert.Nil(t, prompt.promptEnhancer)
			}
		})
	}
}

func TestNewCVParsingPrompt(t *testing.T) {
	tests := []struct {
		name   string
		cvText string
	}{
		{
			name:   "should_create_cv_parsing_prompt_with_text_when_provided",
			cvText: "John Doe\nSenior Software Engineer\n10 years experience in Go and Python",
		},
		{
			name:   "should_create_cv_parsing_prompt_with_empty_text_when_empty",
			cvText: "",
		},
		{
			name:   "should_handle_multiline_cv_text_when_complex_cv",
			cvText: "Jane Smith\n\nEXPERIENCE:\n- Company A (2020-2023)\n- Company B (2018-2020)\n\nSKILLS:\n- Python\n- Machine Learning\n- Data Analysis",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := NewCVParsingPrompt(tt.cvText)

			assert.NotNil(t, prompt)
			assert.Equal(t, "Parse CV and extract structured information", prompt.Instructions)
			assert.Equal(t, tt.cvText, prompt.CVText)
			assert.False(t, prompt.UseEnhancedTemplates)
			assert.NotNil(t, prompt.sanitizer)
			assert.Nil(t, prompt.promptEnhancer)
		})
	}
}

func TestPrompt_SetTemperature(t *testing.T) {
	tests := []struct {
		name        string
		temperature float32
	}{
		{
			name:        "should_set_zero_temperature_when_zero_provided",
			temperature: 0.0,
		},
		{
			name:        "should_set_low_temperature_when_low_value_provided",
			temperature: 0.3,
		},
		{
			name:        "should_set_medium_temperature_when_medium_value_provided",
			temperature: 0.7,
		},
		{
			name:        "should_set_high_temperature_when_high_value_provided",
			temperature: 0.9,
		},
		{
			name:        "should_set_max_temperature_when_one_provided",
			temperature: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := NewCVParsingPrompt("test cv")
			assert.Nil(t, prompt.Temperature) // Initially nil

			prompt.SetTemperature(tt.temperature)

			assert.NotNil(t, prompt.Temperature)
			assert.Equal(t, tt.temperature, *prompt.Temperature)
		})
	}
}

func TestPrompt_GetOptimalTemperature(t *testing.T) {
	tests := []struct {
		name       string
		promptType string
		customTemp *float32
		expected   float32
	}{
		{
			name:       "should_return_custom_temperature_when_set",
			promptType: "cover_letter",
			customTemp: func() *float32 { t := float32(0.5); return &t }(),
			expected:   0.5,
		},
		{
			name:       "should_return_default_temperature_for_cv_parsing_when_no_custom",
			promptType: "cv_parsing",
			customTemp: nil,
			expected:   0.4, // Default temperature
		},
		{
			name:       "should_return_low_temperature_for_job_analysis_when_no_custom",
			promptType: "job_analysis",
			customTemp: nil,
			expected:   0.2, // Lower for analytical consistency
		},
		{
			name:       "should_return_default_temperature_for_match_result_when_no_custom",
			promptType: "match_result",
			customTemp: nil,
			expected:   0.4, // Default temperature
		},
		{
			name:       "should_return_creative_temperature_for_cover_letter_when_no_custom",
			promptType: "cover_letter",
			customTemp: nil,
			expected:   0.65, // Higher creativity for writing
		},
		{
			name:       "should_return_creative_temperature_for_cv_generation_when_no_custom",
			promptType: "cv_generation",
			customTemp: nil,
			expected:   0.55, // Higher creativity for CV content transformation
		},
		{
			name:       "should_return_default_temperature_for_unknown_type_when_no_custom",
			promptType: "unknown_type",
			customTemp: nil,
			expected:   0.4, // Default balanced temperature
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := NewCVParsingPrompt("test")
			prompt.Temperature = tt.customTemp

			result := prompt.GetOptimalTemperature(tt.promptType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPrompt_ToCVGenerationPrompt(t *testing.T) {
	tests := []struct {
		name     string
		prompt   Prompt
		contains []string
	}{
		{
			name: "should_generate_cv_prompt_with_all_fields_when_complete_request",
			prompt: Prompt{
				Instructions: "Generate CV",
				CVText:       "Alice Johnson\nExperienced software engineer with 10 years in tech",
				Request: Request{
					CVText:         "Alice Johnson\nExperienced software engineer with 10 years in tech",
					JobDescription: "Senior Software Engineer at Tech Corp",
					ExtraContext:   "Focus on cloud technologies",
					WorkExperience: []settingsmodels.WorkExperience{
						{
							Company:     "Previous Corp",
							Title:       "Software Engineer",
							Description: "Developed microservices",
							StartDate:   time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
							EndDate:     func() *time.Time { t := time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC); return &t }(),
						},
					},
					Education: []settingsmodels.Education{
						{
							Institution: "MIT",
							Degree:      "BS Computer Science",
							StartDate:   time.Date(2010, 9, 1, 0, 0, 0, 0, time.UTC),
							EndDate:     func() *time.Time { t := time.Date(2014, 6, 30, 0, 0, 0, 0, time.UTC); return &t }(),
						},
					},
					Skills:          []string{"Go", "Python", "Kubernetes"},
					YearsExperience: 10,
				},
			},
			contains: []string{
				"Generate CV",
				"Alice Johnson",
				"Senior Software Engineer at Tech Corp",
				"10 years in tech",
				"cloud technologies",
			},
		},
		{
			name: "should_handle_minimal_request_when_only_basic_info",
			prompt: Prompt{
				Instructions: "Create CV",
				CVText:       "Bob Smith\nSoftware Developer",
				Request: Request{
					CVText:         "Bob Smith\nSoftware Developer",
					JobDescription: "Developer Role",
				},
			},
			contains: []string{
				"Create CV",
				"Bob Smith",
				"Developer Role",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.prompt.ToCVGenerationPrompt()

			for _, text := range tt.contains {
				assert.Contains(t, result, text)
			}

			// Should always contain JSON structure instructions
			assert.Contains(t, result, "JSON")
			assert.Contains(t, result, "structured CV")
			assert.Contains(t, result, "SOURCE OF TRUTH")
			assert.Contains(t, result, "TARGET JOB")
		})
	}
}

func TestPrompt_ToCoverLetterPrompt(t *testing.T) {
	tests := []struct {
		name             string
		prompt           Prompt
		defaultWordRange string
		expectedContains []string
	}{
		{
			name: "should_generate_complete_cover_letter_prompt_when_all_fields_provided",
			prompt: Prompt{
				Instructions: "Generate a professional cover letter",
				Request: Request{
					ApplicantName:    "John Doe",
					ApplicantProfile: "Senior Software Engineer with 5 years experience",
					JobDescription:   "Looking for a Go developer",
					ExtraContext:     "Remote position preferred",
				},
			},
			defaultWordRange: "250-400",
			expectedContains: []string{
				"Generate a professional cover letter",
				"John Doe",
				"Senior Software Engineer with 5 years experience",
				"Looking for a Go developer",
				"Remote position preferred",
				"250-400 words",
				"JSON object",
				"content:",
			},
		},
		{
			name: "should_handle_empty_extra_context_when_not_provided",
			prompt: Prompt{
				Instructions: "Create cover letter",
				Request: Request{
					ApplicantName:    "Jane Smith",
					ApplicantProfile: "Marketing Manager",
					JobDescription:   "Marketing role",
					ExtraContext:     "",
				},
			},
			defaultWordRange: "200-300",
			expectedContains: []string{
				"Create cover letter",
				"Jane Smith",
				"Marketing Manager",
				"Marketing role",
				"200-300 words",
			},
		},
		{
			name: "should_use_custom_word_range_when_specified",
			prompt: Prompt{
				Instructions: "Write letter",
				Request: Request{
					ApplicantName:    "Bob Wilson",
					ApplicantProfile: "Designer",
					JobDescription:   "Design position",
				},
			},
			defaultWordRange: "100-200",
			expectedContains: []string{
				"100-200 words",
				"Bob Wilson",
				"Designer",
				"Design position",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.prompt.ToCoverLetterPrompt(tt.defaultWordRange)

			assert.NotEmpty(t, result)
			for _, expected := range tt.expectedContains {
				assert.Contains(t, result, expected)
			}

			// Ensure JSON structure requirements are present
			assert.Contains(t, result, "JSON object")
			assert.Contains(t, result, "content:")
			assert.Contains(t, result, "professional")
		})
	}
}

func TestPrompt_ToMatchAnalysisPrompt(t *testing.T) {
	tests := []struct {
		name             string
		prompt           Prompt
		minMatchScore    int
		maxMatchScore    int
		expectedContains []string
	}{
		{
			name: "should_generate_complete_match_analysis_prompt_when_all_fields_provided",
			prompt: Prompt{
				Instructions: "Analyze the job match",
				Request: Request{
					ApplicantName:    "Alice Johnson",
					ApplicantProfile: "Full-stack developer with React and Node.js",
					JobDescription:   "React developer position at startup",
					ExtraContext:     "Startup environment experience preferred",
				},
			},
			minMatchScore: 0,
			maxMatchScore: 100,
			expectedContains: []string{
				"Analyze the job match",
				"Full-stack developer with React and Node.js",
				"React developer position at startup",
				"Startup environment experience preferred",
				"integer from 0-100",
				"where 0 is no match and 100 is perfect match",
				"matchScore:",
				"strengths:",
				"weaknesses:",
				"highlights:",
				"feedback:",
				"JSON object",
			},
		},
		{
			name: "should_use_custom_score_range_when_specified",
			prompt: Prompt{
				Instructions: "Match analysis",
				Request: Request{
					ApplicantName:    "Charlie Brown",
					ApplicantProfile: "Project Manager",
					JobDescription:   "PM role",
				},
			},
			minMatchScore: 10,
			maxMatchScore: 90,
			expectedContains: []string{
				"integer from 10-90",
				"where 10 is no match and 90 is perfect match",
				"Project Manager",
				"PM role",
			},
		},
		{
			name: "should_include_default_criteria_when_extra_context_empty",
			prompt: Prompt{
				Instructions: "Analyze match",
				Request: Request{
					ApplicantName:    "David Lee",
					ApplicantProfile: "Data Scientist",
					JobDescription:   "ML Engineer role",
					ExtraContext:     "",
				},
			},
			minMatchScore: 0,
			maxMatchScore: 100,
			expectedContains: []string{
				"Data Scientist",
				"ML Engineer role",
				"Skills alignment",
				"Experience level",
				"Industry knowledge",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.prompt.ToMatchAnalysisPrompt(tt.minMatchScore, tt.maxMatchScore)

			assert.NotEmpty(t, result)
			for _, expected := range tt.expectedContains {
				assert.Contains(t, result, expected)
			}

			assert.Contains(t, result, "Skills alignment")
			assert.Contains(t, result, "Experience level")
			assert.Contains(t, result, "Industry knowledge")
			assert.Contains(t, result, "Cultural fit")
			assert.Contains(t, result, "Growth potential")

			assert.Contains(t, result, "JSON object")
			assert.Contains(t, result, "EXACTLY this structure")
			assert.Contains(t, result, "do NOT mention the applicant's name")
		})
	}
}

func TestPrompt_BoundaryConditions(t *testing.T) {
	t.Run("should_generate_valid_structure_when_all_fields_empty", func(t *testing.T) {
		prompt := Prompt{
			Instructions: "",
			Request: Request{
				ApplicantName:    "",
				ApplicantProfile: "",
				JobDescription:   "",
				ExtraContext:     "",
			},
		}

		result := prompt.ToCoverLetterPrompt("100-200")

		// Should still generate a valid prompt structure
		assert.Contains(t, result, "JSON object")
		assert.Contains(t, result, "content:")
		assert.Contains(t, result, "100-200 words")
	})

	t.Run("should_generate_valid_json_structure_when_fields_empty", func(t *testing.T) {
		prompt := Prompt{
			Instructions: "",
			Request: Request{
				ApplicantName:    "",
				ApplicantProfile: "",
				JobDescription:   "",
				ExtraContext:     "",
			},
		}

		result := prompt.ToMatchAnalysisPrompt(0, 100)

		assert.Contains(t, result, "JSON object")
		assert.Contains(t, result, "matchScore:")
		assert.Contains(t, result, "integer from 0-100")
	})

	t.Run("should_handle_extreme_score_boundaries_when_provided", func(t *testing.T) {
		prompt := Prompt{
			Instructions: "Test",
			Request: Request{
				ApplicantName:    "Test User",
				ApplicantProfile: "Test Profile",
				JobDescription:   "Test Job",
			},
		}

		result := prompt.ToMatchAnalysisPrompt(-50, 200)

		assert.Contains(t, result, "integer from -50-200")
		assert.Contains(t, result, "where -50 is no match and 200 is perfect match")
	})
}
