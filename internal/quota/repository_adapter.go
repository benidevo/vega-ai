package quota

import (
	"context"

	"github.com/benidevo/vega/internal/job/interfaces"
	"github.com/benidevo/vega/internal/quota/models"
)

// JobRepositoryAdapter adapts the job repository interface for quota service
type JobRepositoryAdapter struct {
	jobRepo interfaces.JobRepository
}

// NewJobRepositoryAdapter creates a new adapter
func NewJobRepositoryAdapter(jobRepo interfaces.JobRepository) JobRepository {
	return &JobRepositoryAdapter{
		jobRepo: jobRepo,
	}
}

// GetByID adapts the job repository GetByID method for quota service
func (a *JobRepositoryAdapter) GetByID(ctx context.Context, userID, jobID int) (*models.Job, error) {
	job, err := a.jobRepo.GetByID(ctx, userID, jobID)
	if err != nil {
		return nil, err
	}

	// Convert to quota Job struct
	return &models.Job{
		ID:              job.ID,
		FirstAnalyzedAt: job.FirstAnalyzedAt,
	}, nil
}

// SetFirstAnalyzedAt delegates to the job repository
func (a *JobRepositoryAdapter) SetFirstAnalyzedAt(ctx context.Context, jobID int) error {
	return a.jobRepo.SetFirstAnalyzedAt(ctx, jobID)
}
