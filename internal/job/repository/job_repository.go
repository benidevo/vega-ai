package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/benidevo/vega/internal/cache"
	commonerrors "github.com/benidevo/vega/internal/common/errors"
	"github.com/benidevo/vega/internal/job/interfaces"
	"github.com/benidevo/vega/internal/job/models"
)

// scanner interface abstracts the common Scan method from *sql.Row and *sql.Rows
type scanner interface {
	Scan(dest ...any) error
}

// SQLiteJobRepository is a SQLite implementation of JobRepository
type SQLiteJobRepository struct {
	db                *sql.DB
	companyRepository interfaces.CompanyRepository
	cache             cache.Cache
}

func NewSQLiteJobRepository(db *sql.DB, companyRepository interfaces.CompanyRepository, cache cache.Cache) *SQLiteJobRepository {
	return &SQLiteJobRepository{
		db:                db,
		companyRepository: companyRepository,
		cache:             cache,
	}
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

// GetOrCreate retrieves a job by its SourceURL or creates it if it does not exist.
// Returns the existing or newly created job, a boolean indicating if it was newly created, or an error if the operation fails.
func (r *SQLiteJobRepository) GetOrCreate(ctx context.Context, userID int, jobModel *models.Job) (*models.Job, bool, error) {
	if err := validateJob(jobModel); err != nil {
		return nil, false, err
	}

	if jobModel.SourceURL == "" {
		return nil, false, models.ErrInvalidJobID
	}

	existingJob, err := r.GetBySourceURL(ctx, userID, jobModel.SourceURL)
	if err == nil {
		return existingJob, false, nil // Job already exists, not newly created
	}

	if !errors.Is(err, models.ErrJobNotFound) {
		return nil, false, err
	}

	newJob, err := r.Create(ctx, userID, jobModel)
	if err != nil {
		return nil, false, err
	}
	return newJob, true, nil // Job was newly created
}

func (r *SQLiteJobRepository) GetBySourceURL(ctx context.Context, userID int, sourceURL string) (*models.Job, error) {
	if sourceURL == "" {
		return nil, models.ErrInvalidJobID
	}

	query := `
		SELECT
			j.id, j.title, j.description, j.location, j.job_type,
			j.source_url, j.required_skills,
			j.application_url, j.company_id, j.status, j.match_score,
			j.notes, j.created_at, j.updated_at, j.user_id, j.first_analyzed_at,
			c.name, c.created_at, c.updated_at
		FROM jobs j
		JOIN companies c ON j.company_id = c.id
		WHERE j.source_url = ? AND j.user_id = ?
	`

	row := r.db.QueryRowContext(ctx, query, sourceURL, userID)

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
		return nil, err
	}

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
		return nil, models.WrapError(models.ErrFailedToCreateJob, err)
	}

	query := `
		INSERT INTO jobs (
			title, description, location, job_type, source_url,
			required_skills, application_url,
			company_id, status, notes,
			created_at, updated_at, user_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		jobModel.Title,
		jobModel.Description,
		jobModel.Location,
		int(jobModel.JobType),
		jobModel.SourceURL,
		skillsJSON,
		jobModel.ApplicationURL,
		company.ID,
		int(jobModel.Status),
		jobModel.Notes,
		jobModel.CreatedAt,
		jobModel.UpdatedAt,
		userID,
	)
	if err != nil {
		return nil, models.WrapError(models.ErrFailedToCreateJob, err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, models.WrapError(models.ErrFailedToCreateJob, err)
	}

	jobModel.ID = int(id)
	jobModel.Company = *company

	_ = r.cache.Delete(ctx,
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

	query := `
		SELECT
			j.id, j.title, j.description, j.location, j.job_type,
			j.source_url, j.required_skills,
			j.application_url, j.company_id, j.status, j.match_score,
			j.notes, j.created_at, j.updated_at, j.user_id, j.first_analyzed_at,
			c.name, c.created_at, c.updated_at
		FROM jobs j
		JOIN companies c ON j.company_id = c.id
		WHERE j.id = ? AND j.user_id = ?
	`

	row := r.db.QueryRowContext(ctx, query, id, userID)

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

func (r *SQLiteJobRepository) GetAll(ctx context.Context, userID int, filter models.JobFilter) ([]*models.Job, error) {
	query := `
		SELECT
			j.id, j.title, j.description, j.location, j.job_type,
			j.source_url, j.required_skills,
			j.application_url, j.company_id, j.status, j.match_score,
			j.notes, j.created_at, j.updated_at, j.user_id, j.first_analyzed_at,
			c.name, c.created_at, c.updated_at
		FROM jobs j
		JOIN companies c ON j.company_id = c.id
	`

	var conditions []string
	var args []any

	conditions = append(conditions, "j.user_id = ?")
	args = append(args, userID)

	if filter.CompanyID != nil {
		conditions = append(conditions, "j.company_id = ?")
		args = append(args, *filter.CompanyID)
	}

	if filter.Status != nil {
		conditions = append(conditions, "j.status = ?")
		args = append(args, int(*filter.Status))
	}

	if filter.JobType != nil {
		conditions = append(conditions, "j.job_type = ?")
		args = append(args, int(*filter.JobType))
	}

	if filter.Matched != nil {
		if *filter.Matched {
			conditions = append(conditions, "j.match_score >= 70")
		} else {
			conditions = append(conditions, "(j.match_score IS NULL OR j.match_score < 70)")
		}
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

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

	orderBy := " ORDER BY "
	switch filter.SortBy {
	case "match_score":
		// Sort by match_score, with NULL values last
		if filter.SortOrder == "asc" {
			orderBy += "CASE WHEN j.match_score IS NULL THEN 1 ELSE 0 END, j.match_score ASC"
		} else {
			orderBy += "CASE WHEN j.match_score IS NULL THEN 1 ELSE 0 END, j.match_score DESC"
		}
	case "created_at":
		if filter.SortOrder == "asc" {
			orderBy += "j.created_at ASC"
		} else {
			orderBy += "j.created_at DESC"
		}
	case "updated_at":
		if filter.SortOrder == "asc" {
			orderBy += "j.updated_at ASC"
		} else {
			orderBy += "j.updated_at DESC"
		}
	default:
		orderBy += "j.updated_at DESC"
	}
	query += orderBy

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)

		if filter.Offset > 0 {
			query += " OFFSET ?"
			args = append(args, filter.Offset)
		}
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
		return err
	}

	job.UpdatedAt = time.Now().UTC()

	skillsJSON, err := json.Marshal(job.RequiredSkills)
	if err != nil {
		return models.WrapError(models.ErrFailedToUpdateJob, err)
	}

	query := `
		UPDATE jobs SET
			title = ?, description = ?, location = ?, job_type = ?,
			source_url = ?, required_skills = ?,
			application_url = ?, company_id = ?,
			status = ?, match_score = ?, notes = ?, updated_at = ?
		WHERE id = ? AND user_id = ?
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		job.Title,
		job.Description,
		job.Location,
		int(job.JobType),
		job.SourceURL,
		skillsJSON,
		job.ApplicationURL,
		company.ID,
		int(job.Status),
		job.MatchScore,
		job.Notes,
		job.UpdatedAt,
		job.ID,
		userID,
	)
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

	_ = r.cache.Delete(ctx,
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

	query := `UPDATE jobs SET match_score = ?, updated_at = ? WHERE id = ? AND user_id = ?`

	result, err := r.db.ExecContext(ctx, query, matchScore, time.Now().UTC(), jobID, userID)
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

	_ = r.cache.Delete(ctx,
		fmt.Sprintf("job:u%d:id%d", userID, jobID),
		fmt.Sprintf("stats:u%d:summary", userID),
	)

	return nil
}

func (r *SQLiteJobRepository) Delete(ctx context.Context, userID int, id int) error {
	if id <= 0 {
		return models.ErrInvalidJobID
	}

	query := "DELETE FROM jobs WHERE id = ? AND user_id = ?"

	result, err := r.db.ExecContext(ctx, query, id, userID)
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

	_ = r.cache.Delete(ctx,
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

	query := "UPDATE jobs SET status = ?, updated_at = ? WHERE id = ? AND user_id = ?"

	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx, query, int(status), now, id, userID)
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

	_ = r.cache.Delete(ctx,
		fmt.Sprintf("stats:u%d:summary", userID),
		fmt.Sprintf("stats:u%d:by-status", userID),
		fmt.Sprintf("job:u%d:id%d", userID, id),
	)

	return nil
}

func (r *SQLiteJobRepository) GetCount(ctx context.Context, userID int, filter models.JobFilter) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM jobs j
		JOIN companies c ON j.company_id = c.id
	`

	var conditions []string
	var args []any

	conditions = append(conditions, "j.user_id = ?")
	args = append(args, userID)

	if filter.CompanyID != nil {
		conditions = append(conditions, "j.company_id = ?")
		args = append(args, *filter.CompanyID)
	}

	if filter.Status != nil {
		conditions = append(conditions, "j.status = ?")
		args = append(args, int(*filter.Status))
	}

	if filter.JobType != nil {
		conditions = append(conditions, "j.job_type = ?")
		args = append(args, int(*filter.JobType))
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	var count int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, models.WrapError(models.ErrFailedToGetJobStats, err)
	}

	return count, nil
}

// GetStats returns aggregate statistics about jobs in the database.
// It returns the total number of jobs, jobs with status APPLIED, and jobs with high match scores (>=70).
func (r *SQLiteJobRepository) GetStats(ctx context.Context, userID int) (*models.JobStats, error) {
	query := `
        SELECT
            COUNT(*) AS total_jobs,
            COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS applied,
            COALESCE(SUM(CASE WHEN match_score >= 70 THEN 1 ELSE 0 END), 0) AS high_match
        FROM jobs
        WHERE user_id = ?
    `
	rows, err := r.db.QueryContext(ctx, query, int(models.APPLIED), userID)
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

	query := `
        SELECT
            COUNT(*) AS total_jobs,
            COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS applied,
            COALESCE(SUM(CASE WHEN match_score >= 70 THEN 1 ELSE 0 END), 0) AS high_match
        FROM jobs
        WHERE user_id = ?
    `

	rows, err := r.db.QueryContext(ctx, query, int(models.APPLIED), userID)
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

// GetJobStatsByStatus returns job counts grouped by status for a specific user.
// This is useful for homepage pipeline visualization.
func (r *SQLiteJobRepository) GetJobStatsByStatus(ctx context.Context, userID int) (map[models.JobStatus]int, error) {
	cacheKey := fmt.Sprintf("stats:u%d:by-status", userID)
	var statusCounts map[models.JobStatus]int
	if err := r.cache.Get(ctx, cacheKey, &statusCounts); err == nil {
		return statusCounts, nil
	}

	query := `
        SELECT
            status,
            COUNT(*) as count
        FROM jobs
        WHERE user_id = ?
        GROUP BY status
        ORDER BY status
    `

	rows, err := r.db.QueryContext(ctx, query, userID)
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

	query := `
		INSERT INTO match_results (job_id, match_score, strengths, weaknesses, highlights, feedback, user_id)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		matchResult.JobID,
		matchResult.MatchScore,
		string(strengthsJSON),
		string(weaknessesJSON),
		string(highlightsJSON),
		matchResult.Feedback,
		userID,
	)
	if err != nil {
		return models.WrapError(models.ErrFailedToCreateJob, err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return models.WrapError(models.ErrFailedToCreateJob, err)
	}

	matchResult.ID = int(id)

	_ = r.cache.Delete(ctx,
		fmt.Sprintf("match:u%d:job%d:history", userID, matchResult.JobID),
	)
	_ = r.cache.DeletePattern(ctx, fmt.Sprintf("match:u%d:recent:*", userID))

	return nil
}

func (r *SQLiteJobRepository) GetJobMatchHistory(ctx context.Context, userID int, jobID int) ([]*models.MatchResult, error) {
	cacheKey := fmt.Sprintf("match:u%d:job%d:history", userID, jobID)
	var results []*models.MatchResult
	if err := r.cache.Get(ctx, cacheKey, &results); err == nil {
		return results, nil
	}

	query := `
		SELECT id, job_id, match_score, strengths, weaknesses, highlights, feedback, created_at
		FROM match_results
		WHERE job_id = ? AND user_id = ?
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, jobID, userID)
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

	query := `
		SELECT mr.job_id, j.title, c.name, mr.match_score, mr.strengths,
		       mr.weaknesses, mr.created_at
		FROM match_results mr
		JOIN jobs j ON mr.job_id = j.id
		JOIN companies c ON j.company_id = c.id
		WHERE mr.job_id != ? AND mr.user_id = ?
		ORDER BY
			CASE WHEN c.name = (SELECT c2.name FROM jobs j2 JOIN companies c2 ON j2.company_id = c2.id WHERE j2.id = ?) THEN 0 ELSE 1 END,
			mr.created_at DESC
		LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, query, currentJobID, userID, currentJobID, limit)
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

	query := `
		SELECT mr.id, mr.job_id, mr.match_score, mr.strengths, mr.weaknesses,
		       mr.highlights, mr.feedback, mr.created_at
		FROM match_results mr
		WHERE mr.user_id = ?
		ORDER BY mr.created_at DESC
		LIMIT ?
	`

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
	err := r.db.QueryRowContext(ctx,
		`SELECT job_id FROM match_results WHERE id = ? AND user_id = ?`,
		matchID, userID,
	).Scan(&jobID)
	if err != nil {
		if err == sql.ErrNoRows {
			return models.ErrJobNotFound
		}
		return models.WrapError(models.ErrFailedToDeleteJob, err)
	}

	query := `DELETE FROM match_results WHERE id = ? AND user_id = ?`

	result, err := r.db.ExecContext(ctx, query, matchID, userID)
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

	_ = r.cache.Delete(ctx,
		fmt.Sprintf("match:u%d:job%d:history", userID, jobID),
	)
	_ = r.cache.DeletePattern(ctx, fmt.Sprintf("match:u%d:recent:*", userID))

	return nil
}

func (r *SQLiteJobRepository) MatchResultBelongsToJob(ctx context.Context, userID int, matchID, jobID int) (bool, error) {
	if matchID <= 0 || jobID <= 0 {
		return false, models.ErrInvalidJobID
	}

	query := `SELECT EXISTS(SELECT 1 FROM match_results WHERE id = ? AND job_id = ? AND user_id = ?)`

	var exists bool
	err := r.db.QueryRowContext(ctx, query, matchID, jobID, userID).Scan(&exists)
	if err != nil {
		return false, models.WrapError(models.ErrFailedToGetJob, err)
	}

	return exists, nil
}

func (r *SQLiteJobRepository) GetMonthlyAnalysisCount(ctx context.Context, userID int) (int, error) {
	now := time.Now().UTC()
	firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	query := `
		SELECT COUNT(*)
		FROM jobs
		WHERE user_id = ?
		AND first_analyzed_at IS NOT NULL
		AND first_analyzed_at >= ?
	`

	var count int
	err := r.db.QueryRowContext(ctx, query, userID, firstOfMonth).Scan(&count)
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
