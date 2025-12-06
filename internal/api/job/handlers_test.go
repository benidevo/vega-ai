package job

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/benidevo/vega/internal/common/testutil"
	"github.com/benidevo/vega/internal/job/models"
	quotamodels "github.com/benidevo/vega/internal/quota/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockJobService implements the jobService interface for testing
type mockJobService struct {
	mock.Mock
}

func (m *mockJobService) GetJob(ctx context.Context, userID int, jobID int) (*models.Job, error) {
	args := m.Called(ctx, userID, jobID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Job), args.Error(1)
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

func (m *mockJobService) UpdateJob(ctx context.Context, userID int, job *models.Job) error {
	args := m.Called(ctx, userID, job)
	return args.Error(0)
}

func (m *mockJobService) DeleteJob(ctx context.Context, userID int, jobID int) error {
	args := m.Called(ctx, userID, jobID)
	return args.Error(0)
}

func (m *mockJobService) GetQuotaStatus(ctx context.Context, userID int) (*quotamodels.QuotaStatus, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*quotamodels.QuotaStatus), args.Error(1)
}

func (m *mockJobService) LogError(err error) {
	m.Called(err)
}

// mockQuotaService implements the quotaService interface for testing
type mockQuotaService struct {
	mock.Mock
}

func (m *mockQuotaService) RecordUsage(ctx context.Context, userID int, quotaType string, metadata map[string]interface{}) error {
	args := m.Called(ctx, userID, quotaType, metadata)
	return args.Error(0)
}

func setupTestJobAPIHandler() (*JobAPIHandler, *mockJobService, *mockQuotaService, *gin.Engine) {
	mockJobService := new(mockJobService)
	mockQuotaService := new(mockQuotaService)
	handler := NewJobAPIHandler(mockJobService, mockQuotaService)
	router := testutil.SetupTestRouter()

	return handler, mockJobService, mockQuotaService, router
}

// Helper function to set user context
func setUserContext(c *gin.Context, userID int) {
	c.Set("userID", userID)
}

// GetJob method is not implemented in the current API handler
// This test is kept as documentation for future implementation
func TestJobAPIHandler_GetJob(t *testing.T) {
	t.Skip("GetJob method not implemented in JobAPIHandler")
}

func TestJobAPIHandler_CreateJob(t *testing.T) {
	handler, mockService, mockQuotaService, router := setupTestJobAPIHandler()

	// Setup routes with middleware to set user context
	router.POST("/api/jobs", func(c *gin.Context) {
		setUserContext(c, 1)
		handler.CreateJob(c)
	})

	tests := []testutil.HandlerTestCase{
		{
			Name:   "should_create_job_when_data_valid",
			Method: "POST",
			Path:   "/api/jobs",
			Body: map[string]interface{}{
				"title":       "Software Engineer",
				"description": "Build awesome software",
				"company":     "Acme Corp",
				"location":    "Remote",
				"jobType":     "FULL_TIME",
				"sourceUrl":   "https://example.com/job",
			},
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			MockSetup: func() {
				job := &models.Job{
					ID:          1,
					UserID:      1,
					Title:       "Software Engineer",
					Description: "Build awesome software",
					Company:     models.Company{ID: 1, Name: "Acme Corp"},
					Location:    "Remote",
					JobType:     models.FULL_TIME,
					SourceURL:   "https://example.com/job",
					Status:      models.INTERESTED,
				}
				// For CreateJob with options, we use AnythingOfType to match the variadic arguments
				mockService.On("CreateJob", mock.Anything, 1, "Software Engineer", "Build awesome software", "Acme Corp", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(job, true, nil)
				mockQuotaService.On("RecordUsage", mock.Anything, 1, "job_capture", mock.Anything).
					Return(nil)
			},
			ExpectedStatus: http.StatusOK,
			ValidateBody: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)

				assert.Equal(t, "Job created successfully", response["message"])
				assert.Equal(t, float64(1), response["jobId"])
			},
		},
		{
			Name:   "should_return_info_when_job_already_exists",
			Method: "POST",
			Path:   "/api/jobs",
			Body: map[string]interface{}{
				"title":       "Software Engineer",
				"description": "Build awesome software",
				"company":     "Acme Corp",
				"location":    "Remote",
				"sourceUrl":   "https://example.com/job",
			},
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			MockSetup: func() {
				job := &models.Job{
					ID:    1,
					Title: "Software Engineer",
				}
				mockService.On("CreateJob", mock.Anything, 1, "Software Engineer", "Build awesome software", "Acme Corp", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(job, false, nil)
			},
			ExpectedStatus: http.StatusOK,
			ValidateBody: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, "Job created successfully", response["message"])
				assert.Equal(t, float64(1), response["jobId"])
			},
		},
		{
			Name:   "should_return_400_when_title_missing",
			Method: "POST",
			Path:   "/api/jobs",
			Body: map[string]interface{}{
				"description": "Build awesome software",
				"company":     "Acme Corp",
			},
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			MockSetup: func() {
				mockService.On("LogError", mock.Anything).Return()
			},
			ExpectedStatus: http.StatusBadRequest,
			ValidateBody: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]string
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Contains(t, response["error"], "Invalid request format:")
			},
		},
		{
			Name:   "should_return_409_when_duplicate_job_error",
			Method: "POST",
			Path:   "/api/jobs",
			Body: map[string]interface{}{
				"title":       "Software Engineer",
				"description": "Build awesome software",
				"company":     "Acme Corp",
				"location":    "Remote",
				"sourceUrl":   "https://example.com/job",
			},
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			MockSetup: func() {
				mockService.On("CreateJob", mock.Anything, 1, "Software Engineer", "Build awesome software", "Acme Corp", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil, false, models.ErrDuplicateJob)
			},
			ExpectedStatus: http.StatusConflict,
			ValidateBody: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, "Job already exists with this source URL", response["error"])
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

// DeleteJob method is not implemented in the current API handler
// This test is kept as documentation for future implementation
func TestJobAPIHandler_DeleteJob(t *testing.T) {
	t.Skip("DeleteJob method not implemented in JobAPIHandler")
}

// GetJobs method is not implemented in the current API handler
// This test is kept as documentation for future implementation
func TestJobAPIHandler_GetJobs(t *testing.T) {
	t.Skip("GetJobs method not implemented in JobAPIHandler")
}

func TestJobAPIHandler_GetQuotaStatus(t *testing.T) {
	handler, mockService, _, router := setupTestJobAPIHandler()

	// Setup routes with middleware to set user context
	router.GET("/api/quota/status", func(c *gin.Context) {
		setUserContext(c, 1)
		handler.GetQuotaStatus(c)
	})

	tests := []testutil.HandlerTestCase{
		{
			Name:   "should_return_quota_status_when_authenticated",
			Method: "GET",
			Path:   "/api/quota/status",
			MockSetup: func() {
				quotaStatus := &quotamodels.QuotaStatus{
					Used:      5,
					Limit:     10,
					ResetDate: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				}
				mockService.On("GetQuotaStatus", mock.Anything, 1).
					Return(quotaStatus, nil)
			},
			ExpectedStatus: http.StatusOK,
			ValidateBody: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)

				assert.Equal(t, float64(5), response["used"])
				assert.Equal(t, float64(10), response["limit"])
				assert.Equal(t, float64(5), response["remaining"])
				assert.Equal(t, "2024-01-01", response["reset_date"])
			},
		},
		{
			Name:   "should_return_unlimited_when_limit_negative",
			Method: "GET",
			Path:   "/api/quota/status",
			MockSetup: func() {
				quotaStatus := &quotamodels.QuotaStatus{
					Used:      3,
					Limit:     -1,
					ResetDate: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				}
				mockService.On("GetQuotaStatus", mock.Anything, 1).
					Return(quotaStatus, nil)
			},
			ExpectedStatus: http.StatusOK,
			ValidateBody: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)

				assert.Equal(t, float64(3), response["used"])
				assert.Equal(t, float64(-1), response["limit"])
				assert.Equal(t, float64(-1), response["remaining"])
			},
		},
		{
			Name:   "should_return_error_when_service_fails",
			Method: "GET",
			Path:   "/api/quota/status",
			MockSetup: func() {
				mockService.On("GetQuotaStatus", mock.Anything, 1).
					Return(nil, errors.New("service error"))
				mockService.On("LogError", mock.Anything).Return()
			},
			ExpectedStatus: http.StatusInternalServerError,
			ValidateBody: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, "Failed to get quota status", response["error"])
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

// Helper function for tests
func intPtr(i int) *int {
	return &i
}
