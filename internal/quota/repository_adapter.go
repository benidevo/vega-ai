package quota

import (
	"context"
	"database/sql"

	"github.com/benidevo/vega/internal/job/interfaces"
	"github.com/benidevo/vega/internal/quota/models"
)

// JobRepositoryAdapter adapts the job repository interface for quota service
type JobRepositoryAdapter struct {
	jobRepo interfaces.TransactionalJobRepository
}

// NewJobRepositoryAdapter creates a new adapter
func NewJobRepositoryAdapter(jobRepo interfaces.TransactionalJobRepository) JobRepository {
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

// SetFirstAnalyzedAtWithTx delegates to the job repository within a transaction
func (a *JobRepositoryAdapter) SetFirstAnalyzedAtWithTx(ctx context.Context, tx *sql.Tx, jobID int) error {
	return a.jobRepo.SetFirstAnalyzedAtWithTx(ctx, tx, jobID)
}
