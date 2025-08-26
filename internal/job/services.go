package job

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/benidevo/vega/internal/ai"
	"github.com/benidevo/vega/internal/common/logger"
	"github.com/benidevo/vega/internal/config"
	"github.com/benidevo/vega/internal/documents"
	documentsmodels "github.com/benidevo/vega/internal/documents/models"
	"github.com/benidevo/vega/internal/job/interfaces"
	"github.com/benidevo/vega/internal/job/models"
	"github.com/benidevo/vega/internal/quota"
	"github.com/benidevo/vega/internal/settings"
	"github.com/go-playground/validator/v10"
)

// JobService provides business logic for job management.
type JobService struct {
	jobRepo         interfaces.JobRepository
	aiService       *ai.AIService
	settingsService *settings.SettingsService
	quotaService    *quota.Service
	documentService *documents.DocumentService
	cfg             *config.Settings
	log             *logger.PrivacyLogger
	validator       *validator.Validate
}

// NewJobService creates a new JobService instance.
func NewJobService(jobRepo interfaces.JobRepository, aiService *ai.AIService, settingsService *settings.SettingsService, quotaService *quota.Service, cfg *config.Settings) *JobService {
	return &JobService{
		jobRepo:         jobRepo,
		aiService:       aiService,
		settingsService: settingsService,
		quotaService:    quotaService,
		documentService: nil, // Will be set separately to avoid circular dependencies
		cfg:             cfg,
		log:             logger.GetPrivacyLogger("job"),
		validator:       validator.New(),
	}
}

// SetDocumentService sets the document service (called after initialization to avoid circular dependencies)
func (s *JobService) SetDocumentService(documentService *documents.DocumentService) {
	s.documentService = documentService
}

// CheckCoverLetterExists checks if a cover letter exists for a job
func (s *JobService) CheckCoverLetterExists(ctx context.Context, userID int, jobID int) (bool, error) {
	if s.documentService == nil {
		return false, nil
	}
	return s.documentService.CheckDocumentExists(ctx, userID, jobID, documentsmodels.DocumentTypeCoverLetter)
}

// CheckResumeExists checks if a resume exists for a job
func (s *JobService) CheckResumeExists(ctx context.Context, userID int, jobID int) (bool, error) {
	if s.documentService == nil {
		return false, nil
	}
	return s.documentService.CheckDocumentExists(ctx, userID, jobID, documentsmodels.DocumentTypeResume)
}

// GetQuotaStatus returns the current quota status for a user
func (s *JobService) GetQuotaStatus(ctx context.Context, userID int) (*quota.QuotaStatus, error) {
	if s.quotaService == nil {
		// If quota service is not available, return unlimited quota
		return &quota.QuotaStatus{
			Used:      0,
			Limit:     -1, // -1 indicates no limit
			ResetDate: time.Now().AddDate(0, 1, 0),
		}, nil
	}

	return s.quotaService.GetQuotaStatus(ctx, userID)
}

// CheckJobQuota checks if a user can analyze a specific job
func (s *JobService) CheckJobQuota(ctx context.Context, userID int, jobID int) (*quota.QuotaCheckResult, error) {
	if s.quotaService == nil {
		// If quota service is not available, allow all operations
		return &quota.QuotaCheckResult{
			Allowed: true,
			Reason:  quota.QuotaReasonOK,
			Status: quota.QuotaStatus{
				Used:      0,
				Limit:     -1,
				ResetDate: time.Now().AddDate(0, 1, 0),
			},
		}, nil
	}

	return s.quotaService.CanAnalyzeJob(ctx, userID, jobID)
}

// LogError logs the provided error using the service's logger if the error is not nil.
func (s *JobService) LogError(err error) {
	if err != nil {
		s.log.Error().Err(err).Msg("JobService error")
	}
}

// ValidateFieldName checks if a field name is valid for job updates.
// It returns an error if the field is empty or not one of the allowed values.
func (s *JobService) ValidateFieldName(field string) error {
	if field == "" {
		s.log.Error().Msg("Field parameter is required")
		return models.ErrFieldRequired
	}

	validFields := map[string]bool{
		"status": true,
		"notes":  true,
		"skills": true,
		"basic":  true,
	}

	if !validFields[field] {
		s.log.Error().Str("field", field).Msg("Invalid field parameter")
		return models.ErrInvalidFieldParam
	}

	return nil
}

// ValidateJobIDFormat validates that a job ID string can be parsed as an integer.
func (s *JobService) ValidateJobIDFormat(jobIDStr string) (int, error) {
	jobID, err := strconv.Atoi(jobIDStr)
	if err != nil {
		s.log.Error().Str("job_id", jobIDStr).Msg("Invalid job ID format")
		return 0, models.ErrInvalidJobIDFormat
	}
	return jobID, nil
}

// ValidateAndFilterSkills processes a comma-separated string of skills
// and returns only the non-empty skills after trimming whitespace.
// It returns an error if no valid skills are found.
func (s *JobService) ValidateAndFilterSkills(skillsStr string) []string {
	if skillsStr == "" {
		s.log.Error().Msg("Empty skills string provided")
		return make([]string, 0)
	}

	// Split and clean up skills
	rawSkills := strings.Split(skillsStr, ",")
	skills := make([]string, 0, len(rawSkills))

	// Only add non-empty skills
	for _, skill := range rawSkills {
		trimmedSkill := strings.TrimSpace(skill)
		if trimmedSkill != "" {
			skills = append(skills, trimmedSkill)
		}
	}

	if len(skills) == 0 {
		s.log.Error().Msg("No valid skills found after processing")
		return make([]string, 0)
	}

	return skills
}

// ValidateURL checks if a URL string is valid and safe
func (s *JobService) ValidateURL(urlStr string) error {
	if urlStr == "" {
		return nil // Empty URL is allowed
	}

	parsedURL, err := url.ParseRequestURI(urlStr)
	if err != nil {
		s.log.Error().Str("url", urlStr).Msg("Invalid URL format")
		return models.ErrInvalidURLFormat
	}

	// Only allow http and https schemes to prevent XSS via javascript: or data: URLs
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		s.log.Error().Str("url", urlStr).Str("scheme", parsedURL.Scheme).Msg("Invalid URL scheme")
		return models.ErrInvalidURLFormat
	}

	return nil
}

// CreateJob creates a new job with the given title, description, and company name.
// Additional job options can be provided.
// Returns the job and a boolean indicating whether it was newly created.
func (s *JobService) CreateJob(ctx context.Context, userID int, title, description, companyName string, options ...models.JobOption) (*models.Job, bool, error) {
	s.log.Debug().
		Str("title", title).
		Str("company", companyName).
		Msg("Creating new job")

	company := models.Company{Name: companyName}
	job := models.NewJob(title, description, company, options...)
	job.UserID = userID

	if err := s.validator.Struct(job); err != nil {
		s.log.Error().
			Str("title", title).
			Str("company", companyName).
			Err(err).
			Msg("Job validation failed")
		return nil, false, err
	}

	if err := job.Validate(); err != nil {
		s.log.Error().
			Str("title", title).
			Str("company", companyName).
			Err(err).
			Msg("Job validation failed")
		return nil, false, err
	}

	createdJob, isNew, err := s.jobRepo.GetOrCreate(ctx, userID, job)
	if err != nil {
		s.log.Error().
			Str("title", title).
			Str("company", companyName).
			Err(err).
			Msg("Failed to create job")
		return nil, false, err
	}

	if isNew {
		s.log.Info().
			Int("job_id", createdJob.ID).
			Str("title", createdJob.Title).
			Str("company", createdJob.Company.Name).
			Msg("Job created successfully")
	} else {
		s.log.Info().
			Int("job_id", createdJob.ID).
			Str("title", createdJob.Title).
			Str("company", createdJob.Company.Name).
			Msg("Job already exists")
	}

	return createdJob, isNew, nil
}

// GetJob retrieves a job by its ID.
func (s *JobService) GetJob(ctx context.Context, userID int, id int) (*models.Job, error) {
	s.log.Debug().Int("job_id", id).Msg("Getting job by ID")

	if id <= 0 {
		s.log.Error().Int("job_id", id).Msg("Invalid job ID")
		return nil, models.ErrInvalidJobID
	}

	job, err := s.jobRepo.GetByID(ctx, userID, id)
	if err != nil {
		s.log.Error().
			Int("job_id", id).
			Err(err).
			Msg("Failed to get job")
		return nil, err
	}

	s.log.Debug().
		Int("job_id", job.ID).
		Str("title", job.Title).
		Msg("Job retrieved successfully")

	return job, nil
}

// GetJobsWithPagination retrieves jobs with pagination metadata
func (s *JobService) GetJobsWithPagination(ctx context.Context, userID int, filter models.JobFilter) (*models.JobsWithPagination, error) {
	if filter.Limit <= 0 {
		filter.Limit = 12 // Default page size
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	jobs, err := s.jobRepo.GetAll(ctx, userID, filter)
	if err != nil {
		s.log.Error().
			Err(err).
			Msg("Failed to get jobs with pagination")
		return nil, err
	}

	totalCount, err := s.jobRepo.GetCount(ctx, userID, filter)
	if err != nil {
		s.log.Error().
			Err(err).
			Msg("Failed to get total job count")
		return nil, err
	}

	currentPage := (filter.Offset / filter.Limit) + 1
	totalPages := (totalCount + filter.Limit - 1) / filter.Limit // Ceiling division

	pagination := &models.PaginationInfo{
		CurrentPage:  currentPage,
		TotalPages:   totalPages,
		TotalItems:   totalCount,
		ItemsPerPage: filter.Limit,
		HasNext:      currentPage < totalPages,
		HasPrev:      currentPage > 1,
	}

	result := &models.JobsWithPagination{
		Jobs:       jobs,
		Pagination: pagination,
	}

	s.log.Debug().
		Int("count", len(jobs)).
		Int("total_count", totalCount).
		Int("current_page", currentPage).
		Int("total_pages", totalPages).
		Msg("Jobs with pagination retrieved successfully")

	return result, nil
}

// UpdateJob updates a job's details.
func (s *JobService) UpdateJob(ctx context.Context, userID int, job *models.Job) error {
	if job == nil {
		s.log.Error().Msg("Attempted to update nil job")
		return models.ErrInvalidJobID
	}

	if job.ID <= 0 {
		s.log.Error().Int("job_id", job.ID).Msg("Invalid job ID")
		return models.ErrInvalidJobID
	}

	s.log.Debug().
		Int("job_id", job.ID).
		Str("title", job.Title).
		Msg("Updating job")

	if err := s.validator.Struct(job); err != nil {
		s.log.Error().
			Int("job_id", job.ID).
			Err(err).
			Msg("Job validation failed")
		return err
	}

	if err := job.Validate(); err != nil {
		s.log.Error().
			Int("job_id", job.ID).
			Err(err).
			Msg("Job validation failed")
		return err
	}

	job.UpdatedAt = time.Now().UTC()

	err := s.jobRepo.Update(ctx, userID, job)
	if err != nil {
		s.log.Error().
			Int("job_id", job.ID).
			Err(err).
			Msg("Failed to update job")
		return err
	}

	s.log.Info().
		Int("job_id", job.ID).
		Str("title", job.Title).
		Msg("Job updated successfully")

	return nil
}

// DeleteJob removes a job by its ID.
func (s *JobService) DeleteJob(ctx context.Context, userID int, id int) error {
	s.log.Debug().Int("job_id", id).Msg("Deleting job")

	if id <= 0 {
		s.log.Error().Int("job_id", id).Msg("Invalid job ID")
		return models.ErrInvalidJobID
	}

	job, err := s.jobRepo.GetByID(ctx, userID, id)
	if err != nil {
		s.log.Error().
			Int("job_id", id).
			Err(err).
			Msg("Job not found for deletion")
		return err
	}

	err = s.jobRepo.Delete(ctx, userID, id)
	if err != nil {
		s.log.Error().
			Int("job_id", id).
			Err(err).
			Msg("Failed to delete job")
		return err
	}

	s.log.Info().
		Int("job_id", id).
		Str("title", job.Title).
		Msg("Job deleted successfully")

	return nil
}

// GetJobMatchHistory retrieves the match analysis history for a specific job
func (s *JobService) GetJobMatchHistory(ctx context.Context, userID int, jobID int) ([]*models.MatchResult, error) {
	s.log.Debug().Int("job_id", jobID).Msg("Getting job match history")

	if jobID <= 0 {
		s.log.Error().Int("job_id", jobID).Msg("Invalid job ID")
		return nil, models.ErrInvalidJobID
	}

	_, err := s.jobRepo.GetByID(ctx, userID, jobID)
	if err != nil {
		s.log.Error().
			Int("job_id", jobID).
			Err(err).
			Msg("Job not found")
		return nil, err
	}

	history, err := s.jobRepo.GetJobMatchHistory(ctx, userID, jobID)
	if err != nil {
		s.log.Error().
			Int("job_id", jobID).
			Err(err).
			Msg("Failed to get match history")
		return nil, err
	}

	s.log.Info().
		Int("job_id", jobID).
		Int("history_count", len(history)).
		Msg("Retrieved job match history")

	return history, nil
}

// DeleteMatchResult deletes a specific match result
func (s *JobService) DeleteMatchResult(ctx context.Context, userID int, jobID, matchID int) error {
	s.log.Debug().
		Int("job_id", jobID).
		Int("match_id", matchID).
		Msg("Deleting match result")

	if jobID <= 0 || matchID <= 0 {
		s.log.Error().
			Int("job_id", jobID).
			Int("match_id", matchID).
			Msg("Invalid job or match ID")
		return models.ErrInvalidJobID
	}

	// Verify the match result belongs to the specified job
	belongsToJob, err := s.jobRepo.MatchResultBelongsToJob(ctx, userID, matchID, jobID)
	if err != nil {
		s.log.Error().
			Int("job_id", jobID).
			Int("match_id", matchID).
			Err(err).
			Msg("Failed to verify match ownership")
		return err
	}

	if !belongsToJob {
		s.log.Error().
			Int("job_id", jobID).
			Int("match_id", matchID).
			Msg("Match result not found for this job")
		return models.ErrJobNotFound
	}

	err = s.jobRepo.DeleteMatchResult(ctx, userID, matchID)
	if err != nil {
		s.log.Error().
			Int("match_id", matchID).
			Err(err).
			Msg("Failed to delete match result")
		return err
	}

	s.log.Info().
		Int("job_id", jobID).
		Int("match_id", matchID).
		Msg("Match result deleted successfully")

	return nil
}
