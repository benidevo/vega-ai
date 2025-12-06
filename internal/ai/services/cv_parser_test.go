package services

import (
	"context"
	"fmt"
	"testing"

	"github.com/benidevo/vega/internal/ai/llm"
	llmMocks "github.com/benidevo/vega/internal/ai/llm/mocks"
	"github.com/benidevo/vega/internal/ai/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCVParserService_ParseCV(t *testing.T) {
	tests := []struct {
		name          string
		cvText        string
		setupMock     func(*llmMocks.MockProvider)
		expectError   bool
		errorContains string
	}{
		{
			name:   "should_parse_cv_when_content_valid",
			cvText: "John Doe\nSoftware Engineer\njohn@email.com\nGo, Python, React",
			setupMock: func(m *llmMocks.MockProvider) {
				result := models.CVParsingResult{
					IsValid: true,
					PersonalInfo: models.PersonalInfo{
						FirstName: "John",
						LastName:  "Doe",
						Email:     "john@email.com",
						Title:     "Software Engineer",
					},
					WorkExperience: []models.WorkExperience{},
					Education:      []models.Education{},
					Skills:         []string{"Go", "Python", "React"},
				}
				m.On("Generate", mock.Anything, mock.MatchedBy(func(req llm.GenerateRequest) bool {
					return req.ResponseType == llm.ResponseTypeCVParsing
				})).Return(llm.GenerateResponse{
					Data: result,
				}, nil)
			},
		},
		{
			name:   "should_return_error_when_cv_empty",
			cvText: "",
			setupMock: func(m *llmMocks.MockProvider) {
			},
			expectError:   true,
			errorContains: "validation failed",
		},
		{
			name:   "should_return_error_when_cv_too_short",
			cvText: "John",
			setupMock: func(m *llmMocks.MockProvider) {
				result := models.CVParsingResult{
					IsValid: false,
					Reason:  "CV content too short to extract meaningful information",
				}
				m.On("Generate", mock.Anything, mock.MatchedBy(func(req llm.GenerateRequest) bool {
					return req.ResponseType == llm.ResponseTypeCVParsing
				})).Return(llm.GenerateResponse{
					Data: result,
				}, nil)
			},
			expectError: false,
		},
		{
			name:   "should_return_error_when_provider_fails",
			cvText: "This is a valid CV content with enough text for parsing",
			setupMock: func(m *llmMocks.MockProvider) {
				m.On("Generate", mock.Anything, mock.MatchedBy(func(req llm.GenerateRequest) bool {
					return req.ResponseType == llm.ResponseTypeCVParsing
				})).Return(llm.GenerateResponse{}, fmt.Errorf("AI service error"))
			},
			expectError:   true,
			errorContains: "AI service error",
		},
		{
			name:   "should_return_error_when_response_invalid_type",
			cvText: "Valid CV content for testing type mismatch error case",
			setupMock: func(m *llmMocks.MockProvider) {
				m.On("Generate", mock.Anything, mock.MatchedBy(func(req llm.GenerateRequest) bool {
					return req.ResponseType == llm.ResponseTypeCVParsing
				})).Return(llm.GenerateResponse{
					Data: "invalid type",
				}, nil)
			},
			expectError:   true,
			errorContains: "unexpected response type",
		},
		{
			name:   "should_return_error_when_parsed_cv_invalid",
			cvText: "This is not a CV, it's a police report or other non-CV document",
			setupMock: func(m *llmMocks.MockProvider) {
				result := models.CVParsingResult{
					IsValid: false,
					Reason:  "Invalid document: not a CV",
				}
				m.On("Generate", mock.Anything, mock.MatchedBy(func(req llm.GenerateRequest) bool {
					return req.ResponseType == llm.ResponseTypeCVParsing
				})).Return(llm.GenerateResponse{
					Data: result,
				}, nil)
			},
			expectError: false,
		},
		{
			name:   "should_parse_cv_with_full_details_when_comprehensive",
			cvText: "Jane Smith\nSenior Backend Developer\njane.smith@email.com\n+1-555-0123\n\nExperience:\nTech Corp - Senior Developer (2020-Present)\nStartup Inc - Developer (2018-2020)\n\nEducation:\nBS Computer Science - MIT (2018)\n\nSkills: Go, Python, Kubernetes, Docker",
			setupMock: func(m *llmMocks.MockProvider) {
				result := models.CVParsingResult{
					IsValid: true,
					PersonalInfo: models.PersonalInfo{
						FirstName: "Jane",
						LastName:  "Smith",
						Email:     "jane.smith@email.com",
						Phone:     "+1-555-0123",
						Title:     "Senior Backend Developer",
					},
					WorkExperience: []models.WorkExperience{
						{
							Company:   "Tech Corp",
							Title:     "Senior Developer",
							StartDate: "2020",
							EndDate:   "Present",
						},
						{
							Company:   "Startup Inc",
							Title:     "Developer",
							StartDate: "2018",
							EndDate:   "2020",
						},
					},
					Education: []models.Education{
						{
							Institution:  "MIT",
							Degree:       "BS Computer Science",
							FieldOfStudy: "Computer Science",
							EndDate:      "2018",
						},
					},
					Skills: []string{"Go", "Python", "Kubernetes", "Docker"},
				}
				m.On("Generate", mock.Anything, mock.MatchedBy(func(req llm.GenerateRequest) bool {
					return req.ResponseType == llm.ResponseTypeCVParsing
				})).Return(llm.GenerateResponse{
					Data: result,
				}, nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockProvider := llmMocks.NewMockProvider(t)
			if tt.setupMock != nil {
				tt.setupMock(mockProvider)
			}

			service := NewCVParserService(mockProvider)
			result, err := service.ParseCV(context.Background(), tt.cvText)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, result)
			}
		})
	}
}
