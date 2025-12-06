package quota

import (
	"context"

	"github.com/benidevo/vega/internal/quota/models"
)

// QuotaChecker checks if actions are allowed based on quota
type QuotaChecker interface {
	CanAnalyzeJob(ctx context.Context, userID int, jobID int) (*models.QuotaCheckResult, error)
}

// QuotaReporter provides quota status and usage information
type QuotaReporter interface {
	GetQuotaStatus(ctx context.Context, userID int) (*models.QuotaStatus, error)
	GetMonthlyUsage(ctx context.Context, userID int) (*models.QuotaUsage, error)
}

// QuotaRecorder records quota consumption
type QuotaRecorder interface {
	RecordAnalysis(ctx context.Context, userID int, jobID int) error
}

// QuotaService combines all quota operations
type QuotaService interface {
	QuotaChecker
	QuotaReporter
	QuotaRecorder
}
