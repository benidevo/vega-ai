package job

import (
	"context"

	"github.com/benidevo/vega/internal/job/models"
	quotamodels "github.com/benidevo/vega/internal/quota/models"
)

// jobService defines job operations needed by the API handler
type jobService interface {
	CreateJob(ctx context.Context, userID int, title, description, companyName string, options ...models.JobOption) (*models.Job, bool, error)
	GetJob(ctx context.Context, userID int, jobID int) (*models.Job, error)
	UpdateJob(ctx context.Context, userID int, job *models.Job) error
	DeleteJob(ctx context.Context, userID int, jobID int) error
	GetQuotaStatus(ctx context.Context, userID int) (*quotamodels.QuotaStatus, error)
	LogError(err error)
}

// quotaService defines quota operations needed by the API handler
type quotaService interface {
	RecordUsage(ctx context.Context, userID int, quotaType string, metadata map[string]interface{}) error
}
