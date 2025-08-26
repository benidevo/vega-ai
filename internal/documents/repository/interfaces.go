package repository

import (
	"context"

	"github.com/benidevo/vega/internal/documents/models"
)

type DocumentRepository interface {
	UpsertDocument(ctx context.Context, doc *models.Document) error
	GetDocument(ctx context.Context, docID, userID int) (*models.Document, error)
	GetDocumentByJobAndType(ctx context.Context, userID, jobID int, docType models.DocumentType) (*models.Document, error)
	GetDocumentsByType(ctx context.Context, userID int, docType models.DocumentType, limit, offset int) ([]*models.DocumentSummary, int, error)
	GetAllDocuments(ctx context.Context, userID int, limit, offset int) ([]*models.DocumentSummary, int, error)
	DeleteDocument(ctx context.Context, docID, userID int) error
	GetDocumentMetrics(ctx context.Context, userID int) (*models.DocumentMetrics, error)
	GetDocumentsByJob(ctx context.Context, userID, jobID int) ([]*models.Document, error)
}
