package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
	_ "modernc.org/sqlite"
)

// DBConfig holds database connection pool configuration settings.
type DBConfig struct {
	MaxOpenConns    int           // Maximum number of open connections to the database
	MaxIdleConns    int           // Maximum number of idle connections in the pool
	ConnMaxLifetime time.Duration // Maximum lifetime of a connection
	ConnMaxIdleTime time.Duration // Maximum time a connection can be idle
}

// DefaultDBConfig returns a DBConfig with recommended default values for SQLite.
func DefaultDBConfig() DBConfig {
	return DBConfig{
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	}
}

// SqlDBFromPath creates a new SQL database connection from a path with default configuration.
func SqlDBFromPath(dbPath string) *sql.DB {
	return SqlDBFromPathWithConfig(dbPath, DefaultDBConfig())
}

// SqlDBFromPathWithConfig creates a new SQL database connection with custom configuration.
func SqlDBFromPathWithConfig(dbPath string, config DBConfig) *sql.DB {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatal().Err(err).Str("path", dir).Msg("Failed to create database directory")
		}
	}

	connectionString := dbPath + "?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL&cache=shared"

	db, err := sql.Open("sqlite", connectionString)
	if err != nil {
		log.Fatal().Err(err).Str("path", dbPath).Msg("Failed to open database connection")
	}

	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	if err := db.Ping(); err != nil {
		log.Fatal().Err(err).Str("path", dbPath).Msg("Failed to ping database")
	}

	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		log.Warn().Err(err).Msg("Failed to enable foreign key constraints")
	}

	log.Info().
		Int("max_open_conns", config.MaxOpenConns).
		Int("max_idle_conns", config.MaxIdleConns).
		Dur("conn_max_lifetime", config.ConnMaxLifetime).
		Dur("conn_max_idle_time", config.ConnMaxIdleTime).
		Msg("Database connection pool configured")

	return db
}
