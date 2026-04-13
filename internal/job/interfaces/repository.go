package interfaces

import (
	"context"
	"database/sql"

	"github.com/benidevo/vega/internal/job/models"
)

// CompanyRepository defines methods for interacting with company data
type CompanyRepository interface {
	GetOrCreate(ctx context.Context, name string) (*models.Company, error)
	GetByID(ctx context.Context, id int) (*models.Company, error)
	GetByName(ctx context.Context, name string) (*models.Company, error)
	GetAll(ctx context.Context) ([]*models.Company, error)
	Delete(ctx context.Context, id int) error
	Update(ctx context.Context, company *models.Company) error
}

// JobRepository defines methods for interacting with job data
type JobRepository interface {
	Create(ctx context.Context, userID int, job *models.Job) (*models.Job, error)
	GetByID(ctx context.Context, userID int, id int) (*models.Job, error)
	GetAll(ctx context.Context, userID int, filter models.JobFilter) ([]*models.Job, error)
	GetCount(ctx context.Context, userID int, filter models.JobFilter) (int, error)
	Update(ctx context.Context, userID int, job *models.Job) error
	Delete(ctx context.Context, userID int, id int) error
	UpdateStatus(ctx context.Context, userID int, id int, status models.JobStatus) error
	UpdateMatchScore(ctx context.Context, userID int, jobID int, matchScore *int) error
	GetStats(ctx context.Context, userID int) (*models.JobStats, error)
	GetBySourceURL(ctx context.Context, userID int, sourceURL string) (*models.Job, error)
	GetOrCreate(ctx context.Context, userID int, job *models.Job) (*models.Job, bool, error)

	CreateMatchResult(ctx context.Context, userID int, matchResult *models.MatchResult) error
	GetJobMatchHistory(ctx context.Context, userID int, jobID int) ([]*models.MatchResult, error)
	GetRecentMatchResults(ctx context.Context, userID int, limit int) ([]*models.MatchResult, error)
	GetRecentMatchResultsWithDetails(ctx context.Context, userID int, limit int, currentJobID int) ([]*models.MatchSummary, error)
	DeleteMatchResult(ctx context.Context, userID int, matchID int) error
	MatchResultBelongsToJob(ctx context.Context, userID int, matchID, jobID int) (bool, error)

	// User-specific methods
	GetStatsByUserID(ctx context.Context, userID int) (*models.JobStats, error)
	GetRecentJobsByUserID(ctx context.Context, userID int, limit int) ([]*models.Job, error)
	GetTopMatchesByUserID(ctx context.Context, userID int, limit int) ([]*models.Job, error)
	GetJobStatsByStatus(ctx context.Context, userID int) (map[models.JobStatus]int, error)

	// Quota-related methods
	GetMonthlyAnalysisCount(ctx context.Context, userID int) (int, error)
	SetFirstAnalyzedAt(ctx context.Context, jobID int) error
}

// TransactionalRepository provides transaction support for repositories
type TransactionalRepository interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
	CommitTx(tx *sql.Tx) error
	RollbackTx(tx *sql.Tx) error
}

// TransactionalJobRepository extends JobRepository with transaction support
type TransactionalJobRepository interface {
	JobRepository
	TransactionalRepository
	CreateWithTx(ctx context.Context, tx *sql.Tx, userID int, job *models.Job) (*models.Job, error)
	UpdateWithTx(ctx context.Context, tx *sql.Tx, userID int, job *models.Job) error
	DeleteWithTx(ctx context.Context, tx *sql.Tx, userID int, id int) error
	SetFirstAnalyzedAtWithTx(ctx context.Context, tx *sql.Tx, jobID int) error
}

// TransactionalCompanyRepository extends CompanyRepository with transaction support
type TransactionalCompanyRepository interface {
	CompanyRepository
	TransactionalRepository
	GetOrCreateWithTx(ctx context.Context, tx *sql.Tx, name string) (*models.Company, error)
	UpdateWithTx(ctx context.Context, tx *sql.Tx, company *models.Company) error
}
