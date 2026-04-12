package services

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/benidevo/vega/internal/ai/llm"
	llmMocks "github.com/benidevo/vega/internal/ai/llm/mocks"
	"github.com/benidevo/vega/internal/ai/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCoverLetterGeneratorService_GenerateCoverLetter(t *testing.T) {
	tests := []struct {
		name          string
		request       models.Request
		setupMock     func(*llmMocks.MockProvider)
		expectError   bool
		errorContains string
	}{
		{
			name:    "should_generate_cover_letter_when_request_valid",
			request: createTestRequest(),
			setupMock: func(m *llmMocks.MockProvider) {
				coverLetter := models.CoverLetter{
					Content: "Dear Hiring Manager,\n\nI am writing to express my interest...",
					Format:  models.CoverLetterTypePlainText,
				}

				response := llm.GenerateResponse{
					Data: coverLetter,
				}

				m.On("Generate", mock.Anything, mock.MatchedBy(func(req llm.GenerateRequest) bool {
					return req.ResponseType == llm.ResponseTypeCoverLetter
				})).Return(response, nil)
			},
		},
		{
			name: "should_return_error_when_applicant_name_missing",
			request: models.Request{
				ApplicantName:    "",
				ApplicantProfile: "Some profile",
				JobDescription:   "Some job description",
			},
			setupMock: func(m *llmMocks.MockProvider) {
			},
			expectError:   true,
			errorContains: "validation failed",
		},
		{
			name: "should_return_error_when_applicant_profile_missing",
			request: models.Request{
				ApplicantName:    "John Doe",
				ApplicantProfile: "",
				JobDescription:   "Some job description",
			},
			setupMock: func(m *llmMocks.MockProvider) {
			},
			expectError:   true,
			errorContains: "validation failed",
		},
		{
			name: "should_return_error_when_job_description_missing",
			request: models.Request{
				ApplicantName:    "John Doe",
				ApplicantProfile: "Some profile",
				JobDescription:   "",
			},
			setupMock: func(m *llmMocks.MockProvider) {
			},
			expectError:   true,
			errorContains: "validation failed",
		},
		{
			name:    "should_return_error_when_provider_fails",
			request: createTestRequest(),
			setupMock: func(m *llmMocks.MockProvider) {
				m.On("Generate", mock.Anything, mock.MatchedBy(func(req llm.GenerateRequest) bool {
					return req.ResponseType == llm.ResponseTypeCoverLetter
				})).Return(llm.GenerateResponse{}, fmt.Errorf("AI service error"))
			},
			expectError:   true,
			errorContains: "AI service error",
		},
		{
			name:    "should_return_error_when_response_invalid_type",
			request: createTestRequest(),
			setupMock: func(m *llmMocks.MockProvider) {
				response := llm.GenerateResponse{
					Data: "invalid type",
				}

				m.On("Generate", mock.Anything, mock.MatchedBy(func(req llm.GenerateRequest) bool {
					return req.ResponseType == llm.ResponseTypeCoverLetter
				})).Return(response, nil)
			},
			expectError:   true,
			errorContains: "unexpected response type",
		},
		{
			name: "should_generate_html_format_when_configured",
			request: models.Request{
				ApplicantName:    "Jane Smith",
				ApplicantProfile: "Backend Developer with Go expertise",
				JobDescription:   "Go Developer position",
				ExtraContext:     "Focus on microservices experience",
			},
			setupMock: func(m *llmMocks.MockProvider) {
				coverLetter := models.CoverLetter{
					Content: "<p>Dear Hiring Manager,</p><p>I am excited to apply...</p>",
					Format:  models.CoverLetterTypeHtml,
				}

				m.On("Generate", mock.Anything, mock.MatchedBy(func(req llm.GenerateRequest) bool {
					return req.ResponseType == llm.ResponseTypeCoverLetter
				})).Return(llm.GenerateResponse{
					Data: coverLetter,
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

			service := NewCoverLetterGeneratorService(mockProvider, 30*time.Second)
			result, err := service.GenerateCoverLetter(context.Background(), tt.request)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, result)
				assert.NotEmpty(t, result.Content)
			}
		})
	}
}
