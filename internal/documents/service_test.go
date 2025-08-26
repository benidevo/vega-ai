package documents

import (
	"context"
	"testing"

	"github.com/benidevo/vega/internal/documents/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockDocumentRepository struct {
	mock.Mock
}

func (m *mockDocumentRepository) UpsertDocument(ctx context.Context, doc *models.Document) error {
	args := m.Called(ctx, doc)
	return args.Error(0)
}

func (m *mockDocumentRepository) GetDocument(ctx context.Context, docID, userID int) (*models.Document, error) {
	args := m.Called(ctx, docID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Document), args.Error(1)
}

func (m *mockDocumentRepository) GetDocumentByJobAndType(ctx context.Context, userID, jobID int, docType models.DocumentType) (*models.Document, error) {
	args := m.Called(ctx, userID, jobID, docType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Document), args.Error(1)
}

func (m *mockDocumentRepository) GetDocumentsByType(ctx context.Context, userID int, docType models.DocumentType, limit, offset int) ([]*models.DocumentSummary, int, error) {
	args := m.Called(ctx, userID, docType, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*models.DocumentSummary), args.Int(1), args.Error(2)
}

func (m *mockDocumentRepository) GetAllDocuments(ctx context.Context, userID int, limit, offset int) ([]*models.DocumentSummary, int, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*models.DocumentSummary), args.Int(1), args.Error(2)
}

func (m *mockDocumentRepository) DeleteDocument(ctx context.Context, docID, userID int) error {
	args := m.Called(ctx, docID, userID)
	return args.Error(0)
}

func (m *mockDocumentRepository) GetDocumentMetrics(ctx context.Context, userID int) (*models.DocumentMetrics, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DocumentMetrics), args.Error(1)
}

func (m *mockDocumentRepository) GetDocumentsByJob(ctx context.Context, userID, jobID int) ([]*models.Document, error) {
	args := m.Called(ctx, userID, jobID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Document), args.Error(1)
}

func TestSaveGeneratedDocument(t *testing.T) {
	tests := []struct {
		name      string
		userID    int
		jobID     int
		docType   models.DocumentType
		content   string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "Valid cover letter",
			userID:    1,
			jobID:     1,
			docType:   models.DocumentTypeCoverLetter,
			content:   "<html><body>Test cover letter content</body></html>",
			wantError: false,
		},
		{
			name:      "Valid resume",
			userID:    1,
			jobID:     2,
			docType:   models.DocumentTypeResume,
			content:   "<html><body>Test resume content</body></html>",
			wantError: false,
		},
		{
			name:      "Invalid document type",
			userID:    1,
			jobID:     1,
			docType:   "invalid",
			content:   "content",
			wantError: true,
			errorMsg:  "invalid document type",
		},
		{
			name:      "Document too large",
			userID:    1,
			jobID:     1,
			docType:   models.DocumentTypeCoverLetter,
			content:   string(make([]byte, models.MaxDocumentSize+1)),
			wantError: true,
			errorMsg:  "document exceeds maximum size limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(mockDocumentRepository)
			service := NewDocumentService(mockRepo, nil)

			if !tt.wantError {
				mockRepo.On("UpsertDocument", mock.Anything, mock.AnythingOfType("*models.Document")).
					Return(nil).
					Run(func(args mock.Arguments) {
						doc := args.Get(1).(*models.Document)
						doc.ID = 1 // Simulate DB setting ID
					})
			}

			doc, err := service.SaveGeneratedDocument(
				context.Background(),
				tt.userID,
				tt.jobID,
				tt.docType,
				tt.content,
			)

			if tt.wantError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				assert.Nil(t, doc)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, doc)
				assert.Equal(t, tt.userID, doc.UserID)
				assert.Equal(t, tt.jobID, doc.JobID)
				assert.Equal(t, tt.docType, doc.DocumentType)
				assert.Equal(t, tt.content, doc.Content)
				assert.Equal(t, len(tt.content), doc.SizeBytes)
				mockRepo.AssertExpectations(t)
			}
		})
	}
}

// TestGetDocument tests the GetDocument method
func TestGetDocument(t *testing.T) {
	mockRepo := new(mockDocumentRepository)
	service := NewDocumentService(mockRepo, nil)

	expectedDoc := &models.Document{
		ID:           1,
		UserID:       1,
		JobID:        1,
		DocumentType: models.DocumentTypeCoverLetter,
		Content:      "<html>Test content</html>",
		Format:       "html",
		SizeBytes:    25,
	}

	mockRepo.On("GetDocument", mock.Anything, 1, 1).Return(expectedDoc, nil)
	mockRepo.On("GetDocument", mock.Anything, 2, 1).Return(nil, models.ErrDocumentNotFound)

	// Test successful retrieval
	doc, err := service.GetDocument(context.Background(), 1, 1)
	assert.NoError(t, err)
	assert.Equal(t, expectedDoc, doc)

	// Test document not found
	doc, err = service.GetDocument(context.Background(), 2, 1)
	assert.Error(t, err)
	assert.Equal(t, models.ErrDocumentNotFound, err)
	assert.Nil(t, doc)

	mockRepo.AssertExpectations(t)
}

// TestDeleteDocument tests the DeleteDocument method
func TestDeleteDocument(t *testing.T) {
	mockRepo := new(mockDocumentRepository)
	service := NewDocumentService(mockRepo, nil)

	mockRepo.On("DeleteDocument", mock.Anything, 1, 1).Return(nil)
	mockRepo.On("DeleteDocument", mock.Anything, 2, 1).Return(models.ErrDocumentNotFound)

	// Test successful deletion
	err := service.DeleteDocument(context.Background(), 1, 1)
	assert.NoError(t, err)

	// Test document not found
	err = service.DeleteDocument(context.Background(), 2, 1)
	assert.Error(t, err)
	assert.Equal(t, models.ErrDocumentNotFound, err)

	mockRepo.AssertExpectations(t)
}

// TestCheckDocumentExists tests the CheckDocumentExists method
func TestCheckDocumentExists(t *testing.T) {
	mockRepo := new(mockDocumentRepository)
	service := NewDocumentService(mockRepo, nil)

	existingDoc := &models.Document{
		ID:           1,
		UserID:       1,
		JobID:        1,
		DocumentType: models.DocumentTypeCoverLetter,
	}

	mockRepo.On("GetDocumentByJobAndType", mock.Anything, 1, 1, models.DocumentTypeCoverLetter).
		Return(existingDoc, nil)
	mockRepo.On("GetDocumentByJobAndType", mock.Anything, 1, 2, models.DocumentTypeCoverLetter).
		Return(nil, models.ErrDocumentNotFound)

	// Test document exists
	exists, err := service.CheckDocumentExists(context.Background(), 1, 1, models.DocumentTypeCoverLetter)
	assert.NoError(t, err)
	assert.True(t, exists)

	// Test document doesn't exist
	exists, err = service.CheckDocumentExists(context.Background(), 1, 2, models.DocumentTypeCoverLetter)
	assert.NoError(t, err)
	assert.False(t, exists)

	mockRepo.AssertExpectations(t)
}
