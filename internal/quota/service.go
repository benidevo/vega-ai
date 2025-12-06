package quota

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	ctxutil "github.com/benidevo/vega/internal/common/context"
	timeutil "github.com/benidevo/vega/internal/common/time"
	"github.com/benidevo/vega/internal/quota/models"
)

// JobRepository interface defines methods the quota service needs from the job repository
type JobRepository interface {
	GetByID(ctx context.Context, userID, jobID int) (*models.Job, error)
	SetFirstAnalyzedAtWithTx(ctx context.Context, tx *sql.Tx, jobID int) error
}

// Service handles quota management
type Service struct {
	db          *sql.DB
	repo        Repository
	jobRepo     JobRepository
	isCloudMode bool
}

// NewService creates a new quota service
func NewService(db *sql.DB, jobRepo JobRepository, isCloudMode bool) *Service {
	return &Service{
		db:          db,
		repo:        NewRepository(db),
		jobRepo:     jobRepo,
		isCloudMode: isCloudMode,
	}
}

// isUserAdmin checks if the user in context has admin role
func (s *Service) isUserAdmin(ctx context.Context) bool {
	role, _ := ctxutil.GetRole(ctx)
	return role == "Admin"
}

// CanAnalyzeJob checks if a user can analyze a specific job
func (s *Service) CanAnalyzeJob(ctx context.Context, userID int, jobID int) (*models.QuotaCheckResult, error) {
	// Check if job was previously analyzed
	job, err := s.jobRepo.GetByID(ctx, userID, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	// Get current usage for status
	monthYear := timeutil.GetCurrentMonthYear()
	usage, err := s.repo.GetMonthlyUsage(ctx, userID, monthYear)
	if err != nil {
		return nil, fmt.Errorf("failed to get monthly usage: %w", err)
	}

	// Check if user is admin (unlimited AI analysis quota in cloud mode)
	if s.isCloudMode && s.isUserAdmin(ctx) {
		status := models.QuotaStatus{
			Used:      usage.JobsAnalyzed,
			Limit:     -1,
			ResetDate: time.Time{},
		}

		// If job was previously analyzed, note it as re-analysis
		reason := models.QuotaReasonOK
		if job.FirstAnalyzedAt != nil {
			reason = models.QuotaReasonReanalysis
		}

		return &models.QuotaCheckResult{
			Allowed: true,
			Reason:  reason,
			Status:  status,
		}, nil
	}

	// In non-cloud mode, always allow but show actual usage
	if !s.isCloudMode {
		status := models.QuotaStatus{
			Used:      usage.JobsAnalyzed,
			Limit:     -1,
			ResetDate: time.Time{},
		}

		// If job was previously analyzed, note it as re-analysis
		reason := models.QuotaReasonOK
		if job.FirstAnalyzedAt != nil {
			reason = models.QuotaReasonReanalysis
		}

		return &models.QuotaCheckResult{
			Allowed: true,
			Reason:  reason,
			Status:  status,
		}, nil
	}

	// Cloud mode: get quota configuration
	quotaConfig, err := s.repo.GetQuotaConfig(ctx, "ai_analysis_monthly")
	if err != nil {
		return nil, fmt.Errorf("failed to get quota config: %w", err)
	}

	limit := quotaConfig.FreeLimit

	// Cloud mode: enforce quotas
	status := models.QuotaStatus{
		Used:      usage.JobsAnalyzed,
		Limit:     limit,
		ResetDate: timeutil.GetNextMonthStart(),
	}

	// If job was previously analyzed, it's a re-analysis (always allowed)
	if job.FirstAnalyzedAt != nil {
		return &models.QuotaCheckResult{
			Allowed: true,
			Reason:  models.QuotaReasonReanalysis,
			Status:  status,
		}, nil
	}

	// Check monthly limit for new analyses
	if usage.JobsAnalyzed >= limit {
		return &models.QuotaCheckResult{
			Allowed: false,
			Reason:  models.QuotaReasonLimitReached,
			Status:  status,
		}, nil
	}

	return &models.QuotaCheckResult{
		Allowed: true,
		Reason:  models.QuotaReasonOK,
		Status:  status,
	}, nil
}

// RecordAnalysis records that a job has been analyzed
func (s *Service) RecordAnalysis(ctx context.Context, userID int, jobID int) error {
	// Always record usage for tracking purposes
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Use transactional method to ensure operation participates in transaction
	err = s.jobRepo.SetFirstAnalyzedAtWithTx(ctx, tx, jobID)
	if err != nil {
		return fmt.Errorf("failed to set first analyzed at: %w", err)
	}

	monthYear := timeutil.GetCurrentMonthYear()

	// Use transactional repository method to increment usage within same transaction
	err = s.repo.IncrementMonthlyUsageWithTx(ctx, tx, userID, monthYear)
	if err != nil {
		return fmt.Errorf("failed to update quota usage: %w", err)
	}

	return tx.Commit()
}

// GetMonthlyUsage gets the current month's usage for a user
func (s *Service) GetMonthlyUsage(ctx context.Context, userID int) (*models.QuotaUsage, error) {
	// Always get actual usage data for tracking purposes
	monthYear := timeutil.GetCurrentMonthYear()
	return s.repo.GetMonthlyUsage(ctx, userID, monthYear)
}

// GetQuotaStatus returns the current quota status for a user
func (s *Service) GetQuotaStatus(ctx context.Context, userID int) (*models.QuotaStatus, error) {
	// Always get actual usage data
	usage, err := s.GetMonthlyUsage(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Check if user is admin (unlimited AI analysis quota in cloud mode)
	if s.isCloudMode && s.isUserAdmin(ctx) {
		return &models.QuotaStatus{
			Used:      usage.JobsAnalyzed,
			Limit:     -1,
			ResetDate: time.Time{},
		}, nil
	}

	if !s.isCloudMode {
		// In self-hosted mode, return actual usage but unlimited limit
		return &models.QuotaStatus{
			Used:      usage.JobsAnalyzed,
			Limit:     -1,
			ResetDate: time.Time{},
		}, nil
	}

	// Cloud mode: get quota configuration
	quotaConfig, err := s.repo.GetQuotaConfig(ctx, "ai_analysis_monthly")
	if err != nil {
		return nil, err
	}

	// Cloud mode: return actual quota status with limits
	return &models.QuotaStatus{
		Used:      usage.JobsAnalyzed,
		Limit:     quotaConfig.FreeLimit,
		ResetDate: timeutil.GetNextMonthStart(),
	}, nil
}
