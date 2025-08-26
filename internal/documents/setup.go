package documents

import (
	"database/sql"

	"github.com/benidevo/vega/internal/cache"
	"github.com/benidevo/vega/internal/common/render"
	"github.com/benidevo/vega/internal/config"
	"github.com/benidevo/vega/internal/documents/repository"
)

func Setup(db *sql.DB, cfg *config.Settings, cache cache.Cache, renderer *render.HTMLRenderer) *DocumentHandler {
	repo := repository.NewSQLiteDocumentRepository(db, cache)
	service := NewDocumentService(repo, cache)
	handler := NewDocumentHandler(service, cfg, renderer)

	return handler
}

func SetupService(db *sql.DB, cache cache.Cache) *DocumentService {
	repo := repository.NewSQLiteDocumentRepository(db, cache)
	return NewDocumentService(repo, cache)
}
