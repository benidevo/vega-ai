package job

import (
	"context"
	"testing"
	"time"

	"github.com/benidevo/vega/internal/config"
	jobinterfacesmocks "github.com/benidevo/vega/internal/job/interfaces/mocks"
	"github.com/benidevo/vega/internal/job/models"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func init() {
	// Disable logs for tests
	zerolog.SetGlobalLevel(zerolog.Disabled)
}

const testUserID = 1

func setupTestConfig() *config.Settings {
	return &config.Settings{
		IsTest:   true,
		LogLevel: "disabled",
	}
}

func createTestCompany() models.Company {
	return models.Company{
		ID:        1,
		Name:      "Test Company",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func createTestJob(id int, title string, company models.Company) *models.Job {
	return &models.Job{
		ID:          id,
		UserID:      testUserID,
		Title:       title,
		Description: "Test description",
		Company:     company,
		Status:      models.INTERESTED,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

func TestJobService(t *testing.T) {
	ctx := context.Background()
	company := createTestCompany()
	job := createTestJob(1, "Software Engineer", company)
	cfg := setupTestConfig()

	t.Run("should create job successfully", func(t *testing.T) {
		mockRepo := jobinterfacesmocks.NewMockJobRepository(t)
		mockRepo.On("GetOrCreate", ctx, testUserID, mock.AnythingOfType("*models.Job")).Return(job, true, nil)

		service := NewJobService(mockRepo, nil, nil, nil, cfg)
		createdJob, isNew, err := service.CreateJob(ctx, testUserID, job.Title, job.Description, company.Name)

		require.NoError(t, err)
		assert.True(t, isNew)
		assert.Equal(t, job.ID, createdJob.ID)
		assert.Equal(t, job.Title, createdJob.Title)
	})

	t.Run("should return job when valid ID", func(t *testing.T) {
		mockRepo := jobinterfacesmocks.NewMockJobRepository(t)
		mockRepo.On("GetByID", ctx, testUserID, 1).Return(job, nil)

		service := NewJobService(mockRepo, nil, nil, nil, cfg)
		foundJob, err := service.GetJob(ctx, testUserID, 1)

		require.NoError(t, err)
		assert.Equal(t, job.ID, foundJob.ID)
	})

	t.Run("should return error when invalid ID", func(t *testing.T) {
		service := NewJobService(nil, nil, nil, nil, cfg) // No repo calls expected
		_, err := service.GetJob(ctx, testUserID, 0)
		assert.Equal(t, models.ErrInvalidJobID, err)

		_, err = service.GetJob(ctx, testUserID, -1)
		assert.Equal(t, models.ErrInvalidJobID, err)
	})

	t.Run("should filter jobs with different criteria", func(t *testing.T) {
		jobs := []*models.Job{
			createTestJob(1, "Software Engineer", company),
			createTestJob(2, "Product Manager", company),
		}

		t.Run("should filter by status", func(t *testing.T) {
			mockRepo := jobinterfacesmocks.NewMockJobRepository(t)
			status := models.APPLIED
			statusFilter := models.JobFilter{
				Status: &status,
				Limit:  12,
			}
			mockRepo.On("GetAll", ctx, testUserID, statusFilter).Return(jobs, nil)
			mockRepo.On("GetCount", ctx, testUserID, statusFilter).Return(2, nil)

			service := NewJobService(mockRepo, nil, nil, nil, cfg)
			result, err := service.GetJobsWithPagination(ctx, testUserID, statusFilter)

			require.NoError(t, err)
			assert.Len(t, result.Jobs, 2)
		})

		t.Run("should filter by company ID", func(t *testing.T) {
			mockRepo := jobinterfacesmocks.NewMockJobRepository(t)
			companyID := 1
			companyFilter := models.JobFilter{
				CompanyID: &companyID,
				Limit:     12,
			}
			mockRepo.On("GetAll", ctx, testUserID, companyFilter).Return(jobs, nil)
			mockRepo.On("GetCount", ctx, testUserID, companyFilter).Return(2, nil)

			service := NewJobService(mockRepo, nil, nil, nil, cfg)
			result, err := service.GetJobsWithPagination(ctx, testUserID, companyFilter)

			require.NoError(t, err)
			assert.Len(t, result.Jobs, 2)
		})

		t.Run("should filter by job type", func(t *testing.T) {
			mockRepo := jobinterfacesmocks.NewMockJobRepository(t)
			jobType := models.FULL_TIME
			typeFilter := models.JobFilter{
				JobType: &jobType,
				Limit:   12,
			}
			mockRepo.On("GetAll", ctx, testUserID, typeFilter).Return(jobs, nil)
			mockRepo.On("GetCount", ctx, testUserID, typeFilter).Return(2, nil)

			service := NewJobService(mockRepo, nil, nil, nil, cfg)
			result, err := service.GetJobsWithPagination(ctx, testUserID, typeFilter)

			require.NoError(t, err)
			assert.Len(t, result.Jobs, 2)
		})

		t.Run("should apply complex filter with multiple criteria", func(t *testing.T) {
			mockRepo := jobinterfacesmocks.NewMockJobRepository(t)
			status := models.APPLIED
			jobType := models.FULL_TIME
			complexFilter := models.JobFilter{
				Status:  &status,
				JobType: &jobType,
				Limit:   5,
				Offset:  10,
			}
			mockRepo.On("GetAll", ctx, testUserID, complexFilter).Return(jobs[:1], nil)
			mockRepo.On("GetCount", ctx, testUserID, complexFilter).Return(1, nil)

			service := NewJobService(mockRepo, nil, nil, nil, cfg)
			result, err := service.GetJobsWithPagination(ctx, testUserID, complexFilter)

			require.NoError(t, err)
			assert.Len(t, result.Jobs, 1)
		})
	})

	t.Run("should validate URLs for XSS prevention", func(t *testing.T) {
		mockJobRepo := jobinterfacesmocks.NewMockJobRepository(t)
		cfg := &config.Settings{}
		service := NewJobService(mockJobRepo, nil, nil, nil, cfg)

		testCases := []struct {
			name    string
			url     string
			wantErr bool
		}{
			{"empty URL is valid", "", false},
			{"valid http URL", "http://example.com/job", false},
			{"valid https URL", "https://example.com/job", false},
			{"javascript URL is blocked", "javascript:alert('XSS')", true},
			{"data URL is blocked", "data:text/html,<script>alert('XSS')</script>", true},
			{"file URL is blocked", "file:///etc/passwd", true},
			{"ftp URL is blocked", "ftp://example.com/file", true},
			{"invalid URL format", "not a url", true},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				err := service.ValidateURL(tc.url)
				if tc.wantErr {
					assert.Error(t, err)
					assert.Equal(t, models.ErrInvalidURLFormat, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("should update job successfully", func(t *testing.T) {
		mockRepo := jobinterfacesmocks.NewMockJobRepository(t)
		mockRepo.On("Update", ctx, testUserID, mock.AnythingOfType("*models.Job")).Return(nil)

		service := NewJobService(mockRepo, nil, nil, nil, cfg)
		err := service.UpdateJob(ctx, testUserID, job)

		require.NoError(t, err)
	})

	t.Run("should validate job before updating", func(t *testing.T) {
		service := NewJobService(nil, nil, nil, nil, cfg) // No repo calls expected

		// Test nil job
		err := service.UpdateJob(ctx, testUserID, nil)
		assert.Equal(t, models.ErrInvalidJobID, err)

		// Test job with invalid ID
		invalidJob := createTestJob(0, "Invalid Job", company)
		err = service.UpdateJob(ctx, testUserID, invalidJob)
		assert.Equal(t, models.ErrInvalidJobID, err)
	})

	t.Run("should delete job successfully", func(t *testing.T) {
		mockRepo := jobinterfacesmocks.NewMockJobRepository(t)
		mockRepo.On("GetByID", ctx, testUserID, 1).Return(job, nil)
		mockRepo.On("Delete", ctx, testUserID, 1).Return(nil)

		service := NewJobService(mockRepo, nil, nil, nil, cfg)
		err := service.DeleteJob(ctx, testUserID, 1)

		require.NoError(t, err)
	})

	t.Run("should return error when trying to delete with invalid ID", func(t *testing.T) {
		service := NewJobService(nil, nil, nil, nil, cfg) // No repo calls expected
		err := service.DeleteJob(ctx, testUserID, 0)
		assert.Equal(t, models.ErrInvalidJobID, err)
	})
}
