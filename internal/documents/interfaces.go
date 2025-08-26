package documents

import (
	"context"

	"github.com/benidevo/vega/internal/documents/models"
)

type Service interface {
	SaveGeneratedDocument(ctx context.Context, userID, jobID int, docType models.DocumentType, content string) (*models.Document, error)
	GetDocument(ctx context.Context, docID, userID int) (*models.Document, error)
	GetDocumentByJobAndType(ctx context.Context, userID, jobID int, docType models.DocumentType) (*models.Document, error)
	GetDocumentsByType(ctx context.Context, userID int, docType models.DocumentType, page, pageSize int) ([]*models.DocumentSummary, int, error)
	GetAllDocuments(ctx context.Context, userID int, page, pageSize int) ([]*models.DocumentSummary, int, error)
	DeleteDocument(ctx context.Context, docID, userID int) error
	GetDocumentMetrics(ctx context.Context, userID int) (*models.DocumentMetrics, error)
	GetDocumentsByJob(ctx context.Context, userID, jobID int) ([]*models.Document, error)
	CheckDocumentExists(ctx context.Context, userID, jobID int, docType models.DocumentType) (bool, error)
}
