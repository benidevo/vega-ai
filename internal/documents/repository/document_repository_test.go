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
		countRows := sqlmock.NewRows([]string{"count"}).AddRow(2)
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM documents`).
			WithArgs(1, models.DocumentTypeCoverLetter).
			WillReturnRows(countRows)

		now := time.Now()
		docRows := sqlmock.NewRows([]string{
			"id", "job_id", "title", "name", "status", "document_type",
			"preview", "size_bytes", "created_at", "updated_at",
		}).AddRow(1, 1, "Software Engineer", "Tech Corp", 0, "cover_letter",
			"Dear Hiring Manager...", 100, now, now).
			AddRow(2, 2, "Senior Developer", "Another Corp", 1, "cover_letter",
				"I am writing to...", 150, now, now)

		mock.ExpectQuery(`SELECT .+ FROM documents d JOIN jobs j`).
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
