package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/benidevo/vega/internal/documents/models"
)

func setupMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db, mock
}

func TestUpsertDocument(t *testing.T) {
	ctx := context.Background()
	db, mock := setupMockDB(t)
	repo := NewSQLiteDocumentRepository(db, nil)

	doc := &models.Document{
		UserID:       1,
		JobID:        1,
		DocumentType: models.DocumentTypeCoverLetter,
		Content:      "<html>Test cover letter</html>",
		Format:       "html",
	}

	t.Run("insert new document", func(t *testing.T) {
		now := time.Now()
		rows := sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).
			AddRow(1, now, now)

		mock.ExpectQuery(`INSERT INTO documents`).
			WithArgs(doc.UserID, doc.JobID, doc.DocumentType, doc.Content, doc.Format, len(doc.Content)).
			WillReturnRows(rows)

		err := repo.UpsertDocument(ctx, doc)
		assert.NoError(t, err)
		assert.Equal(t, 1, doc.ID)
		assert.NotZero(t, doc.CreatedAt)
		assert.NotZero(t, doc.UpdatedAt)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("update existing document", func(t *testing.T) {
		doc.Content = "<html>Updated cover letter</html>"
		doc.ID = 0 // Reset ID to simulate fresh upsert

		now := time.Now()
		rows := sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).
			AddRow(1, now.Add(-time.Hour), now)

		mock.ExpectQuery(`INSERT INTO documents`).
			WithArgs(doc.UserID, doc.JobID, doc.DocumentType, doc.Content, doc.Format, len(doc.Content)).
			WillReturnRows(rows)

		err := repo.UpsertDocument(ctx, doc)
		assert.NoError(t, err)
		assert.Equal(t, 1, doc.ID)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestGetDocument(t *testing.T) {
	ctx := context.Background()
	db, mock := setupMockDB(t)
	repo := NewSQLiteDocumentRepository(db, nil)

	t.Run("successful retrieval", func(t *testing.T) {
		now := time.Now()
		rows := sqlmock.NewRows([]string{
			"id", "user_id", "job_id", "document_type", "content",
			"format", "size_bytes", "created_at", "updated_at",
		}).AddRow(1, 1, 1, "cover_letter", "<html>Test</html>", "html", 17, now, now)

		mock.ExpectQuery(`SELECT (.+) FROM documents WHERE id = \? AND user_id = \?`).
			WithArgs(1, 1).
			WillReturnRows(rows)

		doc, err := repo.GetDocument(ctx, 1, 1)
		assert.NoError(t, err)
		assert.NotNil(t, doc)
		assert.Equal(t, 1, doc.ID)
		assert.Equal(t, "<html>Test</html>", doc.Content)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("document not found", func(t *testing.T) {
		mock.ExpectQuery(`SELECT (.+) FROM documents WHERE id = \? AND user_id = \?`).
			WithArgs(999, 1).
			WillReturnError(sql.ErrNoRows)

		doc, err := repo.GetDocument(ctx, 999, 1)
		assert.Error(t, err)
		assert.Equal(t, models.ErrDocumentNotFound, err)
		assert.Nil(t, doc)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestGetDocumentsByType(t *testing.T) {
	ctx := context.Background()
	db, mock := setupMockDB(t)
	repo := NewSQLiteDocumentRepository(db, nil)

	t.Run("get documents with pagination", func(t *testing.T) {
		now := time.Now()
		docRows := sqlmock.NewRows([]string{
			"id", "job_id", "title", "name", "status", "document_type",
			"preview", "size_bytes", "created_at", "updated_at", "total_count",
		}).AddRow(1, 1, "Software Engineer", "Tech Corp", 0, "cover_letter",
			"Dear Hiring Manager...", 100, now, now, 2).
			AddRow(2, 2, "Senior Developer", "Another Corp", 1, "cover_letter",
				"I am writing to...", 150, now, now, 2)

		mock.ExpectQuery(`SELECT .+ COUNT\(\*\) OVER\(\) .+ FROM documents d`).
			WithArgs(1, models.DocumentTypeCoverLetter, 10, 0).
			WillReturnRows(docRows)

		summaries, total, err := repo.GetDocumentsByType(ctx, 1, models.DocumentTypeCoverLetter, 10, 0)
		assert.NoError(t, err)
		assert.Equal(t, 2, total)
		assert.Len(t, summaries, 2)
		assert.Equal(t, "Software Engineer", summaries[0].JobTitle)
		assert.Equal(t, "Tech Corp", summaries[0].CompanyName)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestDeleteDocument(t *testing.T) {
	ctx := context.Background()
	db, mock := setupMockDB(t)
	repo := NewSQLiteDocumentRepository(db, nil)

	t.Run("successful deletion", func(t *testing.T) {
		mock.ExpectExec(`DELETE FROM documents WHERE id = \? AND user_id = \?`).
			WithArgs(1, 1).
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := repo.DeleteDocument(ctx, 1, 1)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("document not found", func(t *testing.T) {
		mock.ExpectExec(`DELETE FROM documents WHERE id = \? AND user_id = \?`).
			WithArgs(999, 1).
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := repo.DeleteDocument(ctx, 999, 1)
		assert.Error(t, err)
		assert.Equal(t, models.ErrDocumentNotFound, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestGetDocumentMetrics(t *testing.T) {
	ctx := context.Background()
	db, mock := setupMockDB(t)
	repo := NewSQLiteDocumentRepository(db, nil)

	t.Run("metrics with documents", func(t *testing.T) {
		lastCreated := time.Now()
		rows := sqlmock.NewRows([]string{
			"total_documents", "cover_letter_count", "resume_count",
			"total_size_bytes", "last_document_created",
		}).AddRow(5, 3, 2, 10000, lastCreated)

		mock.ExpectQuery(`SELECT .+ FROM documents WHERE user_id = \?`).
			WithArgs(1).
			WillReturnRows(rows)

		metrics, err := repo.GetDocumentMetrics(ctx, 1)
		assert.NoError(t, err)
		assert.NotNil(t, metrics)
		assert.Equal(t, 5, metrics.TotalDocuments)
		assert.Equal(t, 3, metrics.CoverLetterCount)
		assert.Equal(t, 2, metrics.ResumeCount)
		assert.Equal(t, 10000, metrics.TotalSizeBytes)
		assert.NotNil(t, metrics.LastDocumentCreated)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("metrics with no documents", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"total_documents", "cover_letter_count", "resume_count",
			"total_size_bytes", "last_document_created",
		}).AddRow(0, 0, 0, 0, nil)

		mock.ExpectQuery(`SELECT .+ FROM documents WHERE user_id = \?`).
			WithArgs(2).
			WillReturnRows(rows)

		metrics, err := repo.GetDocumentMetrics(ctx, 2)
		assert.NoError(t, err)
		assert.NotNil(t, metrics)
		assert.Equal(t, 0, metrics.TotalDocuments)
		assert.Nil(t, metrics.LastDocumentCreated)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestGetDocumentByJobAndType(t *testing.T) {
	ctx := context.Background()
	db, mock := setupMockDB(t)
	repo := NewSQLiteDocumentRepository(db, nil)

	t.Run("successful retrieval", func(t *testing.T) {
		now := time.Now()
		rows := sqlmock.NewRows([]string{
			"id", "user_id", "job_id", "document_type", "content",
			"format", "size_bytes", "created_at", "updated_at",
		}).AddRow(1, 1, 5, "cover_letter", "<html>Cover letter content</html>", "html", 35, now, now)

		mock.ExpectQuery(`SELECT (.+) FROM documents WHERE document_type = \? AND job_id = \? AND user_id = \?`).
			WithArgs(models.DocumentTypeCoverLetter, 5, 1).
			WillReturnRows(rows)

		doc, err := repo.GetDocumentByJobAndType(ctx, 1, 5, models.DocumentTypeCoverLetter)
		assert.NoError(t, err)
		assert.NotNil(t, doc)
		assert.Equal(t, 1, doc.ID)
		assert.Equal(t, 5, doc.JobID)
		assert.Equal(t, models.DocumentTypeCoverLetter, doc.DocumentType)
		assert.Equal(t, "<html>Cover letter content</html>", doc.Content)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("document not found", func(t *testing.T) {
		mock.ExpectQuery(`SELECT (.+) FROM documents WHERE document_type = \? AND job_id = \? AND user_id = \?`).
			WithArgs(models.DocumentTypeCoverLetter, 999, 1).
			WillReturnError(sql.ErrNoRows)

		doc, err := repo.GetDocumentByJobAndType(ctx, 1, 999, models.DocumentTypeCoverLetter)
		assert.Error(t, err)
		assert.Equal(t, models.ErrDocumentNotFound, err)
		assert.Nil(t, doc)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database error", func(t *testing.T) {
		mock.ExpectQuery(`SELECT (.+) FROM documents WHERE document_type = \? AND job_id = \? AND user_id = \?`).
			WithArgs(models.DocumentTypeResume, 5, 1).
			WillReturnError(sql.ErrConnDone)

		doc, err := repo.GetDocumentByJobAndType(ctx, 1, 5, models.DocumentTypeResume)
		assert.Error(t, err)
		assert.Nil(t, doc)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestGetAllDocuments(t *testing.T) {
	ctx := context.Background()
	db, mock := setupMockDB(t)
	repo := NewSQLiteDocumentRepository(db, nil)

	t.Run("get all documents with pagination", func(t *testing.T) {
		now := time.Now()
		rows := sqlmock.NewRows([]string{
			"id", "job_id", "title", "name", "status", "document_type",
			"preview", "size_bytes", "created_at", "updated_at", "total_count",
		}).AddRow(1, 1, "Software Engineer", "Tech Corp", 0, "cover_letter",
			"Dear Hiring Manager...", 100, now, now, 3).
			AddRow(2, 2, "Senior Developer", "Another Corp", 1, "resume",
				"Professional Summary...", 150, now, now, 3).
			AddRow(3, 3, "Backend Engineer", "Third Corp", 2, "cover_letter",
				"I am writing to...", 120, now, now, 3)

		mock.ExpectQuery(`SELECT .+ COUNT\(\*\) OVER\(\) .+ FROM documents d`).
			WithArgs(1, 10, 0).
			WillReturnRows(rows)

		summaries, total, err := repo.GetAllDocuments(ctx, 1, 10, 0)
		assert.NoError(t, err)
		assert.Equal(t, 3, total)
		assert.Len(t, summaries, 3)
		assert.Equal(t, "Software Engineer", summaries[0].JobTitle)
		assert.Equal(t, "Tech Corp", summaries[0].CompanyName)
		assert.Equal(t, "New", summaries[0].JobStatus)
		assert.Equal(t, "Applied", summaries[1].JobStatus)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("empty result", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"id", "job_id", "title", "name", "status", "document_type",
			"preview", "size_bytes", "created_at", "updated_at", "total_count",
		})

		mock.ExpectQuery(`SELECT .+ COUNT\(\*\) OVER\(\) .+ FROM documents d`).
			WithArgs(2, 10, 0).
			WillReturnRows(rows)

		summaries, total, err := repo.GetAllDocuments(ctx, 2, 10, 0)
		assert.NoError(t, err)
		assert.Equal(t, 0, total)
		assert.Empty(t, summaries)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database error", func(t *testing.T) {
		mock.ExpectQuery(`SELECT .+ COUNT\(\*\) OVER\(\) .+ FROM documents d`).
			WithArgs(1, 10, 0).
			WillReturnError(sql.ErrConnDone)

		summaries, total, err := repo.GetAllDocuments(ctx, 1, 10, 0)
		assert.Error(t, err)
		assert.Equal(t, 0, total)
		assert.Nil(t, summaries)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestGetDocumentsByJob(t *testing.T) {
	ctx := context.Background()
	db, mock := setupMockDB(t)
	repo := NewSQLiteDocumentRepository(db, nil)

	t.Run("successful retrieval of multiple documents", func(t *testing.T) {
		now := time.Now()
		rows := sqlmock.NewRows([]string{
			"id", "user_id", "job_id", "document_type", "content",
			"format", "size_bytes", "created_at", "updated_at",
		}).AddRow(1, 1, 5, "cover_letter", "<html>Cover letter</html>", "html", 25, now, now).
			AddRow(2, 1, 5, "resume", "<html>Resume content</html>", "html", 22, now, now)

		// Squirrel generates WHERE conditions in alphabetical order: job_id, user_id
		mock.ExpectQuery(`SELECT (.+) FROM documents WHERE job_id = \? AND user_id = \? ORDER BY document_type`).
			WithArgs(5, 1).
			WillReturnRows(rows)

		docs, err := repo.GetDocumentsByJob(ctx, 1, 5)
		assert.NoError(t, err)
		assert.Len(t, docs, 2)
		assert.Equal(t, models.DocumentTypeCoverLetter, docs[0].DocumentType)
		assert.Equal(t, models.DocumentTypeResume, docs[1].DocumentType)
		assert.Equal(t, 5, docs[0].JobID)
		assert.Equal(t, 5, docs[1].JobID)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("no documents for job", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"id", "user_id", "job_id", "document_type", "content",
			"format", "size_bytes", "created_at", "updated_at",
		})

		mock.ExpectQuery(`SELECT (.+) FROM documents WHERE job_id = \? AND user_id = \? ORDER BY document_type`).
			WithArgs(999, 1).
			WillReturnRows(rows)

		docs, err := repo.GetDocumentsByJob(ctx, 1, 999)
		assert.NoError(t, err)
		assert.Empty(t, docs)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database error", func(t *testing.T) {
		mock.ExpectQuery(`SELECT (.+) FROM documents WHERE job_id = \? AND user_id = \? ORDER BY document_type`).
			WithArgs(5, 1).
			WillReturnError(sql.ErrConnDone)

		docs, err := repo.GetDocumentsByJob(ctx, 1, 5)
		assert.Error(t, err)
		assert.Nil(t, docs)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
