package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"

	"github.com/benidevo/vega/internal/cache"
	commonerrors "github.com/benidevo/vega/internal/common/errors"
	"github.com/benidevo/vega/internal/common/logger"
	"github.com/benidevo/vega/internal/db/querybuilder"
	"github.com/benidevo/vega/internal/job/interfaces"
	"github.com/benidevo/vega/internal/job/models"
)

// scanner interface abstracts the common Scan method from *sql.Row and *sql.Rows
type scanner interface {
	Scan(dest ...any) error
}

// topMatchThreshold is the minimum match score (inclusive) for a job to appear
// in the "top matches" insights panel. Must stay in sync with GetStatsByUserID
// which also uses this value for the high_match counter.
const topMatchThreshold = 70

// jobColumns defines the columns for job queries with company join.
var jobColumns = []string{
	"j.id", "j.title", "j.description", "j.location", "j.job_type",
	"j.source_url", "j.required_skills",
	"j.application_url", "j.company_id", "j.status", "j.match_score",
	"j.notes", "j.created_at", "j.updated_at", "j.user_id", "j.first_analyzed_at",
	"c.name", "c.created_at", "c.updated_at",
}

// SQLiteJobRepository is a SQLite implementation of JobRepository
type SQLiteJobRepository struct {
	db                *sql.DB
	companyRepository interfaces.CompanyRepository
	cache             cache.Cache
	log               *logger.PrivacyLogger
}

func NewSQLiteJobRepository(db *sql.DB, companyRepository interfaces.CompanyRepository, cache cache.Cache) *SQLiteJobRepository {
	return &SQLiteJobRepository{
		db:                db,
		companyRepository: companyRepository,
		cache:             cache,
		log:               logger.GetPrivacyLogger("job_repository"),
	}
}

// BeginTx starts a new database transaction with the given options.
// The transaction must be either committed or rolled back.
func (r *SQLiteJobRepository) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return r.db.BeginTx(ctx, opts)
}

// CommitTx commits the given transaction.
func (r *SQLiteJobRepository) CommitTx(tx *sql.Tx) error {
	return tx.Commit()
}

// RollbackTx rolls back the given transaction.
func (r *SQLiteJobRepository) RollbackTx(tx *sql.Tx) error {
	return tx.Rollback()
}

// invalidateCache deletes cache keys and logs any failures for observability.
// Cache invalidation is best-effort; failures are logged but don't cause errors.
func (r *SQLiteJobRepository) invalidateCache(ctx context.Context, keys ...string) {
	if err := r.cache.Delete(ctx, keys...); err != nil {
		r.log.Warn().
			Err(err).
			Strs("keys", keys).
			Msg("Failed to invalidate cache")
	}
}

func (r *SQLiteJobRepository) invalidateCachePattern(ctx context.Context, pattern string) {
	if err := r.cache.DeletePattern(ctx, pattern); err != nil {
		r.log.Warn().
			Err(err).
			Str("pattern", pattern).
			Msg("Failed to invalidate cache pattern")
	}
}

func (r *SQLiteJobRepository) prepareJobInsert(jobModel *models.Job, userID int) (string, []interface{}, error) {
	now := time.Now().UTC()
	if jobModel.CreatedAt.IsZero() {
		jobModel.CreatedAt = now
	}
	jobModel.UpdatedAt = now
	if jobModel.RequiredSkills == nil {
		jobModel.RequiredSkills = []string{}
	}

	skillsJSON, err := json.Marshal(jobModel.RequiredSkills)
	if err != nil {
		return "", nil, models.WrapError(models.ErrFailedToCreateJob, err)
	}

	query, args, err := querybuilder.Insert("jobs").
		Columns(
			"title", "description", "location", "job_type", "source_url",
			"required_skills", "application_url",
			"company_id", "status", "notes",
			"created_at", "updated_at", "user_id",
		).
		Values(
			jobModel.Title, jobModel.Description, jobModel.Location,
			int(jobModel.JobType), jobModel.SourceURL,
			skillsJSON, jobModel.ApplicationURL,
			jobModel.CompanyID, int(jobModel.Status), jobModel.Notes,
			jobModel.CreatedAt, jobModel.UpdatedAt, userID,
		).
		ToSql()
	if err != nil {
		return "", nil, models.WrapError(models.ErrFailedToCreateJob, err)
	}

	return query, args, nil
}

// CreateWithTx creates a new job within a transaction.
// The job's company must already exist in the database.
func (r *SQLiteJobRepository) CreateWithTx(ctx context.Context, tx *sql.Tx, userID int, jobModel *models.Job) (*models.Job, error) {
	if err := validateJob(jobModel); err != nil {
		return nil, err
	}

	query, args, err := r.prepareJobInsert(jobModel, userID)
	if err != nil {
		return nil, err
	}

	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, models.WrapError(models.ErrFailedToCreateJob, err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, models.WrapError(models.ErrFailedToCreateJob, err)
	}

	jobModel.ID = int(id)
	jobModel.UserID = userID
	return jobModel, nil
}

// UpdateWithTx updates an existing job within a transaction.
// Returns ErrJobNotFound if the job doesn't exist or doesn't belong to the user.
func (r *SQLiteJobRepository) UpdateWithTx(ctx context.Context, tx *sql.Tx, userID int, job *models.Job) error {
	if err := validateJob(job); err != nil {
		return err
	}

	job.UpdatedAt = time.Now().UTC()

	skillsJSON, err := json.Marshal(job.RequiredSkills)
	if err != nil {
		return models.WrapError(models.ErrFailedToUpdateJob, err)
	}

	query, args, err := querybuilder.Update("jobs").
		Set("title", job.Title).
		Set("description", job.Description).
		Set("location", job.Location).
		Set("job_type", int(job.JobType)).
		Set("source_url", job.SourceURL).
		Set("required_skills", skillsJSON).
		Set("application_url", job.ApplicationURL).
		Set("company_id", job.CompanyID).
		Set("status", int(job.Status)).
		Set("match_score", job.MatchScore).
		Set("notes", job.Notes).
		Set("updated_at", job.UpdatedAt).
		Where(sq.Eq{"id": job.ID, "user_id": userID}).
		ToSql()
	if err != nil {
		return models.WrapError(models.ErrFailedToUpdateJob, err)
	}

	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return models.WrapError(models.ErrFailedToUpdateJob, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return models.WrapError(models.ErrFailedToUpdateJob, err)
	}

	if rowsAffected == 0 {
		return models.ErrJobNotFound
	}

	return nil
}

// DeleteWithTx deletes a job within a transaction.
// Returns ErrJobNotFound if the job doesn't exist or doesn't belong to the user.
func (r *SQLiteJobRepository) DeleteWithTx(ctx context.Context, tx *sql.Tx, userID int, id int) error {
	query, args, err := querybuilder.Delete("jobs").
		Where(sq.Eq{"id": id, "user_id": userID}).
		ToSql()
	if err != nil {
		return models.WrapError(models.ErrFailedToDeleteJob, err)
	}

	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return models.WrapError(models.ErrFailedToDeleteJob, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return models.WrapError(models.ErrFailedToDeleteJob, err)
	}

	if rowsAffected == 0 {
		return models.ErrJobNotFound
	}

	return nil
}

// validateJob performs basic validation on a job
func validateJob(jobModel *models.Job) error {
	if jobModel == nil {
		return models.ErrInvalidJobID
	}

	return jobModel.Validate()
}

// scanJob scans a job from any scanner (row or rows) and converts it to a Job model
func (r *SQLiteJobRepository) scanJob(s scanner) (*models.Job, error) {
	var j models.Job
	var company models.Company
	var skillsJSON string
	var jobType, status int
	var matchScore sql.NullInt64
	var notes, sourceURL, applicationURL, location sql.NullString
	var firstAnalyzedAt sql.NullTime

	err := s.Scan(
		&j.ID, &j.Title, &j.Description, &location, &jobType,
		&sourceURL, &skillsJSON,
		&applicationURL, &company.ID, &status, &matchScore,
		&notes, &j.CreatedAt, &j.UpdatedAt, &j.UserID, &firstAnalyzedAt,
		&company.Name, &company.CreatedAt, &company.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if location.Valid {
		j.Location = location.String
	}
	if sourceURL.Valid {
		j.SourceURL = sourceURL.String
	}
	if applicationURL.Valid {
		j.ApplicationURL = applicationURL.String
	}
	if notes.Valid {
		j.Notes = notes.String
	}
	if matchScore.Valid {
		score := int(matchScore.Int64)
		j.MatchScore = &score
	}
	if firstAnalyzedAt.Valid {
		j.FirstAnalyzedAt = &firstAnalyzedAt.Time
	}

	if err := json.Unmarshal([]byte(skillsJSON), &j.RequiredSkills); err != nil {
		j.RequiredSkills = []string{}
	}

	j.JobType = models.JobType(jobType)
	j.Status = models.JobStatus(status)
	j.Company = company

	return &j, nil
}

func (r *SQLiteJobRepository) baseJobSelect() sq.SelectBuilder {
	return querybuilder.Select(jobColumns...).
		From("jobs j").
		Join("companies c ON j.company_id = c.id")
}

// GetOrCreate retrieves a job by its SourceURL or creates it if it does not exist.
// Returns the job, a boolean indicating if it was newly created, and any error.
func (r *SQLiteJobRepository) GetOrCreate(ctx context.Context, userID int, jobModel *models.Job) (*models.Job, bool, error) {
	if err := validateJob(jobModel); err != nil {
		return nil, false, err
	}

	if jobModel.SourceURL == "" {
		return nil, false, models.ErrInvalidJobID
	}

	existingJob, err := r.GetBySourceURL(ctx, userID, jobModel.SourceURL)
	if err == nil {
		return existingJob, false, nil
	}

	company, err := r.companyRepository.GetOrCreate(ctx, jobModel.Company.Name)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get/create company for job: %w", err)
	}
	jobModel.CompanyID = company.ID

	query, args, err := r.prepareJobInsert(jobModel, userID)
	if err != nil {
		return nil, false, err
	}

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") ||
			strings.Contains(err.Error(), "duplicate key value") {
			existingJob, err := r.GetBySourceURL(ctx, userID, jobModel.SourceURL)
			if err != nil {
				return nil, false, err
			}
			return existingJob, false, nil
		}
		return nil, false, models.WrapError(models.ErrFailedToCreateJob, err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, false, models.WrapError(models.ErrFailedToCreateJob, err)
	}

	jobModel.ID = int(id)
	jobModel.UserID = userID
	jobModel.Company = *company

	r.invalidateCache(ctx,
		fmt.Sprintf("stats:u%d:summary", userID),
		fmt.Sprintf("stats:u%d:by-status", userID),
	)

	return jobModel, true, nil
}

func (r *SQLiteJobRepository) GetBySourceURL(ctx context.Context, userID int, sourceURL string) (*models.Job, error) {
	if sourceURL == "" {
		return nil, models.ErrInvalidJobID
	}

	query, args, err := r.baseJobSelect().
		Where(sq.Eq{"j.source_url": sourceURL, "j.user_id": userID}).
		ToSql()
	if err != nil {
		return nil, models.WrapError(models.ErrFailedToGetJob, err)
	}

	row := r.db.QueryRowContext(ctx, query, args...)

	job, err := r.scanJob(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, models.ErrJobNotFound
		}
		return nil, models.WrapError(models.ErrFailedToGetJob, err)
	}

	return job, nil
}

func (r *SQLiteJobRepository) Create(ctx context.Context, userID int, jobModel *models.Job) (*models.Job, error) {
	if err := validateJob(jobModel); err != nil {
		return nil, err
	}
	company, err := r.companyRepository.GetOrCreate(ctx, jobModel.Company.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get/create company for job creation: %w", err)
	}
	jobModel.CompanyID = company.ID

	query, args, err := r.prepareJobInsert(jobModel, userID)
	if err != nil {
		return nil, err
	}

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, models.WrapError(models.ErrFailedToCreateJob, err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, models.WrapError(models.ErrFailedToCreateJob, err)
	}

	jobModel.ID = int(id)
	jobModel.Company = *company

	r.invalidateCache(ctx,
		fmt.Sprintf("stats:u%d:summary", userID),
		fmt.Sprintf("stats:u%d:by-status", userID),
	)

	return jobModel, nil
}

func (r *SQLiteJobRepository) GetByID(ctx context.Context, userID int, id int) (*models.Job, error) {
	if id <= 0 {
		return nil, models.ErrInvalidJobID
	}

	cacheKey := fmt.Sprintf("job:u%d:id%d", userID, id)
	var job models.Job
	if err := r.cache.Get(ctx, cacheKey, &job); err == nil {
		return &job, nil
	}

	query, args, err := r.baseJobSelect().
		Where(sq.Eq{"j.id": id, "j.user_id": userID}).
		ToSql()
	if err != nil {
		return nil, models.WrapError(models.ErrFailedToGetJob, err)
	}

	row := r.db.QueryRowContext(ctx, query, args...)

	jobResult, err := r.scanJob(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, models.ErrJobNotFound
		}
		return nil, &commonerrors.RepositoryError{
			SentinelError: models.ErrJobNotFound,
			InnerError:    err,
		}
	}

	_ = r.cache.Set(ctx, cacheKey, jobResult, time.Hour)

	return jobResult, nil
}

func (r *SQLiteJobRepository) applyJobFilters(builder sq.SelectBuilder, userID int, filter models.JobFilter) sq.SelectBuilder {
	builder = builder.Where(sq.Eq{"j.user_id": userID})

	if filter.CompanyID != nil {
		builder = builder.Where(sq.Eq{"j.company_id": *filter.CompanyID})
	}

	if filter.Status != nil {
		builder = builder.Where(sq.Eq{"j.status": int(*filter.Status)})
	}

	if len(filter.ExcludeStatuses) > 0 {
		excludeInts := make([]int, len(filter.ExcludeStatuses))
		for i, status := range filter.ExcludeStatuses {
			excludeInts[i] = int(status)
		}
		builder = builder.Where(sq.NotEq{"j.status": excludeInts})
	}

	if filter.JobType != nil {
		builder = builder.Where(sq.Eq{"j.job_type": int(*filter.JobType)})
	}

	if filter.Matched != nil {
		if *filter.Matched {
			builder = builder.Where(sq.GtOrEq{"j.match_score": 70})
		} else {
			builder = builder.Where(sq.Or{
				sq.Eq{"j.match_score": nil},
				sq.Lt{"j.match_score": 70},
			})
		}
	}

	return builder
}

func (r *SQLiteJobRepository) GetAll(ctx context.Context, userID int, filter models.JobFilter) ([]*models.Job, error) {
	builder := r.applyJobFilters(r.baseJobSelect(), userID, filter)

	// Validate sort parameters for defense-in-depth
	validSortFields := map[string]bool{
		"match_score": true,
		"created_at":  true,
		"updated_at":  true,
		"":            true, // Allow empty for default
	}

	if !validSortFields[filter.SortBy] {
		filter.SortBy = "" // Default to updated_at
	}

	if filter.SortOrder != "asc" && filter.SortOrder != "desc" && filter.SortOrder != "" {
		filter.SortOrder = "desc"
	}

	// Apply ordering
	switch filter.SortBy {
	case "match_score":
		// Sort by match_score, with NULL values last
		if filter.SortOrder == "asc" {
			builder = builder.OrderBy("CASE WHEN j.match_score IS NULL THEN 1 ELSE 0 END", "j.match_score ASC")
		} else {
			builder = builder.OrderBy("CASE WHEN j.match_score IS NULL THEN 1 ELSE 0 END", "j.match_score DESC")
		}
	case "created_at":
		if filter.SortOrder == "asc" {
			builder = builder.OrderBy("j.created_at ASC")
		} else {
			builder = builder.OrderBy("j.created_at DESC")
		}
	case "updated_at":
		if filter.SortOrder == "asc" {
			builder = builder.OrderBy("j.updated_at ASC")
		} else {
			builder = builder.OrderBy("j.updated_at DESC")
		}
	default:
		builder = builder.OrderBy("j.updated_at DESC")
	}

	// Apply pagination
	if filter.Limit > 0 {
		builder = builder.Limit(uint64(filter.Limit))
		if filter.Offset > 0 {
			builder = builder.Offset(uint64(filter.Offset))
		}
	}

	query, args, err := builder.ToSql()
	if err != nil {
		return nil, models.WrapError(models.ErrFailedToGetJob, err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*models.Job

	for rows.Next() {
		job, err := r.scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return jobs, nil
}

func (r *SQLiteJobRepository) Update(ctx context.Context, userID int, job *models.Job) error {
	if job == nil {
		return models.ErrInvalidJobID
	}

	if job.ID <= 0 {
		return models.ErrInvalidJobID
	}

	if err := validateJob(job); err != nil {
		return err
	}

	company, err := r.companyRepository.GetOrCreate(ctx, job.Company.Name)
	if err != nil {
		return fmt.Errorf("failed to get/create company for job update: %w", err)
	}

	job.UpdatedAt = time.Now().UTC()

	skillsJSON, err := json.Marshal(job.RequiredSkills)
	if err != nil {
		return models.WrapError(models.ErrFailedToUpdateJob, err)
	}

	query, args, err := querybuilder.Update("jobs").
		Set("title", job.Title).
		Set("description", job.Description).
		Set("location", job.Location).
		Set("job_type", int(job.JobType)).
		Set("source_url", job.SourceURL).
		Set("required_skills", skillsJSON).
		Set("application_url", job.ApplicationURL).
		Set("company_id", company.ID).
		Set("status", int(job.Status)).
		Set("match_score", job.MatchScore).
		Set("notes", job.Notes).
		Set("updated_at", job.UpdatedAt).
		Where(sq.Eq{"id": job.ID, "user_id": userID}).
		ToSql()
	if err != nil {
		return models.WrapError(models.ErrFailedToUpdateJob, err)
	}

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return models.WrapError(models.ErrFailedToUpdateJob, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return models.ErrFailedToUpdateJob
	}

	if rowsAffected == 0 {
		return models.ErrJobNotFound
	}

	job.Company = *company

	r.invalidateCache(ctx,
		fmt.Sprintf("job:u%d:id%d", userID, job.ID),
		fmt.Sprintf("stats:u%d:summary", userID),
		fmt.Sprintf("stats:u%d:by-status", userID),
	)

	return nil
}

func (r *SQLiteJobRepository) UpdateMatchScore(ctx context.Context, userID int, jobID int, matchScore *int) error {
	if jobID <= 0 {
		return models.ErrInvalidJobID
	}

	query, args, err := querybuilder.Update("jobs").
		Set("match_score", matchScore).
		Set("updated_at", time.Now().UTC()).
		Where(sq.Eq{"id": jobID, "user_id": userID}).
		ToSql()
	if err != nil {
		return models.WrapError(models.ErrFailedToUpdateJob, err)
	}

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return models.WrapError(models.ErrFailedToUpdateJob, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return models.ErrFailedToUpdateJob
	}

	if rowsAffected == 0 {
		return models.ErrJobNotFound
	}

	r.invalidateCachePattern(ctx, fmt.Sprintf("top-matches:u%d:*", userID))
	r.invalidateCache(ctx,
		fmt.Sprintf("job:u%d:id%d", userID, jobID),
		fmt.Sprintf("stats:u%d:summary", userID),
	)

	return nil
}

func (r *SQLiteJobRepository) Delete(ctx context.Context, userID int, id int) error {
	if id <= 0 {
		return models.ErrInvalidJobID
	}

	query, args, err := querybuilder.Delete("jobs").
		Where(sq.Eq{"id": id, "user_id": userID}).
		ToSql()
	if err != nil {
		return models.WrapError(models.ErrFailedToDeleteJob, err)
	}

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return models.WrapError(models.ErrFailedToDeleteJob, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return models.WrapError(models.ErrFailedToDeleteJob, err)
	}

	if rowsAffected == 0 {
		return models.ErrJobNotFound
	}

	r.invalidateCache(ctx,
		fmt.Sprintf("job:u%d:id%d", userID, id),
		fmt.Sprintf("stats:u%d:summary", userID),
		fmt.Sprintf("stats:u%d:by-status", userID),
	)

	return nil
}

func (r *SQLiteJobRepository) UpdateStatus(ctx context.Context, userID int, id int, status models.JobStatus) error {
	if id <= 0 {
		return models.ErrInvalidJobID
	}

	if status < models.INTERESTED || status > models.NOT_INTERESTED {
		return models.ErrInvalidJobStatus
	}

	query, args, err := querybuilder.Update("jobs").
		Set("status", int(status)).
		Set("updated_at", time.Now().UTC()).
		Where(sq.Eq{"id": id, "user_id": userID}).
		ToSql()
	if err != nil {
		return models.WrapError(models.ErrFailedToUpdateJob, err)
	}

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return models.WrapError(models.ErrFailedToUpdateJob, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return models.WrapError(models.ErrFailedToUpdateJob, err)
	}

	if rowsAffected == 0 {
		return models.ErrJobNotFound
	}

	r.invalidateCache(ctx,
		fmt.Sprintf("stats:u%d:summary", userID),
		fmt.Sprintf("stats:u%d:by-status", userID),
		fmt.Sprintf("job:u%d:id%d", userID, id),
	)

	return nil
}

func (r *SQLiteJobRepository) GetCount(ctx context.Context, userID int, filter models.JobFilter) (int, error) {
	builder := querybuilder.Select("COUNT(*)").
		From("jobs j").
		Join("companies c ON j.company_id = c.id")

	builder = r.applyJobFilters(builder, userID, filter)

	query, args, err := builder.ToSql()
	if err != nil {
		return 0, models.WrapError(models.ErrFailedToGetJobStats, err)
	}

	var count int
	err = r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, models.WrapError(models.ErrFailedToGetJobStats, err)
	}

	return count, nil
}

// GetStats returns aggregate statistics about jobs in the database.
func (r *SQLiteJobRepository) GetStats(ctx context.Context, userID int) (*models.JobStats, error) {
	query, args, err := querybuilder.Select(
		"COUNT(*) AS total_jobs",
		"COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS applied",
		fmt.Sprintf("COALESCE(SUM(CASE WHEN match_score >= %d THEN 1 ELSE 0 END), 0) AS high_match", topMatchThreshold),
	).
		From("jobs").
		Where(sq.Eq{"user_id": userID}).
		ToSql()
	if err != nil {
		return nil, models.WrapError(models.ErrFailedToGetJobStats, err)
	}

	// Inject the APPLIED status value into the args at the beginning
	fullArgs := append([]any{int(models.APPLIED)}, args...)

	rows, err := r.db.QueryContext(ctx, query, fullArgs...)
	if err != nil {
		return nil, models.WrapError(models.ErrFailedToGetJobStats, err)
	}
	defer rows.Close()

	var stats models.JobStats
	if rows.Next() {
		if err := rows.Scan(&stats.TotalJobs, &stats.TotalApplied, &stats.HighMatch); err != nil {
			return nil, models.WrapError(models.ErrFailedToGetJobStats, err)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, models.WrapError(models.ErrFailedToGetJobStats, err)
	}
	return &stats, nil
}

// GetStatsByUserID returns aggregate statistics about jobs for a specific user.
func (r *SQLiteJobRepository) GetStatsByUserID(ctx context.Context, userID int) (*models.JobStats, error) {
	cacheKey := fmt.Sprintf("stats:u%d:summary", userID)
	var stats models.JobStats
	if err := r.cache.Get(ctx, cacheKey, &stats); err == nil {
		return &stats, nil
	}

	query, args, err := querybuilder.Select(
		"COUNT(*) AS total_jobs",
		"COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS applied",
		fmt.Sprintf("COALESCE(SUM(CASE WHEN match_score >= %d THEN 1 ELSE 0 END), 0) AS high_match", topMatchThreshold),
	).
		From("jobs").
		Where(sq.Eq{"user_id": userID}).
		ToSql()
	if err != nil {
		return nil, models.WrapError(models.ErrFailedToGetJobStats, err)
	}

	fullArgs := append([]any{int(models.APPLIED)}, args...)

	rows, err := r.db.QueryContext(ctx, query, fullArgs...)
	if err != nil {
		return nil, models.WrapError(models.ErrFailedToGetJobStats, err)
	}
	defer rows.Close()

	if rows.Next() {
		if err := rows.Scan(&stats.TotalJobs, &stats.TotalApplied, &stats.HighMatch); err != nil {
			return nil, models.WrapError(models.ErrFailedToGetJobStats, err)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, models.WrapError(models.ErrFailedToGetJobStats, err)
	}

	_ = r.cache.Set(ctx, cacheKey, &stats, 10*time.Minute)

	return &stats, nil
}

// GetRecentJobsByUserID returns recent jobs for a specific user, limited by count.
// Jobs are ordered by updated_at DESC to show most recently modified first.
func (r *SQLiteJobRepository) GetRecentJobsByUserID(ctx context.Context, userID int, limit int) ([]*models.Job, error) {
	if limit <= 0 {
		limit = 10 // Default limit
	}

	filter := models.JobFilter{
		Limit: limit,
	}

	jobs, err := r.GetAll(ctx, userID, filter)
	if err != nil {
		return nil, err
	}

	return jobs, nil
}

// GetTopMatchesByUserID returns jobs with the highest match scores for a specific user.
func (r *SQLiteJobRepository) GetTopMatchesByUserID(ctx context.Context, userID int, limit int) ([]*models.Job, error) {
	if limit <= 0 {
		limit = 10
	}

	cacheKey := fmt.Sprintf("top-matches:u%d:limit%d", userID, limit)
	var cached []*models.Job
	if err := r.cache.Get(ctx, cacheKey, &cached); err == nil {
		return cached, nil
	}

	query := sq.Select(jobColumns...).
		From("jobs j").
		LeftJoin("companies c ON j.company_id = c.id").
		Where(sq.Eq{"j.user_id": userID}).
		Where(sq.GtOrEq{"j.match_score": topMatchThreshold}).
		OrderBy("j.match_score DESC", "j.updated_at DESC").
		Limit(uint64(limit))

	sqlStr, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build top matches query: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query top matches: %w", err)
	}
	defer rows.Close()

	var jobs []*models.Job
	for rows.Next() {
		job, err := r.scanJob(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan job: %w", err)
		}
		jobs = append(jobs, job)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate top matches: %w", err)
	}

	_ = r.cache.Set(ctx, cacheKey, jobs, 5*time.Minute)

	return jobs, nil
}

// GetJobStatsByStatus returns job counts grouped by status for a specific user.
// This is useful for homepage pipeline visualization.
func (r *SQLiteJobRepository) GetJobStatsByStatus(ctx context.Context, userID int) (map[models.JobStatus]int, error) {
	cacheKey := fmt.Sprintf("stats:u%d:by-status", userID)
	var statusCounts map[models.JobStatus]int
	if err := r.cache.Get(ctx, cacheKey, &statusCounts); err == nil {
		return statusCounts, nil
	}

	query, args, err := querybuilder.Select("status", "COUNT(*) as count").
		From("jobs").
		Where(sq.Eq{"user_id": userID}).
		GroupBy("status").
		OrderBy("status").
		ToSql()
	if err != nil {
		return nil, models.WrapError(models.ErrFailedToGetJobStats, err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, models.WrapError(models.ErrFailedToGetJobStats, err)
	}
	defer rows.Close()

	statusCounts = make(map[models.JobStatus]int)

	for status := models.INTERESTED; status <= models.NOT_INTERESTED; status++ {
		statusCounts[status] = 0
	}

	for rows.Next() {
		var status int
		var count int

		if err := rows.Scan(&status, &count); err != nil {
			return nil, models.WrapError(models.ErrFailedToGetJobStats, err)
		}

		if status >= int(models.INTERESTED) && status <= int(models.NOT_INTERESTED) {
			statusCounts[models.JobStatus(status)] = count
		}
	}

	if err := rows.Err(); err != nil {
		return nil, models.WrapError(models.ErrFailedToGetJobStats, err)
	}

	_ = r.cache.Set(ctx, cacheKey, statusCounts, 10*time.Minute)

	return statusCounts, nil
}

func (r *SQLiteJobRepository) CreateMatchResult(ctx context.Context, userID int, matchResult *models.MatchResult) error {
	if matchResult == nil {
		return models.ErrInvalidJobID
	}

	strengthsJSON, err := json.Marshal(matchResult.Strengths)
	if err != nil {
		return models.WrapError(models.ErrFailedToCreateJob, err)
	}

	weaknessesJSON, err := json.Marshal(matchResult.Weaknesses)
	if err != nil {
		return models.WrapError(models.ErrFailedToCreateJob, err)
	}

	highlightsJSON, err := json.Marshal(matchResult.Highlights)
	if err != nil {
		return models.WrapError(models.ErrFailedToCreateJob, err)
	}

	query, args, err := querybuilder.Insert("match_results").
		Columns("job_id", "match_score", "strengths", "weaknesses", "highlights", "feedback", "user_id").
		Values(
			matchResult.JobID, matchResult.MatchScore,
			string(strengthsJSON), string(weaknessesJSON), string(highlightsJSON),
			matchResult.Feedback, userID,
		).
		ToSql()
	if err != nil {
		return models.WrapError(models.ErrFailedToCreateJob, err)
	}

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return models.WrapError(models.ErrFailedToCreateJob, err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return models.WrapError(models.ErrFailedToCreateJob, err)
	}

	matchResult.ID = int(id)

	r.invalidateCache(ctx,
		fmt.Sprintf("match:u%d:job%d:history", userID, matchResult.JobID),
	)
	r.invalidateCachePattern(ctx, fmt.Sprintf("match:u%d:recent:*", userID))

	return nil
}

func (r *SQLiteJobRepository) GetJobMatchHistory(ctx context.Context, userID int, jobID int) ([]*models.MatchResult, error) {
	cacheKey := fmt.Sprintf("match:u%d:job%d:history", userID, jobID)
	var results []*models.MatchResult
	if err := r.cache.Get(ctx, cacheKey, &results); err == nil {
		return results, nil
	}

	query, args, err := querybuilder.Select(
		"id", "job_id", "match_score", "strengths", "weaknesses", "highlights", "feedback", "created_at",
	).
		From("match_results").
		Where(sq.Eq{"job_id": jobID, "user_id": userID}).
		OrderBy("created_at DESC").
		ToSql()
	if err != nil {
		return nil, models.WrapError(models.ErrFailedToGetJob, err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, models.WrapError(models.ErrFailedToGetJob, err)
	}
	defer rows.Close()

	results = []*models.MatchResult{}
	for rows.Next() {
		var mr models.MatchResult
		var strengthsJSON, weaknessesJSON, highlightsJSON string

		err := rows.Scan(
			&mr.ID,
			&mr.JobID,
			&mr.MatchScore,
			&strengthsJSON,
			&weaknessesJSON,
			&highlightsJSON,
			&mr.Feedback,
			&mr.CreatedAt,
		)
		if err != nil {
			return nil, models.WrapError(models.ErrFailedToGetJob, err)
		}

		if err := json.Unmarshal([]byte(strengthsJSON), &mr.Strengths); err != nil {
			mr.Strengths = []string{}
		}
		if err := json.Unmarshal([]byte(weaknessesJSON), &mr.Weaknesses); err != nil {
			mr.Weaknesses = []string{}
		}
		if err := json.Unmarshal([]byte(highlightsJSON), &mr.Highlights); err != nil {
			mr.Highlights = []string{}
		}

		results = append(results, &mr)
	}

	if err := rows.Err(); err != nil {
		return nil, models.WrapError(models.ErrFailedToGetJob, err)
	}

	_ = r.cache.Set(ctx, cacheKey, results, 30*time.Minute)

	return results, nil
}

// GetRecentMatchResultsWithDetails retrieves recent match results with job details for context
func (r *SQLiteJobRepository) GetRecentMatchResultsWithDetails(ctx context.Context, userID int, limit int, currentJobID int) ([]*models.MatchSummary, error) {
	if limit <= 0 {
		limit = 5
	}

	cte := `WITH current_company AS (
		SELECT c.name as company_name
		FROM jobs j
		JOIN companies c ON j.company_id = c.id
		WHERE j.id = ?
	)`

	query, args, err := querybuilder.Select(
		"mr.job_id", "j.title", "c.name", "mr.match_score",
		"mr.strengths", "mr.weaknesses", "mr.created_at",
	).
		Prefix(cte, currentJobID).
		From("match_results mr").
		Join("jobs j ON mr.job_id = j.id").
		Join("companies c ON j.company_id = c.id").
		LeftJoin("current_company cc ON 1=1").
		Where(sq.NotEq{"mr.job_id": currentJobID}).
		Where(sq.Eq{"mr.user_id": userID}).
		OrderBy("CASE WHEN c.name = cc.company_name THEN 0 ELSE 1 END", "mr.created_at DESC").
		Limit(uint64(limit)).
		ToSql()
	if err != nil {
		return nil, models.WrapError(models.ErrFailedToGetJob, err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, models.WrapError(models.ErrFailedToGetJob, err)
	}
	defer rows.Close()

	var summaries []*models.MatchSummary
	for rows.Next() {
		var jobID int
		var jobTitle, companyName string
		var matchScore int
		var strengthsJSON, weaknessesJSON string
		var createdAt time.Time

		err := rows.Scan(
			&jobID,
			&jobTitle,
			&companyName,
			&matchScore,
			&strengthsJSON,
			&weaknessesJSON,
			&createdAt,
		)
		if err != nil {
			return nil, models.WrapError(models.ErrFailedToGetJob, err)
		}

		var strengths, weaknesses []string
		if err := json.Unmarshal([]byte(strengthsJSON), &strengths); err != nil {
			strengths = []string{}
		}
		if err := json.Unmarshal([]byte(weaknessesJSON), &weaknesses); err != nil {
			weaknesses = []string{}
		}

		var insights []string
		if len(strengths) > 0 {
			insights = append(insights, "Strengths: "+strengths[0])
			if len(strengths) > 1 {
				insights = append(insights, strengths[1])
			}
		}
		if len(weaknesses) > 0 {
			insights = append(insights, "Gap: "+weaknesses[0])
		}

		daysAgo := int(time.Since(createdAt).Hours() / 24)

		summary := &models.MatchSummary{
			JobTitle:    jobTitle,
			Company:     companyName,
			MatchScore:  matchScore,
			KeyInsights: strings.Join(insights, "; "),
			DaysAgo:     daysAgo,
		}

		summaries = append(summaries, summary)
	}

	if err := rows.Err(); err != nil {
		return nil, models.WrapError(models.ErrFailedToGetJob, err)
	}

	return summaries, nil
}

func (r *SQLiteJobRepository) GetRecentMatchResults(ctx context.Context, userID int, limit int) ([]*models.MatchResult, error) {
	if limit <= 0 {
		limit = 10
	}

	cacheKey := fmt.Sprintf("match:u%d:recent:%d", userID, limit)
	var results []*models.MatchResult
	if err := r.cache.Get(ctx, cacheKey, &results); err == nil {
		return results, nil
	}

	query := `SELECT mr.id, mr.job_id, mr.match_score, mr.strengths, mr.weaknesses,
		mr.highlights, mr.feedback, mr.created_at
		FROM match_results mr WHERE mr.user_id = ? ORDER BY mr.created_at DESC LIMIT ?`

	rows, err := r.db.QueryContext(ctx, query, userID, limit)
	if err != nil {
		return nil, models.WrapError(models.ErrFailedToGetJob, err)
	}
	defer rows.Close()

	results = []*models.MatchResult{}
	for rows.Next() {
		var mr models.MatchResult
		var strengthsJSON, weaknessesJSON, highlightsJSON string

		err := rows.Scan(
			&mr.ID,
			&mr.JobID,
			&mr.MatchScore,
			&strengthsJSON,
			&weaknessesJSON,
			&highlightsJSON,
			&mr.Feedback,
			&mr.CreatedAt,
		)
		if err != nil {
			return nil, models.WrapError(models.ErrFailedToGetJob, err)
		}

		if err := json.Unmarshal([]byte(strengthsJSON), &mr.Strengths); err != nil {
			mr.Strengths = []string{}
		}
		if err := json.Unmarshal([]byte(weaknessesJSON), &mr.Weaknesses); err != nil {
			mr.Weaknesses = []string{}
		}
		if err := json.Unmarshal([]byte(highlightsJSON), &mr.Highlights); err != nil {
			mr.Highlights = []string{}
		}

		results = append(results, &mr)
	}

	if err := rows.Err(); err != nil {
		return nil, models.WrapError(models.ErrFailedToGetJob, err)
	}

	_ = r.cache.Set(ctx, cacheKey, results, 10*time.Minute)

	return results, nil
}

func (r *SQLiteJobRepository) DeleteMatchResult(ctx context.Context, userID int, matchID int) error {
	if matchID <= 0 {
		return models.ErrInvalidJobID
	}

	var jobID int
	selectQuery, selectArgs, err := querybuilder.Select("job_id").
		From("match_results").
		Where(sq.Eq{"id": matchID, "user_id": userID}).
		ToSql()
	if err != nil {
		return models.WrapError(models.ErrFailedToDeleteJob, err)
	}

	err = r.db.QueryRowContext(ctx, selectQuery, selectArgs...).Scan(&jobID)
	if err != nil {
		if err == sql.ErrNoRows {
			return models.ErrJobNotFound
		}
		return models.WrapError(models.ErrFailedToDeleteJob, err)
	}

	query, args, err := querybuilder.Delete("match_results").
		Where(sq.Eq{"id": matchID, "user_id": userID}).
		ToSql()
	if err != nil {
		return models.WrapError(models.ErrFailedToDeleteJob, err)
	}

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return models.WrapError(models.ErrFailedToDeleteJob, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return models.WrapError(models.ErrFailedToDeleteJob, err)
	}

	if rowsAffected == 0 {
		return models.ErrJobNotFound
	}

	r.invalidateCache(ctx,
		fmt.Sprintf("match:u%d:job%d:history", userID, jobID),
	)
	r.invalidateCachePattern(ctx, fmt.Sprintf("match:u%d:recent:*", userID))

	return nil
}

func (r *SQLiteJobRepository) MatchResultBelongsToJob(ctx context.Context, userID int, matchID, jobID int) (bool, error) {
	if matchID <= 0 || jobID <= 0 {
		return false, models.ErrInvalidJobID
	}

	query, args, err := querybuilder.Select("1").
		Prefix("SELECT EXISTS(").
		From("match_results").
		Where(sq.Eq{"id": matchID, "job_id": jobID, "user_id": userID}).
		Suffix(")").
		ToSql()
	if err != nil {
		return false, models.WrapError(models.ErrFailedToGetJob, err)
	}

	var exists bool
	err = r.db.QueryRowContext(ctx, query, args...).Scan(&exists)
	if err != nil {
		return false, models.WrapError(models.ErrFailedToGetJob, err)
	}

	return exists, nil
}

func (r *SQLiteJobRepository) GetMonthlyAnalysisCount(ctx context.Context, userID int) (int, error) {
	now := time.Now().UTC()
	firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	query, args, err := querybuilder.Select("COUNT(*)").
		From("jobs").
		Where(sq.Eq{"user_id": userID}).
		Where(sq.NotEq{"first_analyzed_at": nil}).
		Where(sq.GtOrEq{"first_analyzed_at": firstOfMonth}).
		ToSql()
	if err != nil {
		return 0, models.WrapError(models.ErrFailedToGetJob, err)
	}

	var count int
	err = r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, models.WrapError(models.ErrFailedToGetJob, err)
	}

	return count, nil
}

func (r *SQLiteJobRepository) SetFirstAnalyzedAt(ctx context.Context, jobID int) error {
	query := `
		UPDATE jobs
		SET first_analyzed_at = CURRENT_TIMESTAMP
		WHERE id = ?
		AND first_analyzed_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, jobID)
	if err != nil {
		return models.WrapError(models.ErrFailedToUpdateJob, err)
	}

	_, err = result.RowsAffected()
	if err != nil {
		return models.WrapError(models.ErrFailedToUpdateJob, err)
	}

	return nil
}

// SetFirstAnalyzedAtWithTx sets the first_analyzed_at timestamp within a transaction.
// Only updates if first_analyzed_at is currently NULL.
func (r *SQLiteJobRepository) SetFirstAnalyzedAtWithTx(ctx context.Context, tx *sql.Tx, jobID int) error {
	// Using raw SQL for CURRENT_TIMESTAMP and conditional update
	query := `
		UPDATE jobs
		SET first_analyzed_at = CURRENT_TIMESTAMP
		WHERE id = ?
		AND first_analyzed_at IS NULL
	`

	result, err := tx.ExecContext(ctx, query, jobID)
	if err != nil {
		return models.WrapError(models.ErrFailedToUpdateJob, err)
	}

	_, err = result.RowsAffected()
	if err != nil {
		return models.WrapError(models.ErrFailedToUpdateJob, err)
	}

	return nil
}
