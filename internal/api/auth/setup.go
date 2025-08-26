package auth

import (
	"database/sql"

	"github.com/benidevo/vega/internal/auth/repository"
	"github.com/benidevo/vega/internal/auth/services"
	"github.com/benidevo/vega/internal/config"
)

func Setup(db *sql.DB, cfg *config.Settings) *AuthAPIHandler {
	repo := repository.NewSQLiteUserRepository(db)
	authService := services.NewAuthService(repo, cfg)
	return NewAuthAPIHandler(authService)
}
