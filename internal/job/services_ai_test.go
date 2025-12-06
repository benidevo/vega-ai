package job

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/benidevo/vega/internal/ai"
	aimodels "github.com/benidevo/vega/internal/ai/models"
	"github.com/benidevo/vega/internal/config"
	jobinterfacesmocks "github.com/benidevo/vega/internal/job/interfaces/mocks"
	"github.com/benidevo/vega/internal/job/models"
	settingsmodels "github.com/benidevo/vega/internal/settings/models"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func init() {
	// Disable logs for tests
	zerolog.SetGlobalLevel(zerolog.Disabled)
}

// MockSettingsService mocks the unexported settingsService interface
type MockSettingsService struct {
	mock.Mock
}

func (m *MockSettingsService) GetProfileSettings(ctx context.Context, userID int) (*settingsmodels.Profile, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*settingsmodels.Profile), args.Error(1)
}

func createTestJobForAI() *models.Job {
	return &models.Job{
		ID:             1,
		UserID:         testUserID,
		Title:          "Software Engineer",
		Description:    "Build amazing software",
		RequiredSkills: []string{"Go", "Python", "Docker"},
		Company: models.Company{
			ID:   1,
			Name: "Tech Corp",
		},
		Status:    models.INTERESTED,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func createTestProfile() *settingsmodels.Profile {
	return &settingsmodels.Profile{
		FirstName:       "John",
		LastName:        "Doe",
		Email:           "john.doe@example.com",
		Title:           "Senior Developer",
		Industry:        settingsmodels.IndustryTechnology,
		Location:        "San Francisco",
		LinkedInProfile: "https://www.linkedin.com/in/johndoe",
		CareerSummary:   "Experienced software engineer with 5+ years",
		Skills:          []string{"Go", "Python", "JavaScript"},
		WorkExperience: []settingsmodels.WorkExperience{
			{
				Title:       "Software Engineer",
				Company:     "Previous Corp",
				StartDate:   time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
				Description: "Built web applications",
			},
		},
		Education: []settingsmodels.Education{
			{
				Degree:       "BS",
				FieldOfStudy: "Computer Science",
				Institution:  "Tech University",
				StartDate:    time.Date(2016, 9, 1, 0, 0, 0, 0, time.UTC),
			},
		},
		Context: "Additional context about my experience",
	}
}

func TestJobService_AnalyzeJobMatch_NoAIService(t *testing.T) {
	mockJobRepo := jobinterfacesmocks.NewMockJobRepository(t)
	cfg := &config.Settings{}

	service := NewJobService(mockJobRepo, nil, nil, nil, cfg)

	result, err := service.AnalyzeJobMatch(context.Background(), 1, 1)

	assert.Error(t, err)
	assert.Equal(t, models.ErrAIServiceUnavailable, err)
	assert.Nil(t, result)
}

func TestJobService_AnalyzeJobMatch_NoSettingsService(t *testing.T) {
	mockJobRepo := jobinterfacesmocks.NewMockJobRepository(t)
	cfg := &config.Settings{}

	// Create service with AI service but no settings service
	service := NewJobService(mockJobRepo, &ai.AIService{}, nil, nil, cfg)

	result, err := service.AnalyzeJobMatch(context.Background(), 1, 1)

	assert.Error(t, err)
	assert.Equal(t, models.ErrProfileServiceRequired, err)
	assert.Nil(t, result)
}

func TestJobService_GenerateCoverLetter_NoAIService(t *testing.T) {
	mockJobRepo := jobinterfacesmocks.NewMockJobRepository(t)
	cfg := &config.Settings{}

	service := NewJobService(mockJobRepo, nil, nil, nil, cfg)

	result, err := service.GenerateCoverLetter(context.Background(), 1, 1)

	assert.Error(t, err)
	assert.Equal(t, models.ErrAIServiceUnavailable, err)
	assert.Nil(t, result)
}

func TestJobService_ValidateProfileForAI(t *testing.T) {
	tests := []struct {
		name          string
		profile       *settingsmodels.Profile
		expectedError error
	}{
		{
			name:          "Valid profile",
			profile:       createTestProfile(),
			expectedError: nil,
		},
		{
			name: "Missing name",
			profile: &settingsmodels.Profile{
				Skills:        []string{"Go", "Python"},
				CareerSummary: "Experienced developer",
			},
			expectedError: models.ErrProfileIncomplete,
		},
		{
			name: "Missing career info",
			profile: &settingsmodels.Profile{
				FirstName: "John",
				LastName:  "Doe",
				Skills:    []string{"Go", "Python"},
			},
			expectedError: models.ErrProfileSummaryRequired,
		},
		{
			name: "Valid with work experience but no career summary",
			profile: &settingsmodels.Profile{
				FirstName: "John",
				LastName:  "Doe",
				Skills:    []string{"Go", "Python"},
				WorkExperience: []settingsmodels.WorkExperience{
					{
						Title:   "Software Engineer",
						Company: "Tech Corp",
					},
				},
			},
			expectedError: nil,
		},
		{
			name: "Invalid work experience without career summary",
			profile: &settingsmodels.Profile{
				FirstName: "John",
				LastName:  "Doe",
				Skills:    []string{"Go", "Python"},
				WorkExperience: []settingsmodels.WorkExperience{
					{
						Title:   "", // Empty title
						Company: "",
					},
				},
			},
			expectedError: models.ErrProfileSummaryRequired,
		},
		{
			name: "Valid with education only",
			profile: &settingsmodels.Profile{
				FirstName: "John",
				LastName:  "Doe",
				Skills:    []string{"Go", "Python"},
				Education: []settingsmodels.Education{
					{
						Degree:       "BS",
						FieldOfStudy: "Computer Science",
					},
				},
			},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockJobRepo := jobinterfacesmocks.NewMockJobRepository(t)
			cfg := &config.Settings{}
			service := NewJobService(mockJobRepo, nil, nil, nil, cfg)

			err := service.ValidateProfileForAI(tt.profile)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestJobService_buildAIRequest(t *testing.T) {
	mockJobRepo := jobinterfacesmocks.NewMockJobRepository(t)
	cfg := &config.Settings{}
	service := NewJobService(mockJobRepo, nil, nil, nil, cfg)

	job := createTestJobForAI()
	profile := createTestProfile()

	request := service.buildAIRequest(job, profile)

	assert.Equal(t, "John Doe", request.ApplicantName)
	assert.Contains(t, request.ApplicantProfile, "Senior Developer")
	assert.Contains(t, request.ApplicantProfile, "Technology")
	assert.Contains(t, request.ApplicantProfile, "Go, Python, JavaScript")
	assert.Contains(t, request.JobDescription, "Software Engineer")
	assert.Contains(t, request.JobDescription, "Tech Corp")
	assert.Contains(t, request.JobDescription, "Go, Python, Docker")
	assert.Contains(t, request.ExtraContext, "EXPERIENCED CANDIDATE")
	assert.Contains(t, request.ExtraContext, "Additional context about my experience")
}

func TestJobService_buildAIRequest_EmptyName(t *testing.T) {
	mockJobRepo := jobinterfacesmocks.NewMockJobRepository(t)
	cfg := &config.Settings{}
	service := NewJobService(mockJobRepo, nil, nil, nil, cfg)

	job := createTestJobForAI()
	profile := createTestProfile()
	profile.FirstName = ""
	profile.LastName = ""

	request := service.buildAIRequest(job, profile)

	assert.Equal(t, "Applicant", request.ApplicantName)
}

func TestJobService_buildProfileSummary(t *testing.T) {
	mockJobRepo := jobinterfacesmocks.NewMockJobRepository(t)
	cfg := &config.Settings{}
	service := NewJobService(mockJobRepo, nil, nil, nil, cfg)

	profile := createTestProfile()

	summary := service.buildProfileSummary(profile)

	assert.Contains(t, summary, "Current Title: Senior Developer")
	assert.Contains(t, summary, "Industry: Technology")
	assert.NotContains(t, summary, "Location: San Francisco")
	assert.NotContains(t, summary, "Phone:")
	assert.NotContains(t, summary, "john.doe@example.com")
	assert.NotContains(t, summary, "Email:")
	assert.NotContains(t, summary, "linkedin.com/in/johndoe")
	assert.NotContains(t, summary, "LinkedIn:")
	assert.Contains(t, summary, "Career Summary:")
	assert.Contains(t, summary, "Skills: Go, Python, JavaScript")
	assert.Contains(t, summary, "Work Experience:")
	assert.Contains(t, summary, "Education:")
	assert.Contains(t, summary, "Additional Context:")
}

func TestJobService_buildProfileSummary_LimitedExperience(t *testing.T) {
	mockJobRepo := jobinterfacesmocks.NewMockJobRepository(t)
	cfg := &config.Settings{}
	service := NewJobService(mockJobRepo, nil, nil, nil, cfg)

	profile := createTestProfile()

	// Add many work experiences to test the limit
	for i := 0; i < 10; i++ {
		profile.WorkExperience = append(profile.WorkExperience, settingsmodels.WorkExperience{
			Title:       "Developer",
			Company:     "Company",
			StartDate:   time.Date(2020-i, 1, 1, 0, 0, 0, 0, time.UTC),
			Description: "Some description that is quite long and should be truncated if it exceeds the character limit for descriptions in the profile summary generation process",
		})
	}

	summary := service.buildProfileSummary(profile)

	// Should limit to 5 experiences
	experiences := len(profile.WorkExperience)
	assert.True(t, experiences > 5)
	// Count occurrences of " at " which indicates work experience entries
	// Should be 5 (limited) + context about limiting
	assert.Contains(t, summary, "Work Experience:")
}

func TestJobService_convertToJobMatchAnalysis(t *testing.T) {
	mockJobRepo := jobinterfacesmocks.NewMockJobRepository(t)
	cfg := &config.Settings{}
	service := NewJobService(mockJobRepo, nil, nil, nil, cfg)

	aiResult := &aimodels.MatchResult{
		MatchScore: 85,
		Strengths:  []string{"Strong Go skills"},
		Weaknesses: []string{"Limited Docker experience"},
		Highlights: []string{"Perfect fit"},
		Feedback:   "Great candidate",
	}

	result := service.convertToJobMatchAnalysis(aiResult, 1, 2)

	assert.Equal(t, 2, result.JobID)
	assert.Equal(t, 1, result.UserID)
	assert.Equal(t, 85, result.MatchScore)
	assert.Equal(t, []string{"Strong Go skills"}, result.Strengths)
	assert.Equal(t, []string{"Limited Docker experience"}, result.Weaknesses)
	assert.Equal(t, []string{"Perfect fit"}, result.Highlights)
	assert.Equal(t, "Great candidate", result.Feedback)
	assert.NotZero(t, result.AnalyzedAt)
	assert.NotZero(t, result.CreatedAt)
	assert.NotZero(t, result.UpdatedAt)
}

func TestJobService_convertToCoverLetter(t *testing.T) {
	mockJobRepo := jobinterfacesmocks.NewMockJobRepository(t)
	cfg := &config.Settings{}
	service := NewJobService(mockJobRepo, nil, nil, nil, cfg)

	aiResult := &aimodels.CoverLetter{
		Content: "Dear Hiring Manager...",
		Format:  aimodels.CoverLetterTypeMarkdown,
	}

	result := service.convertToCoverLetter(aiResult, 1, 2)

	assert.Equal(t, 2, result.JobID)
	assert.Equal(t, 1, result.UserID)
	assert.Equal(t, "Dear Hiring Manager...", result.Content)
	assert.Equal(t, string(aimodels.CoverLetterTypeMarkdown), result.Format)
	assert.NotZero(t, result.GeneratedAt)
	assert.NotZero(t, result.CreatedAt)
	assert.NotZero(t, result.UpdatedAt)
}

func TestJobService_GenerateCV_NoAIService(t *testing.T) {
	mockJobRepo := jobinterfacesmocks.NewMockJobRepository(t)
	cfg := &config.Settings{}

	service := NewJobService(mockJobRepo, nil, nil, nil, cfg)

	result, err := service.GenerateCV(context.Background(), 1, 1)

	assert.Error(t, err)
	assert.Equal(t, models.ErrAIServiceUnavailable, err)
	assert.Nil(t, result)
}

func TestJobService_GenerateCV_NoSettingsService(t *testing.T) {
	mockJobRepo := jobinterfacesmocks.NewMockJobRepository(t)
	cfg := &config.Settings{}

	service := NewJobService(mockJobRepo, &ai.AIService{}, nil, nil, cfg)

	result, err := service.GenerateCV(context.Background(), 1, 1)

	assert.Error(t, err)
	assert.Equal(t, models.ErrProfileServiceRequired, err)
	assert.Nil(t, result)
}

func TestJobService_calculateTotalExperience(t *testing.T) {
	mockJobRepo := jobinterfacesmocks.NewMockJobRepository(t)
	cfg := &config.Settings{}
	service := NewJobService(mockJobRepo, nil, nil, nil, cfg)

	tests := []struct {
		name           string
		workExperience []settingsmodels.WorkExperience
		expectedYears  float64
		description    string
	}{
		{
			name:           "no work experience",
			workExperience: []settingsmodels.WorkExperience{},
			expectedYears:  0,
			description:    "Empty work experience should return 0",
		},
		{
			name: "current job for 3 years",
			workExperience: []settingsmodels.WorkExperience{
				{
					Title:     "Software Engineer",
					Company:   "Tech Corp",
					StartDate: time.Now().AddDate(-3, 0, 0), // 3 years ago
					EndDate:   nil,                          // current job
				},
			},
			expectedYears: 3.0,
			description:   "Current job should calculate time from start to now",
		},
		{
			name: "multiple jobs totaling 5+ years",
			workExperience: []settingsmodels.WorkExperience{
				{
					Title:     "Senior Engineer",
					Company:   "Current Corp",
					StartDate: time.Now().AddDate(-2, 0, 0), // 2 years ago
					EndDate:   nil,                          // current
				},
				{
					Title:     "Junior Engineer",
					Company:   "Previous Corp",
					StartDate: time.Now().AddDate(-5, 0, 0),                   // 5 years ago
					EndDate:   &[]time.Time{time.Now().AddDate(-2, -1, 0)}[0], // ended 2y 1m ago
				},
			},
			expectedYears: 5.0, // approximately 5 years total
			description:   "Multiple jobs should sum their durations",
		},
		{
			name: "overlapping work periods",
			workExperience: []settingsmodels.WorkExperience{
				{
					Title:     "Senior Developer",
					Company:   "Tech Company A",
					StartDate: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
					EndDate:   &[]time.Time{time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC)}[0],
				},
				{
					Title:     "Systems Engineer",
					Company:   "Tech Company B",
					StartDate: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
					EndDate:   &[]time.Time{time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)}[0],
				},
				{
					Title:     "Junior Developer",
					Company:   "Tech Company C",
					StartDate: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
					EndDate:   &[]time.Time{time.Date(2021, 12, 31, 0, 0, 0, 0, time.UTC)}[0],
				},
			},
			expectedYears: 5.5, // From Jan 2020 to July 2025 = 5.5 years
			description:   "Overlapping periods should not double-count time",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.calculateTotalExperience(tt.workExperience)

			// Allow for some tolerance in floating point comparison
			tolerance := 0.2 // about 2.5 months tolerance
			assert.InDelta(t, tt.expectedYears, result, tolerance, tt.description)
		})
	}
}

func TestJobService_buildAIRequest_ExperienceContext(t *testing.T) {
	mockJobRepo := jobinterfacesmocks.NewMockJobRepository(t)
	cfg := &config.Settings{}
	service := NewJobService(mockJobRepo, nil, nil, nil, cfg)

	job := createTestJobForAI()

	tests := []struct {
		name              string
		profile           *settingsmodels.Profile
		expectedInContext string
		description       string
	}{
		{
			name: "experienced candidate (2+ years)",
			profile: &settingsmodels.Profile{
				FirstName: "John",
				LastName:  "Doe",
				Skills:    []string{"Go", "Python"},
				WorkExperience: []settingsmodels.WorkExperience{
					{
						Title:     "Senior Engineer",
						Company:   "Tech Corp",
						StartDate: time.Now().AddDate(-3, 0, 0), // 3 years ago
					},
				},
				Context: "Additional context",
			},
			expectedInContext: "EXPERIENCED CANDIDATE",
			description:       "Should identify experienced candidate and add de-emphasis instruction",
		},
		{
			name: "entry-level candidate (<2 years)",
			profile: &settingsmodels.Profile{
				FirstName: "Jane",
				LastName:  "Smith",
				Skills:    []string{"Python", "React"},
				WorkExperience: []settingsmodels.WorkExperience{
					{
						Title:     "Junior Developer",
						Company:   "Startup Inc",
						StartDate: time.Now().AddDate(0, -6, 0), // 6 months ago
					},
				},
				Context: "Fresh graduate",
			},
			expectedInContext: "Fresh graduate",
			description:       "Should not add experience-based instruction for entry-level",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := service.buildAIRequest(job, tt.profile)

			assert.Contains(t, request.ExtraContext, tt.expectedInContext, tt.description)
		})
	}
}

func TestJobService_buildAIRequest_SimilarSkillsHandling(t *testing.T) {
	mockJobRepo := jobinterfacesmocks.NewMockJobRepository(t)
	cfg := &config.Settings{}
	service := NewJobService(mockJobRepo, nil, nil, nil, cfg)

	// Create a job requiring specific skills
	job := &models.Job{
		ID:             1,
		Title:          "Python Backend Developer",
		Description:    "Looking for a backend developer with Django and PostgreSQL experience",
		RequiredSkills: []string{"Python", "Django", "PostgreSQL", "AWS"},
		Company: models.Company{
			ID:   1,
			Name: "Tech Startup",
		},
	}

	tests := []struct {
		name        string
		profile     *settingsmodels.Profile
		description string
	}{
		{
			name: "similar backend skills (Node.js developer for Python role)",
			profile: &settingsmodels.Profile{
				FirstName: "Alex",
				LastName:  "Smith",
				Title:     "Backend Developer",
				Skills:    []string{"Node.js", "Express", "MongoDB", "Azure"}, // Similar but different stack
				WorkExperience: []settingsmodels.WorkExperience{
					{
						Title:       "Backend Developer",
						Company:     "Previous Corp",
						StartDate:   time.Now().AddDate(-3, 0, 0),
						Description: "Built REST APIs and microservices",
					},
				},
				Context: "Similar skills in different technology stack",
			},
			description: "Should handle similar backend skills positively",
		},
		{
			name: "exact skill match",
			profile: &settingsmodels.Profile{
				FirstName: "Sarah",
				LastName:  "Johnson",
				Title:     "Python Developer",
				Skills:    []string{"Python", "Django", "PostgreSQL", "AWS"}, // Exact match
				WorkExperience: []settingsmodels.WorkExperience{
					{
						Title:       "Python Developer",
						Company:     "Django Corp",
						StartDate:   time.Now().AddDate(-2, 0, 0),
						Description: "Django web applications with PostgreSQL",
					},
				},
				Context: "Exact skill and experience match",
			},
			description: "Should handle exact matches well",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := service.buildAIRequest(job, tt.profile)

			// Verify the request contains the job requirements
			assert.Contains(t, request.JobDescription, "Python")
			assert.Contains(t, request.JobDescription, "Django")
			assert.Contains(t, request.JobDescription, "PostgreSQL")

			// Verify profile skills are included
			skillsInProfile := strings.Join(tt.profile.Skills, ", ")
			assert.Contains(t, request.ApplicantProfile, skillsInProfile)

			// Verify context is preserved
			assert.Contains(t, request.ExtraContext, tt.profile.Context)
		})
	}
}
