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

func TestCVGeneratorService_GenerateCV(t *testing.T) {
	tests := []struct {
		name          string
		request       models.Request
		setupMock     func(*llmMocks.MockProvider)
		expectError   bool
		errorContains string
	}{
		{
			name: "should_generate_cv_when_request_valid",
			request: models.Request{
				ApplicantName:    "Sarah Johnson",
				ApplicantProfile: "Senior Frontend Developer with 6+ years experience",
				JobDescription:   "Frontend Developer position requiring React skills",
			},
			setupMock: func(m *llmMocks.MockProvider) {
				validCVResult := models.CVParsingResult{
					IsValid: true,
					PersonalInfo: models.PersonalInfo{
						FirstName: "Sarah",
						LastName:  "Johnson",
						Email:     "sarah.johnson@example.com",
						Phone:     "+1234567890",
					},
					WorkExperience: []models.WorkExperience{
						{
							Title:       "Senior Frontend Developer",
							Company:     "Tech Corp",
							StartDate:   "2020",
							EndDate:     "Present",
							Description: "Led frontend development team",
						},
					},
					Education: []models.Education{
						{
							Degree:      "Bachelor of Computer Science",
							Institution: "Tech University",
							StartDate:   "2012",
							EndDate:     "2016",
						},
					},
					Skills: []string{"React", "TypeScript", "JavaScript", "CSS", "HTML"},
				}
				m.On("Generate", mock.Anything, mock.MatchedBy(func(req llm.GenerateRequest) bool {
					return req.ResponseType == llm.ResponseTypeCV
				})).Return(llm.GenerateResponse{
					Data: validCVResult,
				}, nil)
			},
		},
		{
			name: "should_return_error_when_request_empty",
			request: models.Request{
				ApplicantName:    "",
				ApplicantProfile: "",
				JobDescription:   "",
			},
			setupMock: func(m *llmMocks.MockProvider) {
			},
			expectError:   true,
			errorContains: "validation failed",
		},
		{
			name: "should_return_error_when_provider_fails",
			request: models.Request{
				ApplicantName:    "John Doe",
				ApplicantProfile: "Software Engineer",
				JobDescription:   "Developer position",
			},
			setupMock: func(m *llmMocks.MockProvider) {
				m.On("Generate", mock.Anything, mock.MatchedBy(func(req llm.GenerateRequest) bool {
					return req.ResponseType == llm.ResponseTypeCV
				})).Return(llm.GenerateResponse{}, fmt.Errorf("AI service error"))
			},
			expectError:   true,
			errorContains: "AI service error",
		},
		{
			name: "should_return_error_when_response_invalid_type",
			request: models.Request{
				ApplicantName:    "Jane Smith",
				ApplicantProfile: "Backend Developer",
				JobDescription:   "Python Developer",
			},
			setupMock: func(m *llmMocks.MockProvider) {
				m.On("Generate", mock.Anything, mock.MatchedBy(func(req llm.GenerateRequest) bool {
					return req.ResponseType == llm.ResponseTypeCV
				})).Return(llm.GenerateResponse{
					Data: "invalid type",
				}, nil)
			},
			expectError:   true,
			errorContains: "unexpected response type",
		},
		{
			name: "should_return_error_when_generated_cv_invalid",
			request: models.Request{
				ApplicantName:    "Bob Wilson",
				ApplicantProfile: "Full Stack Developer",
				JobDescription:   "Web Developer",
			},
			setupMock: func(m *llmMocks.MockProvider) {
				invalidCVResult := models.CVParsingResult{
					IsValid: false,
					Reason:  "Failed to generate valid CV structure",
				}
				m.On("Generate", mock.Anything, mock.MatchedBy(func(req llm.GenerateRequest) bool {
					return req.ResponseType == llm.ResponseTypeCV
				})).Return(llm.GenerateResponse{
					Data: invalidCVResult,
				}, nil)
			},
			expectError:   true,
			errorContains: "Failed to generate valid CV structure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockProvider := llmMocks.NewMockProvider(t)
			if tt.setupMock != nil {
				tt.setupMock(mockProvider)
			}

			service := NewCVGeneratorService(mockProvider)
			result, err := service.GenerateCV(context.Background(), tt.request, 1, "Test Job")

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, result)
				assert.True(t, result.IsValid)
				assert.NotEmpty(t, result.PersonalInfo.FirstName)
			}
		})
	}
}
