package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestMigrateDatabase(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "migrations-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	dbFile := filepath.Join(tempDir, "test.db")

	migrationsDir := filepath.Join(tempDir, "migrations")
	err = os.MkdirAll(migrationsDir, 0755)
	require.NoError(t, err)

	upMigration := `CREATE TABLE test_users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL
	);`
	downMigration := `DROP TABLE test_users;`

	err = os.WriteFile(filepath.Join(migrationsDir, "000001_create_test_users.up.sql"), []byte(upMigration), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(migrationsDir, "000001_create_test_users.down.sql"), []byte(downMigration), 0644)
	require.NoError(t, err)

	err = MigrateDatabase(dbFile, migrationsDir)
	require.NoError(t, err)

	database, err := sql.Open("sqlite", dbFile)
	require.NoError(t, err)
	defer database.Close()

	var tableName string
	err = database.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='test_users'").Scan(&tableName)
	require.NoError(t, err)
	assert.Equal(t, "test_users", tableName)

	err = MigrateDatabase(dbFile, migrationsDir)
	assert.NoError(t, err)

	err = MigrateDatabase(dbFile, "/nonexistent/migrations/dir")
	assert.Error(t, err)
}
