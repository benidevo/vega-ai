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

func createTestRequest() models.Request {
	return models.Request{
		ApplicantName: "John Doe",
		ApplicantProfile: `Current Title: Senior Developer
Industry: Technology
Location: San Francisco
Career Summary: Experienced software engineer with 5+ years
Skills: Go, Python, JavaScript
Work Experience:
- Software Engineer at Previous Corp (2020 - Present)
  Built web applications`,
		JobDescription: `Position: Software Engineer
Company: Tech Corp

Description:
Build amazing software

Required Skills: Go, Python, Docker`,
		ExtraContext: "Additional context about experience",
	}
}

func TestJobMatcherService_AnalyzeMatch(t *testing.T) {
	tests := []struct {
		name           string
		request        models.Request
		setupMock      func(*llmMocks.MockProvider)
		expectedResult *models.MatchResult
		expectError    bool
		errorContains  string
	}{
		{
			name:    "should_match_job_when_request_valid",
			request: createTestRequest(),
			setupMock: func(m *llmMocks.MockProvider) {
				matchResult := models.MatchResult{
					MatchScore: 85,
					Strengths:  []string{"Strong Go skills", "Relevant experience"},
					Weaknesses: []string{"Limited Docker experience"},
					Highlights: []string{"Perfect cultural fit"},
					Feedback:   "Great candidate for this role",
				}

				response := llm.GenerateResponse{
					Data: matchResult,
				}

				m.On("Generate", mock.Anything, mock.MatchedBy(func(req llm.GenerateRequest) bool {
					return req.ResponseType == llm.ResponseTypeMatchResult
				})).Return(response, nil)
			},
			expectedResult: &models.MatchResult{
				MatchScore: 85,
				Strengths:  []string{"Strong Go skills", "Relevant experience"},
				Weaknesses: []string{"Limited Docker experience"},
				Highlights: []string{"Perfect cultural fit"},
				Feedback:   "Great candidate for this role",
			},
		},
		{
			name: "should_return_error_when_applicant_name_missing",
			request: models.Request{
				ApplicantName:    "", // Missing
				ApplicantProfile: "Some profile",
				JobDescription:   "Some job description",
			},
			setupMock: func(m *llmMocks.MockProvider) {
			},
			expectError:   true,
			errorContains: "validation failed",
		},
		{
			name: "should_return_error_when_applicant_profile_empty",
			request: models.Request{
				ApplicantName:    "John Doe",
				ApplicantProfile: "", // Empty
				JobDescription:   "Some job description",
			},
			setupMock: func(m *llmMocks.MockProvider) {
			},
			expectError:   true,
			errorContains: "validation failed",
		},
		{
			name: "should_return_error_when_job_description_empty",
			request: models.Request{
				ApplicantName:    "John Doe",
				ApplicantProfile: "Some profile",
				JobDescription:   "", // Empty
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
					return req.ResponseType == llm.ResponseTypeMatchResult
				})).Return(llm.GenerateResponse{}, fmt.Errorf("AI service error"))
			},
			expectError:   true,
			errorContains: "AI service error",
		},
		{
			name:    "should_return_error_when_response_invalid_type",
			request: createTestRequest(),
			setupMock: func(m *llmMocks.MockProvider) {
				m.On("Generate", mock.Anything, mock.MatchedBy(func(req llm.GenerateRequest) bool {
					return req.ResponseType == llm.ResponseTypeMatchResult
				})).Return(llm.GenerateResponse{
					Data: "invalid type",
				}, nil)
			},
			expectError:   true,
			errorContains: "unexpected response type",
		},
		{
			name: "should_analyze_match_for_data_scientist_role",
			request: models.Request{
				ApplicantName: "Jane Smith",
				ApplicantProfile: `Current Title: Data Scientist
Skills: Python, ML, SQL`,
				JobDescription: "Data Scientist role requiring ML experience",
			},
			setupMock: func(m *llmMocks.MockProvider) {
				matchResult := models.MatchResult{
					MatchScore: 80,
					Strengths:  []string{"Strong ML background"},
					Weaknesses: []string{"Limited production experience"},
					Highlights: []string{"Excellent Python skills"},
					Feedback:   "Good fit with room for growth",
				}

				m.On("Generate", mock.Anything, mock.MatchedBy(func(req llm.GenerateRequest) bool {
					return req.ResponseType == llm.ResponseTypeMatchResult
				})).Return(llm.GenerateResponse{
					Data: matchResult,
				}, nil)
			},
			expectedResult: &models.MatchResult{
				MatchScore: 80,
				Strengths:  []string{"Strong ML background"},
				Weaknesses: []string{"Limited production experience"},
				Highlights: []string{"Excellent Python skills"},
				Feedback:   "Good fit with room for growth",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockProvider := llmMocks.NewMockProvider(t)
			if tt.setupMock != nil {
				tt.setupMock(mockProvider)
			}

			service := NewJobMatcherService(mockProvider, 30*time.Second)
			result, err := service.AnalyzeMatch(context.Background(), tt.request)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.expectedResult, result)
			}
		})
	}
}

func TestJobMatcherService_GetMatchCategories(t *testing.T) {
	tests := []struct {
		name                string
		score               int
		expectedCategory    string
		expectedDescription string
	}{
		{
			name:                "should_return_excellent_when_score_90_or_above",
			score:               95,
			expectedCategory:    "Excellent Match",
			expectedDescription: "You're an ideal candidate with all key qualifications and relevant experience. Apply with confidence.",
		},
		{
			name:                "should_return_strong_when_score_80_to_89",
			score:               85,
			expectedCategory:    "Strong Match",
			expectedDescription: "You meet 80%+ of requirements with transferable skills covering gaps. Strong application potential.",
		},
		{
			name:                "should_return_good_when_score_70_to_79",
			score:               75,
			expectedCategory:    "Good Match",
			expectedDescription: "You have solid core qualifications. Address the skill gaps in your cover letter to strengthen your application.",
		},
		{
			name:                "should_return_fair_when_score_60_to_69",
			score:               65,
			expectedCategory:    "Fair Match",
			expectedDescription: "You show promise but lack some key requirements. Consider gaining experience in missing areas first.",
		},
		{
			name:                "should_return_partial_when_score_50_to_59",
			score:               55,
			expectedCategory:    "Partial Match",
			expectedDescription: "Your profile shows potential but significant gaps exist. This role may be a stretch at this time.",
		},
		{
			name:                "should_return_poor_when_score_below_50",
			score:               30,
			expectedCategory:    "Poor Match",
			expectedDescription: "Your current qualifications don't align with this role. Focus on building relevant skills and experience.",
		},
		{
			name:                "should_handle_boundary_score_90",
			score:               90,
			expectedCategory:    "Excellent Match",
			expectedDescription: "You're an ideal candidate with all key qualifications and relevant experience. Apply with confidence.",
		},
		{
			name:                "should_handle_boundary_score_80",
			score:               80,
			expectedCategory:    "Strong Match",
			expectedDescription: "You meet 80%+ of requirements with transferable skills covering gaps. Strong application potential.",
		},
		{
			name:                "should_handle_boundary_score_70",
			score:               70,
			expectedCategory:    "Good Match",
			expectedDescription: "You have solid core qualifications. Address the skill gaps in your cover letter to strengthen your application.",
		},
		{
			name:                "should_handle_boundary_score_60",
			score:               60,
			expectedCategory:    "Fair Match",
			expectedDescription: "You show promise but lack some key requirements. Consider gaining experience in missing areas first.",
		},
		{
			name:                "should_handle_boundary_score_50",
			score:               50,
			expectedCategory:    "Partial Match",
			expectedDescription: "Your profile shows potential but significant gaps exist. This role may be a stretch at this time.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockProvider := llmMocks.NewMockProvider(t)
			service := NewJobMatcherService(mockProvider, 30*time.Second)

			category, description := service.GetMatchCategories(tt.score)

			assert.Equal(t, tt.expectedCategory, category)
			assert.Equal(t, tt.expectedDescription, description)
		})
	}
}
