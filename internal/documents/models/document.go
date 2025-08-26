package models

import (
	"errors"
	"time"
)

type DocumentType string

const (
	DocumentTypeCoverLetter DocumentType = "cover_letter"
	DocumentTypeResume      DocumentType = "resume"
)

const MaxDocumentSize = 2 * 1024 * 1024

type Document struct {
	ID           int          `json:"id"`
	UserID       int          `json:"user_id"`
	JobID        int          `json:"job_id"`
	DocumentType DocumentType `json:"document_type"`
	Content      string       `json:"content"`
	Format       string       `json:"format"`
	SizeBytes    int          `json:"size_bytes"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

type DocumentSummary struct {
	ID           int          `json:"id"`
	JobID        int          `json:"job_id"`
	JobTitle     string       `json:"job_title"`
	CompanyName  string       `json:"company_name"`
	JobStatus    string       `json:"job_status"`
	DocumentType DocumentType `json:"document_type"`
	Preview      string       `json:"preview"`
	SizeBytes    int          `json:"size_bytes"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

type DocumentMetrics struct {
	TotalDocuments      int        `json:"total_documents"`
	CoverLetterCount    int        `json:"cover_letter_count"`
	ResumeCount         int        `json:"resume_count"`
	TotalSizeBytes      int        `json:"total_size_bytes"`
	LastDocumentCreated *time.Time `json:"last_document_created,omitempty"`
}

var (
	ErrDocumentNotFound    = errors.New("document not found")
	ErrDocumentTooLarge    = errors.New("document exceeds maximum size limit")
	ErrInvalidDocumentType = errors.New("invalid document type")
	ErrDocumentSavesFailed = errors.New("failed to save document")
	ErrUnauthorized        = errors.New("unauthorized access to document")
)

func ValidateDocumentType(docType DocumentType) error {
	switch docType {
	case DocumentTypeCoverLetter, DocumentTypeResume:
		return nil
	default:
		return ErrInvalidDocumentType
	}
}

func (d *Document) Validate() error {
	if d.UserID <= 0 {
		return errors.New("invalid user ID")
	}
	if d.JobID <= 0 {
		return errors.New("invalid job ID")
	}
	if err := ValidateDocumentType(d.DocumentType); err != nil {
		return err
	}
	if d.Content == "" {
		return errors.New("document content cannot be empty")
	}
	if len(d.Content) > MaxDocumentSize {
		return ErrDocumentTooLarge
	}
	return nil
}

func (d *Document) GetPreview() string {
	content := d.Content
	if len(content) > 200 {
		content = content[:200] + "..."
	}
	return content
}
