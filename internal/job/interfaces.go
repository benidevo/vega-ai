package job

import (
	"context"

	"github.com/benidevo/vega/internal/job/models"
)

// JobReader handles job retrieval operations
type JobReader interface {
	GetJob(ctx context.Context, userID int, jobID int) (*models.Job, error)
	GetJobsWithPagination(ctx context.Context, userID int, filter models.JobFilter) (*models.JobsWithPagination, error)
	GetStats(ctx context.Context, userID int) (*models.JobStats, error)
}

// JobWriter handles job creation and modification operations
type JobWriter interface {
	CreateJob(ctx context.Context, userID int, title, description, companyName string) (*models.Job, bool, error)
	UpdateJob(ctx context.Context, userID int, job *models.Job) error
	UpdateJobStatus(ctx context.Context, userID int, jobID int, status models.JobStatus) error
	DeleteJob(ctx context.Context, userID int, jobID int) error
}

// JobValidator handles job validation operations
type JobValidator interface {
	ValidateURL(url string) error
}

// FullJobService combines all job-related interfaces
type FullJobService interface {
	JobReader
	JobWriter
	JobValidator
}

// JobAnalyzer handles job analysis operations
type JobAnalyzer interface {
	AnalyzeJob(ctx context.Context, userID int, jobID int) (*models.MatchResult, error)
	GetJobMatchHistory(ctx context.Context, userID int, jobID int) ([]*models.MatchResult, error)
}

// CoverLetterGenerator handles cover letter generation
type CoverLetterGenerator interface {
	GenerateCoverLetter(ctx context.Context, userID int, jobID int) (*models.CoverLetter, error)
}

// DocumentChecker checks if documents exist for a job
type DocumentChecker interface {
	CheckCoverLetterExists(ctx context.Context, userID int, jobID int) (bool, error)
	CheckResumeExists(ctx context.Context, userID int, jobID int) (bool, error)
}

// JobCommandFactory creates commands for job operations
type JobCommandFactory interface {
	CreateAnalyzeJobCommand(jobID int) interface{}
	CreateGenerateCoverLetterCommand(jobID int) interface{}
}

// CompanyService handles company-related operations
type CompanyService interface {
	GetCompany(ctx context.Context, companyID int) (*models.Company, error)
	GetAllCompanies(ctx context.Context) ([]*models.Company, error)
}

// For API handlers that need minimal functionality
type JobAPIService interface {
	GetJob(ctx context.Context, userID int, jobID int) (*models.Job, error)
	CreateJob(ctx context.Context, userID int, title, description, companyName string) (*models.Job, bool, error)
	UpdateJob(ctx context.Context, userID int, job *models.Job) error
	DeleteJob(ctx context.Context, userID int, jobID int) error
}
