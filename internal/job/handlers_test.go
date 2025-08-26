package job

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/benidevo/vega/internal/common/alerts"
	"github.com/benidevo/vega/internal/common/testutil"
	"github.com/benidevo/vega/internal/config"
	"github.com/benidevo/vega/internal/job/models"
	"github.com/benidevo/vega/internal/quota"
	settingsmodels "github.com/benidevo/vega/internal/settings/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
)

// mockJobService implements the jobService interface for testing
type mockJobService struct {
	mock.Mock
}

func (m *mockJobService) CreateJob(ctx context.Context, userID int, title, description, companyName string, options ...models.JobOption) (*models.Job, bool, error) {
	// Handle variadic options
	args := []interface{}{ctx, userID, title, description, companyName}
	for _, opt := range options {
		args = append(args, opt)
	}
	called := m.Called(args...)
	if called.Get(0) == nil {
		return nil, called.Bool(1), called.Error(2)
	}
	return called.Get(0).(*models.Job), called.Bool(1), called.Error(2)
}

func (m *mockJobService) GetJob(ctx context.Context, userID int, jobID int) (*models.Job, error) {
	args := m.Called(ctx, userID, jobID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Job), args.Error(1)
}

func (m *mockJobService) GetJobsWithPagination(ctx context.Context, userID int, filter models.JobFilter) (*models.JobsWithPagination, error) {
	args := m.Called(ctx, userID, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.JobsWithPagination), args.Error(1)
}

func (m *mockJobService) UpdateJob(ctx context.Context, userID int, job *models.Job) error {
	args := m.Called(ctx, userID, job)
	return args.Error(0)
}

func (m *mockJobService) DeleteJob(ctx context.Context, userID int, jobID int) error {
	args := m.Called(ctx, userID, jobID)
	return args.Error(0)
}

func (m *mockJobService) ValidateJobIDFormat(jobIDStr string) (int, error) {
	args := m.Called(jobIDStr)
	return args.Int(0), args.Error(1)
}

func (m *mockJobService) ValidateURL(url string) error {
	args := m.Called(url)
	return args.Error(0)
}

func (m *mockJobService) ValidateAndFilterSkills(skillsStr string) []string {
	args := m.Called(skillsStr)
	return args.Get(0).([]string)
}

func (m *mockJobService) ValidateFieldName(field string) error {
	args := m.Called(field)
	return args.Error(0)
}

func (m *mockJobService) ValidateProfileForAI(profile *settingsmodels.Profile) error {
	args := m.Called(profile)
	return args.Error(0)
}

func (m *mockJobService) AnalyzeJobMatch(ctx context.Context, userID int, jobID int) (*models.JobMatchAnalysis, error) {
	args := m.Called(ctx, userID, jobID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.JobMatchAnalysis), args.Error(1)
}

func (m *mockJobService) GenerateCoverLetter(ctx context.Context, userID int, jobID int) (*models.CoverLetterWithProfile, error) {
	args := m.Called(ctx, userID, jobID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.CoverLetterWithProfile), args.Error(1)
}

func (m *mockJobService) GenerateCV(ctx context.Context, userID int, jobID int) (*models.GeneratedCV, error) {
	args := m.Called(ctx, userID, jobID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.GeneratedCV), args.Error(1)
}

func (m *mockJobService) CheckJobQuota(ctx context.Context, userID int, jobID int) (*quota.QuotaCheckResult, error) {
	args := m.Called(ctx, userID, jobID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*quota.QuotaCheckResult), args.Error(1)
}

func (m *mockJobService) GetJobMatchHistory(ctx context.Context, userID int, jobID int) ([]*models.MatchResult, error) {
	args := m.Called(ctx, userID, jobID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.MatchResult), args.Error(1)
}

func (m *mockJobService) DeleteMatchResult(ctx context.Context, userID int, jobID int, matchID int) error {
	args := m.Called(ctx, userID, jobID, matchID)
	return args.Error(0)
}

func (m *mockJobService) LogError(err error) {
	m.Called(err)
}

// mockCommandFactory implements the commandFactory interface for testing
type mockCommandFactory struct {
	mock.Mock
}

func (m *mockCommandFactory) CreateAnalyzeJobCommand(jobID int) interface{} {
	args := m.Called(jobID)
	return args.Get(0)
}

func (m *mockCommandFactory) CreateGenerateCoverLetterCommand(jobID int) interface{} {
	args := m.Called(jobID)
	return args.Get(0)
}

func (m *mockCommandFactory) GetCommand(field string) (FieldCommand, error) {
	args := m.Called(field)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(FieldCommand), args.Error(1)
}

// mockSettingsService implements the settingsService interface for testing
type mockSettingsService struct {
	mock.Mock
}

func (m *mockSettingsService) GetProfileSettings(ctx context.Context, userID int) (*settingsmodels.Profile, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*settingsmodels.Profile), args.Error(1)
}

func setupTestJobHandler() (*JobHandler, *mockJobService, *mockCommandFactory, *gin.Engine) {
	mockService := new(mockJobService)
	mockCommandFactory := new(mockCommandFactory)
	mockSettingsService := new(mockSettingsService)
	cfg := &config.Settings{
		IsTest: true,
	}

	handler := &JobHandler{
		service:         mockService,
		cfg:             cfg,
		commandFactory:  mockCommandFactory,
		renderer:        nil, // Will be mocked in tests if needed
		settingsService: mockSettingsService,
	}

	router := testutil.SetupTestRouter()
	return handler, mockService, mockCommandFactory, router
}

// Helper function to set user context
func setUserContext(c *gin.Context, userID int) {
	c.Set("userID", userID)
}

// Helper function to set job context
func setJobContext(c *gin.Context, userID int, jobID int) {
	c.Set("userID", userID)
	c.Set("jobID", jobID)
}

func TestJobHandler_CreateJob(t *testing.T) {
	handler, mockService, _, router := setupTestJobHandler()

	// Setup routes with middleware to set user context
	router.POST("/jobs", func(c *gin.Context) {
		setUserContext(c, 1)
		handler.CreateJob(c)
	})

	tests := []testutil.HandlerTestCase{
		{
			Name:   "should_create_job_when_data_valid",
			Method: "POST",
			Path:   "/jobs",
			FormData: map[string]string{
				"title":        "Software Engineer",
				"description":  "Build awesome software",
				"company_name": "Acme Corp",
				"location":     "Remote",
				"job_type":     "1", // FULL_TIME
				"source_url":   "https://example.com/job",
				"status":       "interested",
			},
			Headers: map[string]string{
				"HX-Request": "true",
			},
			MockSetup: func() {
				mockService.On("ValidateURL", "https://example.com/job").Return(nil)
				mockService.On("ValidateURL", "").Return(nil)
				mockService.On("ValidateAndFilterSkills", "").Return([]string{})
				job := &models.Job{
					ID:          1,
					Title:       "Software Engineer",
					Description: "Build awesome software",
					Company:     models.Company{Name: "Acme Corp"},
					Location:    "Remote",
					JobType:     models.FULL_TIME,
					SourceURL:   "https://example.com/job",
					Status:      models.INTERESTED,
				}
				// For CreateJob with options, we match the context and basic params, then use MatchedBy for variadic options
				mockService.On("CreateJob", mock.Anything, 1, "Software Engineer", "Build awesome software", "Acme Corp", mock.AnythingOfType("models.JobOption"), mock.AnythingOfType("models.JobOption"), mock.AnythingOfType("models.JobOption"), mock.AnythingOfType("models.JobOption"), mock.AnythingOfType("models.JobOption")).
					Return(job, true, nil)
			},
			ExpectedStatus: http.StatusOK,
			ExpectedHeader: map[string]string{
				"HX-Redirect": "/jobs/1/details",
			},
			ExpectedToast: &testutil.ToastAssertion{
				Message: "Job added successfully!",
				Type:    string(alerts.TypeSuccess),
			},
		},
		{
			Name:   "should_return_error_when_title_missing",
			Method: "POST",
			Path:   "/jobs",
			FormData: map[string]string{
				"title":        "",
				"description":  "Build awesome software",
				"company_name": "Acme Corp",
				"status":       "interested",
			},
			MockSetup: func() {
				mockService.On("ValidateURL", "").Return(nil).Times(2)
				mockService.On("ValidateAndFilterSkills", "").Return([]string{})
				mockService.On("CreateJob", mock.Anything, 1, "", "Build awesome software", "Acme Corp", mock.Anything, mock.Anything, mock.Anything).
					Return(nil, false, models.ErrJobTitleRequired)
			},
			ExpectedStatus: http.StatusBadRequest,
			ExpectedToast: &testutil.ToastAssertion{
				Message: models.ErrJobTitleRequired.Error(),
				Type:    string(alerts.TypeError),
			},
		},
		{
			Name:   "should_return_error_when_description_missing",
			Method: "POST",
			Path:   "/jobs",
			FormData: map[string]string{
				"title":        "Software Engineer",
				"description":  "",
				"company_name": "Acme Corp",
				"status":       "interested",
			},
			MockSetup: func() {
				mockService.On("ValidateURL", "").Return(nil).Times(2)
				mockService.On("ValidateAndFilterSkills", "").Return([]string{})
				mockService.On("CreateJob", mock.Anything, 1, "Software Engineer", "", "Acme Corp", mock.Anything, mock.Anything, mock.Anything).
					Return(nil, false, models.ErrJobDescriptionRequired)
			},
			ExpectedStatus: http.StatusBadRequest,
			ExpectedToast: &testutil.ToastAssertion{
				Message: models.ErrJobDescriptionRequired.Error(),
				Type:    string(alerts.TypeError),
			},
		},
		{
			Name:   "should_return_error_when_company_missing",
			Method: "POST",
			Path:   "/jobs",
			FormData: map[string]string{
				"title":        "Software Engineer",
				"description":  "Build awesome software",
				"company_name": "",
				"status":       "interested",
			},
			MockSetup: func() {
				mockService.On("ValidateURL", "").Return(nil).Times(2)
				mockService.On("ValidateAndFilterSkills", "").Return([]string{})
				mockService.On("CreateJob", mock.Anything, 1, "Software Engineer", "Build awesome software", "", mock.Anything, mock.Anything, mock.Anything).
					Return(nil, false, models.ErrCompanyNameRequired)
			},
			ExpectedStatus: http.StatusBadRequest,
			ExpectedToast: &testutil.ToastAssertion{
				Message: models.ErrCompanyNameRequired.Error(),
				Type:    string(alerts.TypeError),
			},
		},
		{
			Name:   "should_validate_source_url_when_provided",
			Method: "POST",
			Path:   "/jobs",
			FormData: map[string]string{
				"title":        "Software Engineer",
				"description":  "Build awesome software",
				"company_name": "Acme Corp",
				"source_url":   "javascript:alert('XSS')",
				"status":       "interested",
			},
			MockSetup: func() {
				mockService.On("ValidateURL", "javascript:alert('XSS')").
					Return(models.ErrInvalidURLFormat)
			},
			ExpectedStatus: http.StatusBadRequest,
			ExpectedToast: &testutil.ToastAssertion{
				Message: models.ErrInvalidURLFormat.Error(),
				Type:    string(alerts.TypeError),
			},
		},
		{
			Name:   "should_handle_duplicate_job_gracefully",
			Method: "POST",
			Path:   "/jobs",
			FormData: map[string]string{
				"title":        "Software Engineer",
				"description":  "Build awesome software",
				"company_name": "Acme Corp",
				"status":       "interested",
			},
			MockSetup: func() {
				mockService.On("ValidateURL", "").Return(nil).Times(2)
				mockService.On("ValidateAndFilterSkills", "").Return([]string{})
				job := &models.Job{
					ID:    1,
					Title: "Software Engineer",
				}
				mockService.On("CreateJob", mock.Anything, 1, "Software Engineer", "Build awesome software", "Acme Corp", mock.Anything, mock.Anything, mock.Anything).
					Return(job, false, nil)
			},
			ExpectedStatus: http.StatusOK,
			ExpectedHeader: map[string]string{
				"HX-Redirect": "/jobs/1/details",
			},
			ExpectedToast: &testutil.ToastAssertion{
				Message: "Job already exists in your list",
				Type:    string(alerts.TypeInfo),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			mockService.ExpectedCalls = nil
			mockService.Calls = nil
			testutil.RunHandlerTest(t, router, tc)
			mockService.AssertExpectations(t)
		})
	}
}

// UpdateJobStatus method is not implemented in JobHandler
// The handler uses UpdateJobField for all field updates including status
func TestJobHandler_UpdateJobStatus(t *testing.T) {
	t.Skip("UpdateJobStatus method not implemented - use UpdateJobField instead")
}
func TestJobHandler_DeleteJob(t *testing.T) {
	handler, mockService, _, router := setupTestJobHandler()

	// Setup routes with middleware to set user and job context
	router.DELETE("/jobs/:id", func(c *gin.Context) {
		jobIDStr := c.Param("id")
		jobID, _ := strconv.Atoi(jobIDStr)
		setJobContext(c, 1, jobID)
		handler.DeleteJob(c)
	})

	tests := []testutil.HandlerTestCase{
		{
			Name:   "should_delete_job_when_exists",
			Method: "DELETE",
			Path:   "/jobs/1",
			Headers: map[string]string{
				"HX-Request": "true",
			},
			MockSetup: func() {
				mockService.On("DeleteJob", mock.Anything, 1, 1).
					Return(nil)
			},
			ExpectedStatus: http.StatusOK,
			ExpectedHeader: map[string]string{
				"HX-Redirect": "/jobs",
			},
		},
		{
			Name:   "should_return_404_when_job_not_found",
			Method: "DELETE",
			Path:   "/jobs/999",
			Headers: map[string]string{
				"HX-Request": "true",
			},
			MockSetup: func() {
				mockService.On("DeleteJob", mock.Anything, 1, 999).
					Return(models.ErrJobNotFound)
			},
			ExpectedStatus: http.StatusNotFound,
			ExpectedToast: &testutil.ToastAssertion{
				Message: models.ErrJobNotFound.Error(),
				Type:    string(alerts.TypeError),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			mockService.ExpectedCalls = nil
			mockService.Calls = nil
			testutil.RunHandlerTest(t, router, tc)
			mockService.AssertExpectations(t)
		})
	}
}

// GetJobs method is not implemented in JobHandler
// The handler uses ListJobsPage for displaying jobs
func TestJobHandler_GetJobs(t *testing.T) {
	t.Skip("GetJobs method not implemented - use ListJobsPage instead")
}

func intPtr(i int) *int {
	return &i
}
