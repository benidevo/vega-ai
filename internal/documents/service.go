package documents

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/benidevo/vega/internal/cache"
	"github.com/benidevo/vega/internal/common/logger"
	"github.com/benidevo/vega/internal/documents/models"
	"github.com/benidevo/vega/internal/documents/repository"
)

type DocumentService struct {
	repo    repository.DocumentRepository
	cache   cache.Cache
	log     *logger.PrivacyLogger
	cacheMu sync.RWMutex
}

func NewDocumentService(repo repository.DocumentRepository, cache cache.Cache) *DocumentService {
	return &DocumentService{
		repo:  repo,
		cache: cache,
		log:   logger.GetPrivacyLogger("documents"),
	}
}

func (s *DocumentService) SaveGeneratedDocument(ctx context.Context, userID, jobID int, docType models.DocumentType, content string) (*models.Document, error) {
	userRef := fmt.Sprintf("user_%d", userID)

	s.log.Debug().
		Str("user_ref", userRef).
		Int("job_id", jobID).
		Str("document_type", string(docType)).
		Msg("Saving generated document")

	if err := models.ValidateDocumentType(docType); err != nil {
		s.log.Error().
			Str("user_ref", userRef).
			Str("document_type", string(docType)).
			Err(err).
			Msg("Invalid document type")
		return nil, err
	}

	if len(content) > models.MaxDocumentSize {
		s.log.Error().
			Str("user_ref", userRef).
			Int("size", len(content)).
			Int("max_size", models.MaxDocumentSize).
			Msg("Document exceeds maximum size")
		return nil, models.ErrDocumentTooLarge
	}

	doc := &models.Document{
		UserID:       userID,
		JobID:        jobID,
		DocumentType: docType,
		Content:      content,
		Format:       "html",
		SizeBytes:    len(content),
	}

	if err := doc.Validate(); err != nil {
		s.log.Error().
			Str("user_ref", userRef).
			Err(err).
			Msg("Document validation failed")
		return nil, err
	}

	err := s.repo.UpsertDocument(ctx, doc)
	if err != nil {
		s.log.Error().
			Str("user_ref", userRef).
			Int("job_id", jobID).
			Str("document_type", string(docType)).
			Err(err).
			Msg("Failed to save document")
		return nil, models.ErrDocumentSavesFailed
	}

	s.invalidateDocumentCaches(userID, jobID, docType)

	s.log.Info().
		Str("user_ref", userRef).
		Int("job_id", jobID).
		Int("document_id", doc.ID).
		Str("document_type", string(docType)).
		Int("size_bytes", doc.SizeBytes).
		Msg("Document saved successfully")

	return doc, nil
}

func (s *DocumentService) GetDocument(ctx context.Context, docID, userID int) (*models.Document, error) {
	userRef := fmt.Sprintf("user_%d", userID)

	s.log.Debug().
		Str("user_ref", userRef).
		Int("document_id", docID).
		Msg("Getting document")

	doc, err := s.repo.GetDocument(ctx, docID, userID)
	if err != nil {
		if err == models.ErrDocumentNotFound {
			s.log.Warn().
				Str("user_ref", userRef).
				Int("document_id", docID).
				Msg("Document not found")
		} else {
			s.log.Error().
				Str("user_ref", userRef).
				Int("document_id", docID).
				Err(err).
				Msg("Failed to get document")
		}
		return nil, err
	}

	return doc, nil
}

func (s *DocumentService) GetDocumentByJobAndType(ctx context.Context, userID, jobID int, docType models.DocumentType) (*models.Document, error) {
	userRef := fmt.Sprintf("user_%d", userID)

	s.log.Debug().
		Str("user_ref", userRef).
		Int("job_id", jobID).
		Str("document_type", string(docType)).
		Msg("Getting document by job and type")

	doc, err := s.repo.GetDocumentByJobAndType(ctx, userID, jobID, docType)
	if err != nil {
		if err == models.ErrDocumentNotFound {
			s.log.Debug().
				Str("user_ref", userRef).
				Int("job_id", jobID).
				Str("document_type", string(docType)).
				Msg("Document not found for job")
		} else {
			s.log.Error().
				Str("user_ref", userRef).
				Int("job_id", jobID).
				Str("document_type", string(docType)).
				Err(err).
				Msg("Failed to get document by job and type")
		}
		return nil, err
	}

	return doc, nil
}

func (s *DocumentService) GetDocumentsByType(ctx context.Context, userID int, docType models.DocumentType, page, pageSize int) ([]*models.DocumentSummary, int, error) {
	userRef := fmt.Sprintf("user_%d", userID)

	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}

	if page == 1 && s.cache != nil {
		cacheKey := fmt.Sprintf("user:%d:docs:%s", userID, docType)
		var cached struct {
			Docs  []*models.DocumentSummary
			Total int
		}
		if err := s.cache.Get(ctx, cacheKey, &cached); err == nil {
			s.log.Debug().
				Str("user_ref", userRef).
				Str("document_type", string(docType)).
				Msg("Returning cached documents")
			return cached.Docs, cached.Total, nil
		}
	}

	s.log.Debug().
		Str("user_ref", userRef).
		Str("document_type", string(docType)).
		Int("page", page).
		Int("page_size", pageSize).
		Msg("Getting documents by type")

	docs, total, err := s.repo.GetDocumentsByType(ctx, userID, docType, pageSize, offset)
	if err != nil {
		s.log.Error().
			Str("user_ref", userRef).
			Str("document_type", string(docType)).
			Err(err).
			Msg("Failed to get documents by type")
		return nil, 0, err
	}

	if page == 1 && s.cache != nil {
		cacheData := struct {
			Docs  []*models.DocumentSummary
			Total int
		}{
			Docs:  docs,
			Total: total,
		}
		_ = s.cache.Set(ctx, fmt.Sprintf("user:%d:docs:%s", userID, docType), cacheData, 5*time.Minute)
	}

	return docs, total, nil
}

func (s *DocumentService) GetAllDocuments(ctx context.Context, userID int, page, pageSize int) ([]*models.DocumentSummary, int, error) {
	userRef := fmt.Sprintf("user_%d", userID)

	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}

	s.log.Debug().
		Str("user_ref", userRef).
		Int("page", page).
		Int("page_size", pageSize).
		Msg("Getting all documents")

	docs, total, err := s.repo.GetAllDocuments(ctx, userID, pageSize, offset)
	if err != nil {
		s.log.Error().
			Str("user_ref", userRef).
			Err(err).
			Msg("Failed to get all documents")
		return nil, 0, err
	}

	return docs, total, nil
}

func (s *DocumentService) DeleteDocument(ctx context.Context, docID, userID int) error {
	userRef := fmt.Sprintf("user_%d", userID)

	s.log.Info().
		Str("user_ref", userRef).
		Int("document_id", docID).
		Msg("Deleting document")

	err := s.repo.DeleteDocument(ctx, docID, userID)
	if err != nil {
		if err == models.ErrDocumentNotFound {
			s.log.Warn().
				Str("user_ref", userRef).
				Int("document_id", docID).
				Msg("Document not found for deletion")
		} else {
			s.log.Error().
				Str("user_ref", userRef).
				Int("document_id", docID).
				Err(err).
				Msg("Failed to delete document")
		}
		return err
	}

	if s.cache != nil {
		_ = s.cache.DeletePattern(ctx, fmt.Sprintf("user:%d:docs:*", userID))

		_ = s.cache.Delete(ctx, fmt.Sprintf("user:%d:metrics", userID))

		_ = s.cache.Delete(ctx, fmt.Sprintf("doc:%d", docID))
	}

	s.log.Info().
		Str("user_ref", userRef).
		Int("document_id", docID).
		Msg("Document deleted successfully")

	return nil
}

func (s *DocumentService) GetDocumentMetrics(ctx context.Context, userID int) (*models.DocumentMetrics, error) {
	userRef := fmt.Sprintf("user_%d", userID)

	if s.cache != nil {
		cacheKey := fmt.Sprintf("user:%d:metrics", userID)
		var metrics models.DocumentMetrics
		if err := s.cache.Get(ctx, cacheKey, &metrics); err == nil {
			return &metrics, nil
		}
	}

	s.log.Debug().
		Str("user_ref", userRef).
		Msg("Getting document metrics")

	metrics, err := s.repo.GetDocumentMetrics(ctx, userID)
	if err != nil {
		s.log.Error().
			Str("user_ref", userRef).
			Err(err).
			Msg("Failed to get document metrics")
		return nil, err
	}

	if s.cache != nil {
		_ = s.cache.Set(ctx, fmt.Sprintf("user:%d:metrics", userID), *metrics, 15*time.Minute)
	}

	return metrics, nil
}

func (s *DocumentService) GetDocumentsByJob(ctx context.Context, userID, jobID int) ([]*models.Document, error) {
	userRef := fmt.Sprintf("user_%d", userID)

	if s.cache != nil {
		cacheKey := fmt.Sprintf("job:%d:docs", jobID)
		var docs []*models.Document

		// Use a write lock for the entire operation to prevent race conditions
		s.cacheMu.Lock()
		cacheErr := s.cache.Get(ctx, cacheKey, &docs)
		if cacheErr == nil && len(docs) > 0 {
			if docs[0].UserID == userID {
				s.cacheMu.Unlock()
				return docs, nil
			}
			_ = s.cache.Delete(ctx, cacheKey)
			s.log.Warn().
				Str("user_ref", userRef).
				Int("job_id", jobID).
				Msg("Cleared invalid cache entry for job documents")
		}
		s.cacheMu.Unlock()
	}

	s.log.Debug().
		Str("user_ref", userRef).
		Int("job_id", jobID).
		Msg("Getting documents by job")

	docs, err := s.repo.GetDocumentsByJob(ctx, userID, jobID)
	if err != nil {
		s.log.Error().
			Str("user_ref", userRef).
			Int("job_id", jobID).
			Err(err).
			Msg("Failed to get documents by job")
		return nil, err
	}

	if s.cache != nil && len(docs) > 0 {
		s.cacheMu.Lock()
		_ = s.cache.Set(ctx, fmt.Sprintf("job:%d:docs", jobID), docs, 10*time.Minute)
		s.cacheMu.Unlock()
	}

	return docs, nil
}

func (s *DocumentService) CheckDocumentExists(ctx context.Context, userID, jobID int, docType models.DocumentType) (bool, error) {
	doc, err := s.repo.GetDocumentByJobAndType(ctx, userID, jobID, docType)
	if err == models.ErrDocumentNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return doc != nil, nil
}

func (s *DocumentService) invalidateDocumentCaches(userID int, jobID int, docType models.DocumentType) {
	if s.cache == nil {
		return
	}

	ctx := context.Background()

	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()

	patterns := []string{
		fmt.Sprintf("user:%d:docs:*", userID),
	}
	for _, pattern := range patterns {
		if err := s.cache.DeletePattern(ctx, pattern); err != nil {
			s.log.Warn().
				Str("pattern", pattern).
				Err(err).
				Msg("Failed to invalidate cache pattern")
		}
	}

	keys := []string{
		fmt.Sprintf("user:%d:metrics", userID),
		fmt.Sprintf("job:%d:docs", jobID),
	}
	for _, key := range keys {
		if err := s.cache.Delete(ctx, key); err != nil {
			s.log.Warn().
				Str("key", key).
				Err(err).
				Msg("Failed to invalidate cache key")
		}
	}
}
