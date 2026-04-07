package db

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/rs/zerolog/log"
)

// MigrateDatabase performs database migrations using the golang-migrate library.
// It opens a dedicated connection for migrations — golang-migrate owns and closes
// that connection, so the app's shared pool is unaffected.
func MigrateDatabase(dbPath, migrationsDir string) error {
	log.Info().
		Str("migrationsDir", migrationsDir).
		Msg("Starting database migration")

	migrationDB, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return fmt.Errorf("failed to open migration connection: %w", err)
	}

	driver, err := sqlite.WithInstance(migrationDB, &sqlite.Config{})
	if err != nil {
		return fmt.Errorf("failed to create database driver instance: %w", err)
	}

	absPath, err := filepath.Abs(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for migrations: %w", err)
	}

	sourceURL := fmt.Sprintf("file://%s", absPath)
	m, err := migrate.NewWithDatabaseInstance(
		sourceURL,
		"sqlite",
		driver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			log.Info().Msg("No migrations to apply, database is up to date")
			return nil
		}
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Info().Msg("Database migration completed successfully")
	return nil
}
