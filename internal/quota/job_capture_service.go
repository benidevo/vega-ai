package quota

import (
	"context"
	"fmt"
	"time"

	timeutil "github.com/benidevo/vega/internal/common/time"
	"github.com/benidevo/vega/internal/quota/models"
)

// JobCaptureService handles job capture tracking from browser extension
type JobCaptureService struct {
	repo        Repository
	isCloudMode bool
}

// NewJobCaptureService creates a new job capture service
func NewJobCaptureService(repo Repository, isCloudMode bool) *JobCaptureService {
	return &JobCaptureService{
		repo:        repo,
		isCloudMode: isCloudMode,
	}
}

// CanCaptureJobs checks if a user can capture more jobs (always returns true)
func (s *JobCaptureService) CanCaptureJobs(ctx context.Context, userID int) (*models.QuotaCheckResult, error) {
	today := timeutil.GetCurrentDate()
	usage, err := s.repo.GetDailyUsage(ctx, userID, today, models.QuotaKeyJobsCaptured)
	if err != nil {
		return nil, fmt.Errorf("failed to get job capture usage: %w", err)
	}

	return &models.QuotaCheckResult{
		Allowed: true,
		Reason:  models.QuotaReasonOK,
		Status: models.QuotaStatus{
			Used:      usage,
			Limit:     -1,
			ResetDate: time.Time{},
		},
	}, nil
}

// RecordJobsCaptured records that jobs were captured via extension
func (s *JobCaptureService) RecordJobsCaptured(ctx context.Context, userID int, count int) error {
	today := timeutil.GetCurrentDate()
	return s.repo.IncrementDailyUsage(ctx, userID, today, models.QuotaKeyJobsCaptured, count)
}

// GetStatus returns the current job capture status
func (s *JobCaptureService) GetStatus(ctx context.Context, userID int) (*models.QuotaCheckResult, error) {
	today := timeutil.GetCurrentDate()
	jobsCaptured, err := s.repo.GetDailyUsage(ctx, userID, today, models.QuotaKeyJobsCaptured)
	if err != nil {
		return nil, fmt.Errorf("failed to get job capture usage: %w", err)
	}

	// Job captures are unlimited for everyone
	return &models.QuotaCheckResult{
		Allowed: true,
		Reason:  models.QuotaReasonOK,
		Status: models.QuotaStatus{
			Used:      jobsCaptured,
			Limit:     -1,
			ResetDate: time.Time{},
		},
	}, nil
}
