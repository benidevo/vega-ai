package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"

	"github.com/benidevo/vega/internal/cache"
	"github.com/benidevo/vega/internal/common/logger"
	"github.com/benidevo/vega/internal/db/querybuilder"
	"github.com/benidevo/vega/internal/documents/models"
)

// documentColumns defines columns for document queries.
var documentColumns = []string{
	"id", "user_id", "job_id", "document_type", "content", "format", "size_bytes", "created_at", "updated_at",
}

type SQLiteDocumentRepository struct {
	db    *sql.DB
	cache cache.Cache
	log   *logger.PrivacyLogger
}

func NewSQLiteDocumentRepository(db *sql.DB, cache cache.Cache) *SQLiteDocumentRepository {
	return &SQLiteDocumentRepository{
		db:    db,
		cache: cache,
		log:   logger.GetPrivacyLogger("documents_repository"),
	}
}

func (r *SQLiteDocumentRepository) UpsertDocument(ctx context.Context, doc *models.Document) error {
	if doc == nil {
		return fmt.Errorf("document cannot be nil")
	}

	doc.SizeBytes = len(doc.Content)
	if doc.SizeBytes > models.MaxDocumentSize {
		return fmt.Errorf("document size %d exceeds maximum size %d", doc.SizeBytes, models.MaxDocumentSize)
	}

	// Add timeout to prevent indefinite blocking on SQLite locks
	ctx, cancel := context.WithTimeout(ctx, 7*time.Second)
	defer cancel()

	// Keep UPSERT as raw SQL - Squirrel doesn't handle ON CONFLICT well
	query := `
		INSERT INTO documents (user_id, job_id, document_type, content, format, size_bytes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(user_id, job_id, document_type)
		DO UPDATE SET
			content = excluded.content,
			format = excluded.format,
			size_bytes = excluded.size_bytes,
			updated_at = CURRENT_TIMESTAMP
		RETURNING id, created_at, updated_at`

	err := r.db.QueryRowContext(ctx, query,
		doc.UserID,
		doc.JobID,
		doc.DocumentType,
		doc.Content,
		doc.Format,
		doc.SizeBytes,
	).Scan(&doc.ID, &doc.CreatedAt, &doc.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to upsert document: %w", err)
	}

	r.invalidateDocumentCache(doc.UserID, doc.JobID, doc.DocumentType)

	return nil
}

func (r *SQLiteDocumentRepository) GetDocument(ctx context.Context, docID, userID int) (*models.Document, error) {
	cacheKey := fmt.Sprintf("doc:%d", docID)
	var doc models.Document

	if r.cache != nil {
		if err := r.cache.Get(ctx, cacheKey, &doc); err == nil && doc.UserID == userID {
			return &doc, nil
		}
	}

	query, args, err := querybuilder.Select(documentColumns...).
		From("documents").
		Where(sq.Eq{"id": docID, "user_id": userID}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	err = r.db.QueryRowContext(ctx, query, args...).Scan(
		&doc.ID,
		&doc.UserID,
		&doc.JobID,
		&doc.DocumentType,
		&doc.Content,
		&doc.Format,
		&doc.SizeBytes,
		&doc.CreatedAt,
		&doc.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, models.ErrDocumentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	if r.cache != nil {
		_ = r.cache.Set(ctx, cacheKey, doc, 10*time.Minute)
	}

	return &doc, nil
}

func (r *SQLiteDocumentRepository) GetDocumentByJobAndType(ctx context.Context, userID, jobID int, docType models.DocumentType) (*models.Document, error) {
	var doc models.Document

	query, args, err := querybuilder.Select(documentColumns...).
		From("documents").
		Where(sq.Eq{"user_id": userID, "job_id": jobID, "document_type": docType}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	err = r.db.QueryRowContext(ctx, query, args...).Scan(
		&doc.ID,
		&doc.UserID,
		&doc.JobID,
		&doc.DocumentType,
		&doc.Content,
		&doc.Format,
		&doc.SizeBytes,
		&doc.CreatedAt,
		&doc.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, models.ErrDocumentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get document by job and type: %w", err)
	}

	return &doc, nil
}

func (r *SQLiteDocumentRepository) GetDocumentsByType(ctx context.Context, userID int, docType models.DocumentType, limit, offset int) ([]*models.DocumentSummary, int, error) {
	// Get count
	countQuery, countArgs, err := querybuilder.Select("COUNT(*)").
		From("documents d").
		Where(sq.Eq{"d.user_id": userID}).
		Where(sq.Eq{"d.document_type": docType}).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to build count query: %w", err)
	}

	var totalCount int
	err = r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get document count: %w", err)
	}

	query, args, err := querybuilder.Select(
		"d.id", "d.job_id", "j.title", "c.name", "j.status", "d.document_type",
		"SUBSTR(d.content, 1, 200) as preview", "d.size_bytes", "d.created_at", "d.updated_at",
	).
		From("documents d").
		Join("jobs j ON d.job_id = j.id").
		Join("companies c ON j.company_id = c.id").
		Where(sq.Eq{"d.user_id": userID}).
		Where(sq.Eq{"d.document_type": docType}).
		OrderBy("d.updated_at DESC").
		Limit(uint64(limit)).
		Offset(uint64(offset)).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to build query: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query documents: %w", err)
	}
	defer rows.Close()

	var summaries []*models.DocumentSummary
	for rows.Next() {
		var summary models.DocumentSummary
		var jobStatus int

		err := rows.Scan(
			&summary.ID,
			&summary.JobID,
			&summary.JobTitle,
			&summary.CompanyName,
			&jobStatus,
			&summary.DocumentType,
			&summary.Preview,
			&summary.SizeBytes,
			&summary.CreatedAt,
			&summary.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan document summary: %w", err)
		}

		summary.JobStatus = r.jobStatusToString(jobStatus)
		summaries = append(summaries, &summary)
	}

	return summaries, totalCount, nil
}

func (r *SQLiteDocumentRepository) GetAllDocuments(ctx context.Context, userID int, limit, offset int) ([]*models.DocumentSummary, int, error) {
	// Get count
	countQuery, countArgs, err := querybuilder.Select("COUNT(*)").
		From("documents d").
		Where(sq.Eq{"d.user_id": userID}).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to build count query: %w", err)
	}

	var totalCount int
	err = r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get document count: %w", err)
	}

	query, args, err := querybuilder.Select(
		"d.id", "d.job_id", "j.title", "c.name", "j.status", "d.document_type",
		"SUBSTR(d.content, 1, 200) as preview", "d.size_bytes", "d.created_at", "d.updated_at",
	).
		From("documents d").
		Join("jobs j ON d.job_id = j.id").
		Join("companies c ON j.company_id = c.id").
		Where(sq.Eq{"d.user_id": userID}).
		OrderBy("d.updated_at DESC").
		Limit(uint64(limit)).
		Offset(uint64(offset)).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to build query: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query documents: %w", err)
	}
	defer rows.Close()

	var summaries []*models.DocumentSummary
	for rows.Next() {
		var summary models.DocumentSummary
		var jobStatus int

		err := rows.Scan(
			&summary.ID,
			&summary.JobID,
			&summary.JobTitle,
			&summary.CompanyName,
			&jobStatus,
			&summary.DocumentType,
			&summary.Preview,
			&summary.SizeBytes,
			&summary.CreatedAt,
			&summary.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan document summary: %w", err)
		}

		summary.JobStatus = r.jobStatusToString(jobStatus)
		summaries = append(summaries, &summary)
	}

	return summaries, totalCount, nil
}

func (r *SQLiteDocumentRepository) DeleteDocument(ctx context.Context, docID, userID int) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query, args, err := querybuilder.Delete("documents").
		Where(sq.Eq{"id": docID, "user_id": userID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to build query: %w", err)
	}

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return models.ErrDocumentNotFound
	}

	if r.cache != nil {
		cacheKey := fmt.Sprintf("doc:%d", docID)
		_ = r.cache.Delete(ctx, cacheKey)
	}

	return nil
}

func (r *SQLiteDocumentRepository) GetDocumentMetrics(ctx context.Context, userID int) (*models.DocumentMetrics, error) {
	query, args, err := querybuilder.Select(
		"COUNT(*) as total_documents",
		"COUNT(CASE WHEN document_type = 'cover_letter' THEN 1 END) as cover_letter_count",
		"COUNT(CASE WHEN document_type = 'resume' THEN 1 END) as resume_count",
		"COALESCE(SUM(size_bytes), 0) as total_size_bytes",
		"MAX(created_at) as last_document_created",
	).
		From("documents").
		Where(sq.Eq{"user_id": userID}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	var metrics models.DocumentMetrics
	var lastCreated sql.NullString

	err = r.db.QueryRowContext(ctx, query, args...).Scan(
		&metrics.TotalDocuments,
		&metrics.CoverLetterCount,
		&metrics.ResumeCount,
		&metrics.TotalSizeBytes,
		&lastCreated,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get document metrics: %w", err)
	}

	if lastCreated.Valid && lastCreated.String != "" {
		parsedTime, err := time.Parse("2006-01-02 15:04:05", lastCreated.String)
		if err != nil {
			parsedTime, err = time.Parse("2006-01-02T15:04:05Z07:00", lastCreated.String)
			if err == nil {
				metrics.LastDocumentCreated = &parsedTime
			}
		} else {
			metrics.LastDocumentCreated = &parsedTime
		}
	}

	return &metrics, nil
}

func (r *SQLiteDocumentRepository) GetDocumentsByJob(ctx context.Context, userID, jobID int) ([]*models.Document, error) {
	query, args, err := querybuilder.Select(documentColumns...).
		From("documents").
		Where(sq.Eq{"user_id": userID, "job_id": jobID}).
		OrderBy("document_type").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query documents by job: %w", err)
	}
	defer rows.Close()

	var documents []*models.Document
	for rows.Next() {
		var doc models.Document
		err := rows.Scan(
			&doc.ID,
			&doc.UserID,
			&doc.JobID,
			&doc.DocumentType,
			&doc.Content,
			&doc.Format,
			&doc.SizeBytes,
			&doc.CreatedAt,
			&doc.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan document: %w", err)
		}
		documents = append(documents, &doc)
	}

	return documents, nil
}

func (r *SQLiteDocumentRepository) invalidateDocumentCache(userID int, jobID int, docType models.DocumentType) {
	if r.cache == nil {
		return
	}

	patterns := []string{
		fmt.Sprintf("user:%d:docs:*", userID),
		fmt.Sprintf("job:%d:docs", jobID),
	}

	for _, pattern := range patterns {
		if err := r.cache.DeletePattern(context.Background(), pattern); err != nil {
			// Log but don't fail - cache invalidation is best-effort
			// Stale data will expire naturally via TTL
			r.log.Warn().
				Str("pattern", pattern).
				Err(err).
				Msg("Failed to invalidate cache pattern")
		}
	}
}

func (r *SQLiteDocumentRepository) jobStatusToString(status int) string {
	statusMap := map[int]string{
		0: "New",
		1: "Applied",
		2: "Interview",
		3: "Offer",
		4: "Rejected",
		5: "Accepted",
	}

	if s, ok := statusMap[status]; ok {
		return s
	}
	return "Unknown"
}
