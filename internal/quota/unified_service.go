package quota

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/benidevo/vega/internal/quota/models"
)

// UnifiedService provides a single interface for all quota operations
type UnifiedService struct {
	aiQuota    *Service
	jobCapture *JobCaptureService
	repo       Repository
}

// NewUnifiedService creates a new unified quota service
func NewUnifiedService(db *sql.DB, jobRepo JobRepository, isCloudMode bool) *UnifiedService {
	repo := NewRepository(db)

	return &UnifiedService{
		aiQuota:    NewService(db, jobRepo, isCloudMode),
		jobCapture: NewJobCaptureService(repo, isCloudMode),
		repo:       repo,
	}
}

// CheckQuota checks any quota type
func (s *UnifiedService) CheckQuota(ctx context.Context, userID int, quotaType string, metadata map[string]interface{}) (*models.QuotaCheckResult, error) {
	switch quotaType {
	case models.QuotaTypeAIAnalysis:
		jobID, ok := metadata["job_id"].(int)
		if !ok {
			return nil, fmt.Errorf("job_id required for AI analysis quota check")
		}
		return s.aiQuota.CanAnalyzeJob(ctx, userID, jobID)

	case models.QuotaTypeJobCapture:
		return s.jobCapture.CanCaptureJobs(ctx, userID)

	default:
		return nil, fmt.Errorf("unknown quota type: %s", quotaType)
	}
}

// RecordUsage records usage for any quota type
func (s *UnifiedService) RecordUsage(ctx context.Context, userID int, quotaType string, metadata map[string]interface{}) error {
	switch quotaType {
	case models.QuotaTypeAIAnalysis:
		jobID, ok := metadata["job_id"].(int)
		if !ok {
			return fmt.Errorf("job_id required for AI analysis recording")
		}
		return s.aiQuota.RecordAnalysis(ctx, userID, jobID)

	case models.QuotaTypeJobCapture:
		count, ok := metadata["count"].(int)
		if !ok {
			count = 1
		}
		return s.jobCapture.RecordJobsCaptured(ctx, userID, count)

	default:
		return fmt.Errorf("unknown quota type: %s", quotaType)
	}
}

// GetAllQuotaStatus gets all quota statuses for a user
func (s *UnifiedService) GetAllQuotaStatus(ctx context.Context, userID int) (interface{}, error) {
	status := &models.UnifiedQuotaStatus{}

	// Get AI analysis quota (monthly)
	aiStatus, err := s.aiQuota.GetQuotaStatus(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get AI quota status: %w", err)
	}
	status.AIAnalysis = *aiStatus

	// Get job capture quota (daily)
	captureResult, err := s.jobCapture.GetStatus(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get job capture quota status: %w", err)
	}
	status.JobCapture = captureResult.Status

	return status, nil
}

// Expose underlying services for backward compatibility
func (s *UnifiedService) AIQuotaService() *Service {
	return s.aiQuota
}

func (s *UnifiedService) JobCaptureService() *JobCaptureService {
	return s.jobCapture
}
