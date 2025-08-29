package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/benidevo/vega/internal/cache"
	"github.com/benidevo/vega/internal/db"
	"github.com/benidevo/vega/internal/job/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConcurrentJobCreation(t *testing.T) {
	testDB := setupTestDB(t)
	defer testDB.Close()

	mockCache := &cache.NoOpCache{}
	companyRepo := NewSQLiteCompanyRepository(testDB, mockCache)
	jobRepo := NewSQLiteJobRepository(testDB, companyRepo, mockCache)

	ctx := context.Background()
	userID := 1
	sourceURL := "https://example.com/job/123"

	testCompany, err := companyRepo.GetOrCreate(ctx, "Test Company")
	require.NoError(t, err)

	numGoroutines := 5
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	type result struct {
		job        *models.Job
		wasCreated bool
		err        error
	}
	results := make(chan result, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			defer wg.Done()

			job := models.NewJob(
				fmt.Sprintf("Software Engineer %d", index),
				"Job description",
				*testCompany,
				models.WithSourceURL(sourceURL),
				models.WithLocation("San Francisco"),
			)

			var createdJob *models.Job
			var wasCreated bool
			var err error

			for retries := 0; retries < 3; retries++ {
				createdJob, wasCreated, err = jobRepo.GetOrCreate(ctx, userID, job)
				if err == nil || !strings.Contains(err.Error(), "database is locked") {
					break
				}
				time.Sleep(time.Millisecond * 10 * time.Duration(retries+1))
			}

			results <- result{
				job:        createdJob,
				wasCreated: wasCreated,
				err:        err,
			}
		}(i)
	}

	wg.Wait()
	close(results)

	var createdCount int
	var successCount int
	var firstJobID int

	for res := range results {
		if res.err != nil {
			t.Logf("Error (expected with SQLite concurrency): %v", res.err)
			continue
		}

		successCount++
		if res.wasCreated {
			createdCount++
		}

		if res.job != nil {
			if firstJobID == 0 {
				firstJobID = res.job.ID
			}
			assert.Equal(t, firstJobID, res.job.ID, "All results should reference the same job")
			assert.Equal(t, sourceURL, res.job.SourceURL, "Source URL should match")
		}
	}

	assert.Greater(t, successCount, 0, "At least some operations should succeed")
	assert.LessOrEqual(t, createdCount, 1, "At most one job should be created")

	jobs, err := jobRepo.GetAll(ctx, userID, models.JobFilter{})
	require.NoError(t, err)
	if successCount > 0 {
		assert.Len(t, jobs, 1, "Only one job should exist in the database")
		assert.Equal(t, sourceURL, jobs[0].SourceURL)
	}
}

func TestConcurrentJobUpdates(t *testing.T) {
	testDB := setupTestDB(t)
	defer testDB.Close()

	mockCache := &cache.NoOpCache{}
	companyRepo := NewSQLiteCompanyRepository(testDB, mockCache)
	jobRepo := NewSQLiteJobRepository(testDB, companyRepo, mockCache)

	ctx := context.Background()
	userID := 1

	testCompany := models.Company{Name: "Test Company"}
	initialJob := models.NewJob(
		"Software Engineer",
		"Job description",
		testCompany,
		models.WithSourceURL("https://example.com/job/456"),
	)

	createdJob, err := jobRepo.Create(ctx, userID, initialJob)
	require.NoError(t, err)

	numUpdates := 5
	var wg sync.WaitGroup
	wg.Add(numUpdates)

	statuses := []models.JobStatus{
		models.INTERESTED,
		models.APPLIED,
		models.INTERVIEWING,
		models.OFFER_RECEIVED,
		models.REJECTED,
	}

	for i := 0; i < numUpdates; i++ {
		go func(index int) {
			defer wg.Done()

			var err error
			for retries := 0; retries < 3; retries++ {
				err = jobRepo.UpdateStatus(ctx, userID, createdJob.ID, statuses[index])
				if err == nil || !strings.Contains(err.Error(), "database is locked") {
					break
				}
				time.Sleep(time.Millisecond * 10 * time.Duration(retries+1))
			}

			if err != nil && strings.Contains(err.Error(), "database is locked") {
				t.Logf("Expected SQLite concurrency error: %v", err)
			} else {
				assert.NoError(t, err, "Non-concurrency errors should not occur")
			}
		}(i)
	}

	wg.Wait()

	updatedJob, err := jobRepo.GetByID(ctx, userID, createdJob.ID)
	require.NoError(t, err)

	validStatus := false
	for _, status := range statuses {
		if updatedJob.Status == status {
			validStatus = true
			break
		}
	}
	assert.True(t, validStatus, "Job should have one of the attempted statuses")
}

func TestTransactionRollback(t *testing.T) {
	testDB := setupTestDB(t)
	defer testDB.Close()

	mockCache := &cache.NoOpCache{}
	companyRepo := NewSQLiteCompanyRepository(testDB, mockCache)
	jobRepo := NewSQLiteJobRepository(testDB, companyRepo, mockCache)

	ctx := context.Background()
	userID := 1

	testCompany, err := companyRepo.GetOrCreate(ctx, "Test Company")
	require.NoError(t, err)

	tx, err := jobRepo.BeginTx(ctx, nil)
	require.NoError(t, err)

	job := models.NewJob(
		"Software Engineer",
		"Job description",
		*testCompany,
		models.WithSourceURL("https://example.com/job/789"),
	)
	job.CompanyID = testCompany.ID

	createdJob, err := jobRepo.CreateWithTx(ctx, tx, userID, job)
	require.NoError(t, err)
	assert.NotZero(t, createdJob.ID)

	err = jobRepo.RollbackTx(tx)
	require.NoError(t, err)

	_, err = jobRepo.GetByID(ctx, userID, createdJob.ID)
	assert.ErrorIs(t, err, models.ErrJobNotFound, "Job should not exist after rollback")
}

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dbPath := fmt.Sprintf("/tmp/vega_test_%d.db", time.Now().UnixNano())
	testDB := db.SqlDBFromPath(dbPath)

	queries := []string{
		`CREATE TABLE IF NOT EXISTS companies (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS jobs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			description TEXT NOT NULL,
			location TEXT,
			job_type INTEGER DEFAULT 0,
			source_url TEXT,
			required_skills TEXT,
			application_url TEXT,
			company_id INTEGER NOT NULL,
			status INTEGER DEFAULT 0,
			match_score INTEGER,
			notes TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			first_analyzed_at TIMESTAMP,
			content_hash TEXT,
			FOREIGN KEY (company_id) REFERENCES companies(id)
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_jobs_user_source_url 
		 ON jobs(user_id, source_url) 
		 WHERE source_url IS NOT NULL AND source_url != ''`,
	}

	for _, query := range queries {
		_, err := testDB.Exec(query)
		require.NoError(t, err, "Failed to create test table")
	}

	return testDB
}
