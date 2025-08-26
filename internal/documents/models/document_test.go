package models

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDocumentValidation(t *testing.T) {
	tests := []struct {
		name    string
		doc     Document
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid_cover_letter",
			doc: Document{
				UserID:       1,
				JobID:        1,
				DocumentType: DocumentTypeCoverLetter,
				Content:      "Dear Hiring Manager, ...",
				Format:       "html",
			},
			wantErr: false,
		},
		{
			name: "valid_resume",
			doc: Document{
				UserID:       1,
				JobID:        1,
				DocumentType: DocumentTypeResume,
				Content:      "Professional Summary...",
				Format:       "html",
			},
			wantErr: false,
		},
		{
			name: "invalid_user_id",
			doc: Document{
				UserID:       0,
				JobID:        1,
				DocumentType: DocumentTypeCoverLetter,
				Content:      "Content",
			},
			wantErr: true,
			errMsg:  "invalid user ID",
		},
		{
			name: "invalid_job_id",
			doc: Document{
				UserID:       1,
				JobID:        0,
				DocumentType: DocumentTypeCoverLetter,
				Content:      "Content",
			},
			wantErr: true,
			errMsg:  "invalid job ID",
		},
		{
			name: "empty_content",
			doc: Document{
				UserID:       1,
				JobID:        1,
				DocumentType: DocumentTypeCoverLetter,
				Content:      "",
			},
			wantErr: true,
			errMsg:  "document content cannot be empty",
		},
		{
			name: "invalid_document_type",
			doc: Document{
				UserID:       1,
				JobID:        1,
				DocumentType: "invalid",
				Content:      "Content",
			},
			wantErr: true,
			errMsg:  "invalid document type",
		},
		{
			name: "document_too_large",
			doc: Document{
				UserID:       1,
				JobID:        1,
				DocumentType: DocumentTypeCoverLetter,
				Content:      strings.Repeat("a", MaxDocumentSize+1),
			},
			wantErr: true,
			errMsg:  "document exceeds maximum size",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.doc.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateDocumentType(t *testing.T) {
	tests := []struct {
		name    string
		docType DocumentType
		wantErr bool
	}{
		{
			name:    "valid_cover_letter",
			docType: DocumentTypeCoverLetter,
			wantErr: false,
		},
		{
			name:    "valid_resume",
			docType: DocumentTypeResume,
			wantErr: false,
		},
		{
			name:    "invalid_type",
			docType: "invalid",
			wantErr: true,
		},
		{
			name:    "empty_type",
			docType: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDocumentType(tt.docType)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, ErrInvalidDocumentType, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDocumentSummary(t *testing.T) {
	now := time.Now()
	summary := DocumentSummary{
		ID:           1,
		JobID:        100,
		JobTitle:     "Software Engineer",
		CompanyName:  "Tech Corp",
		JobStatus:    "Applied",
		DocumentType: DocumentTypeCoverLetter,
		Preview:      "Dear Hiring Manager, I am writing to express my interest...",
		SizeBytes:    1024,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	assert.Equal(t, 1, summary.ID)
	assert.Equal(t, 100, summary.JobID)
	assert.Equal(t, "Software Engineer", summary.JobTitle)
	assert.Equal(t, "Tech Corp", summary.CompanyName)
	assert.Equal(t, "Applied", summary.JobStatus)
	assert.Equal(t, DocumentTypeCoverLetter, summary.DocumentType)
	assert.Contains(t, summary.Preview, "Dear Hiring Manager")
	assert.Equal(t, 1024, summary.SizeBytes)
	assert.Equal(t, now, summary.CreatedAt)
	assert.Equal(t, now, summary.UpdatedAt)
}

func TestDocumentMetrics(t *testing.T) {
	lastCreated := time.Now()
	metrics := DocumentMetrics{
		TotalDocuments:      10,
		CoverLetterCount:    6,
		ResumeCount:         4,
		TotalSizeBytes:      1024000,
		LastDocumentCreated: &lastCreated,
	}

	assert.Equal(t, 10, metrics.TotalDocuments)
	assert.Equal(t, 6, metrics.CoverLetterCount)
	assert.Equal(t, 4, metrics.ResumeCount)
	assert.Equal(t, 1024000, metrics.TotalSizeBytes)
	assert.NotNil(t, metrics.LastDocumentCreated)
	assert.Equal(t, lastCreated, *metrics.LastDocumentCreated)

	metricsEmpty := DocumentMetrics{
		TotalDocuments: 0,
	}
	assert.Nil(t, metricsEmpty.LastDocumentCreated)
}

func TestErrorConstants(t *testing.T) {
	assert.Equal(t, "document not found", ErrDocumentNotFound.Error())
	assert.Equal(t, "document exceeds maximum size limit", ErrDocumentTooLarge.Error())
	assert.Equal(t, "invalid document type", ErrInvalidDocumentType.Error())
	assert.Equal(t, "failed to save document", ErrDocumentSavesFailed.Error())
}

func TestMaxDocumentSize(t *testing.T) {
	assert.Equal(t, 2*1024*1024, MaxDocumentSize)
}
