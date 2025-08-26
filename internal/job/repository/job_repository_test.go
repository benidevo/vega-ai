package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/benidevo/vega/internal/cache"
	"github.com/benidevo/vega/internal/job/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testUserID = 1

func setupJobRepositoryTest(t *testing.T) (*SQLiteJobRepository, sqlmock.Sqlmock, *MinimalMockCompanyRepository) {
	db, mock := setupMockDB(t)
	mockCompanyRepo := NewMinimalMockCompanyRepository()
	repo := NewSQLiteJobRepository(db, mockCompanyRepo, cache.NewNoOpCache())
	return repo, mock, mockCompanyRepo
}

// MinimalMockCompanyRepository is a simplified mock for testing job repository
type MinimalMockCompanyRepository struct {
	companies map[string]*models.Company
	nextID    int
}

func NewMinimalMockCompanyRepository() *MinimalMockCompanyRepository {
	return &MinimalMockCompanyRepository{
		companies: make(map[string]*models.Company),
		nextID:    1,
	}
}

func (r *MinimalMockCompanyRepository) GetOrCreate(ctx context.Context, name string) (*models.Company, error) {
	if name == "" {
		return nil, models.ErrCompanyNameRequired
	}

	if company, ok := r.companies[name]; ok {
		return company, nil
	}

	now := time.Now()
	company := &models.Company{
		ID:        r.nextID,
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}
	r.nextID++
	r.companies[name] = company
	return company, nil
}

func (r *MinimalMockCompanyRepository) GetByID(ctx context.Context, id int) (*models.Company, error) {
	for _, company := range r.companies {
		if company.ID == id {
			return company, nil
		}
	}
	return nil, models.ErrCompanyNotFound
}

func (r *MinimalMockCompanyRepository) GetByName(ctx context.Context, name string) (*models.Company, error) {
	if company, ok := r.companies[name]; ok {
		return company, nil
	}
	return nil, models.ErrCompanyNotFound
}

func (r *MinimalMockCompanyRepository) GetAll(ctx context.Context) ([]*models.Company, error) {
	companies := make([]*models.Company, 0, len(r.companies))
	for _, company := range r.companies {
		companies = append(companies, company)
	}
	return companies, nil
}

func (r *MinimalMockCompanyRepository) Delete(ctx context.Context, id int) error {
	for name, company := range r.companies {
		if company.ID == id {
			delete(r.companies, name)
			return nil
		}
	}
	return models.ErrCompanyNotFound
}

func (r *MinimalMockCompanyRepository) Update(ctx context.Context, company *models.Company) error {
	if company == nil || company.ID == 0 {
		return errors.New("invalid company")
	}

	for name, c := range r.companies {
		if c.ID == company.ID {
			delete(r.companies, name)
			company.UpdatedAt = time.Now()
			r.companies[company.Name] = company
			return nil
		}
	}
	return models.ErrCompanyNotFound
}

func TestSQLiteJobRepository_Create(t *testing.T) {
	tests := []struct {
		name         string
		job          *models.Job
		setupMock    func(sqlmock.Sqlmock, *models.Job)
		setupCompany func(*MinimalMockCompanyRepository)
		wantErr      error
		validateJob  func(*testing.T, *models.Job)
	}{
		{
			name: "successful creation",
			job: &models.Job{
				Title:          "Software Engineer",
				Description:    "Build awesome software",
				Location:       "Remote",
				JobType:        models.FULL_TIME,
				RequiredSkills: []string{"Go", "SQL"},
				Company:        models.Company{Name: "Acme Corp"},
				Status:         models.INTERESTED,
			},
			setupMock: func(mock sqlmock.Sqlmock, j *models.Job) {
				skillsJSON, _ := json.Marshal(j.RequiredSkills)
				mock.ExpectExec("INSERT INTO jobs").
					WithArgs(
						j.Title, j.Description, j.Location, int(j.JobType),
						j.SourceURL, skillsJSON, j.ApplicationURL, 1,
						int(j.Status), j.Notes,
						sqlmock.AnyArg(), sqlmock.AnyArg(), testUserID,
					).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			validateJob: func(t *testing.T, j *models.Job) {
				assert.Equal(t, 1, j.ID)
				assert.Equal(t, "Acme Corp", j.Company.Name)
				assert.NotZero(t, j.CreatedAt)
				assert.NotZero(t, j.UpdatedAt)
			},
		},
		{
			name: "validation error - missing title",
			job: &models.Job{
				Description: "Build awesome software",
				Company:     models.Company{Name: "Acme Corp"},
			},
			wantErr: models.ErrJobTitleRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock, mockCompanyRepo := setupJobRepositoryTest(t)
			defer mock.ExpectClose()

			if tt.setupCompany != nil {
				tt.setupCompany(mockCompanyRepo)
			}

			if tt.setupMock != nil {
				tt.setupMock(mock, tt.job)
			}

			createdJob, err := repo.Create(context.Background(), testUserID, tt.job)

			if tt.wantErr != nil {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErr))
				assert.Nil(t, createdJob)
			} else {
				require.NoError(t, err)
				require.NotNil(t, createdJob)
				if tt.validateJob != nil {
					tt.validateJob(t, createdJob)
				}
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestSQLiteJobRepository_GetByID(t *testing.T) {
	repo, mock, _ := setupJobRepositoryTest(t)
	defer mock.ExpectClose()

	t.Run("existing job", func(t *testing.T) {
		jobID := 1
		now := time.Now()

		rows := sqlmock.NewRows([]string{
			"j.id", "j.title", "j.description", "j.location", "j.job_type",
			"j.source_url", "j.required_skills",
			"j.application_url", "j.company_id", "j.status", "j.match_score",
			"j.notes", "j.created_at", "j.updated_at", "j.user_id", "j.first_analyzed_at",
			"c.name", "c.created_at", "c.updated_at",
		}).AddRow(
			jobID, "Software Engineer", "Build awesome software", "Remote", int(models.FULL_TIME),
			"https://example.com", `["Go","SQL"]`,
			"https://apply.example.com", 2, int(models.INTERESTED), 85,
			"Great company", now.Add(-24*time.Hour), now, testUserID, nil,
			"Acme Corp", now, now,
		)

		mock.ExpectQuery("SELECT.*FROM jobs.*WHERE j.id = \\? AND j.user_id = \\?").
			WithArgs(jobID, testUserID).
			WillReturnRows(rows)

		job, err := repo.GetByID(context.Background(), testUserID, jobID)

		require.NoError(t, err)
		require.NotNil(t, job)
		assert.Equal(t, jobID, job.ID)
		assert.Equal(t, "Software Engineer", job.Title)
		assert.Equal(t, "Acme Corp", job.Company.Name)
	})

	t.Run("non-existent job", func(t *testing.T) {
		mock.ExpectQuery("SELECT.*FROM jobs.*WHERE j.id = \\? AND j.user_id = \\?").
			WithArgs(999, testUserID).
			WillReturnError(sql.ErrNoRows)

		job, err := repo.GetByID(context.Background(), testUserID, 999)

		assert.Error(t, err)
		assert.True(t, errors.Is(err, models.ErrJobNotFound))
		assert.Nil(t, job)
	})
}

func TestSQLiteJobRepository_UpdateStatus(t *testing.T) {
	repo, mock, _ := setupJobRepositoryTest(t)
	defer mock.ExpectClose()

	tests := []struct {
		name         string
		jobID        int
		status       models.JobStatus
		rowsAffected int64
		wantErr      error
	}{
		{
			name:         "successful update",
			jobID:        1,
			status:       models.APPLIED,
			rowsAffected: 1,
		},
		{
			name:         "job not found",
			jobID:        999,
			status:       models.APPLIED,
			rowsAffected: 0,
			wantErr:      models.ErrJobNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock.ExpectExec("UPDATE jobs SET status = \\?, updated_at = \\? WHERE id = \\? AND user_id = \\?").
				WithArgs(int(tt.status), sqlmock.AnyArg(), tt.jobID, testUserID).
				WillReturnResult(sqlmock.NewResult(0, tt.rowsAffected))

			err := repo.UpdateStatus(context.Background(), testUserID, tt.jobID, tt.status)

			if tt.wantErr != nil {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErr))
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSQLiteJobRepository_Delete(t *testing.T) {
	repo, mock, _ := setupJobRepositoryTest(t)
	defer mock.ExpectClose()

	mock.ExpectExec("DELETE FROM jobs WHERE id = \\? AND user_id = \\?").
		WithArgs(1, testUserID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.Delete(context.Background(), testUserID, 1)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLiteJobRepository_GetAll(t *testing.T) {
	repo, mock, _ := setupJobRepositoryTest(t)
	defer mock.ExpectClose()

	t.Run("filter by company", func(t *testing.T) {
		companyID := 1
		now := time.Now()

		rows := sqlmock.NewRows([]string{
			"j.id", "j.title", "j.description", "j.location", "j.job_type",
			"j.source_url", "j.required_skills",
			"j.application_url", "j.company_id", "j.status", "j.match_score",
			"j.notes", "j.created_at", "j.updated_at", "j.user_id", "j.first_analyzed_at",
			"c.name", "c.created_at", "c.updated_at",
		}).AddRow(
			1, "Software Engineer", "Build awesome software", "Remote", int(models.FULL_TIME),
			"https://example.com", `["Go","SQL"]`,
			"https://apply.example.com", companyID, int(models.INTERESTED), 92,
			"Great company", now.Add(-24*time.Hour), now, testUserID, nil,
			"Acme Corp", now, now,
		)

		mock.ExpectQuery("SELECT.*FROM jobs.*WHERE.*user_id.*company_id.*ORDER BY").
			WithArgs(testUserID, companyID).
			WillReturnRows(rows)

		filter := models.JobFilter{CompanyID: &companyID}
		jobs, err := repo.GetAll(context.Background(), testUserID, filter)

		require.NoError(t, err)
		require.Len(t, jobs, 1)
		assert.Equal(t, companyID, jobs[0].Company.ID)
	})

	t.Run("sort by match_score desc", func(t *testing.T) {
		now := time.Now()

		rows := sqlmock.NewRows([]string{
			"j.id", "j.title", "j.description", "j.location", "j.job_type",
			"j.source_url", "j.required_skills",
			"j.application_url", "j.company_id", "j.status", "j.match_score",
			"j.notes", "j.created_at", "j.updated_at", "j.user_id", "j.first_analyzed_at",
			"c.name", "c.created_at", "c.updated_at",
		}).AddRow(
			1, "Senior Engineer", "Senior role", "Remote", int(models.FULL_TIME),
			"https://example.com", `["Go"]`,
			"https://apply.example.com", 1, int(models.INTERESTED), 95,
			"High match", now.Add(-48*time.Hour), now, testUserID, nil,
			"Tech Corp", now, now,
		).AddRow(
			2, "Junior Engineer", "Junior role", "Remote", int(models.FULL_TIME),
			"https://example.com", `["Python"]`,
			"https://apply.example.com", 1, int(models.INTERESTED), 75,
			"Good match", now.Add(-24*time.Hour), now, testUserID, nil,
			"Tech Corp", now, now,
		)

		// Expect query with CASE statement for NULL handling
		mock.ExpectQuery("SELECT.*FROM jobs.*ORDER BY CASE WHEN j.match_score IS NULL THEN 1 ELSE 0 END, j.match_score DESC").
			WithArgs(testUserID).
			WillReturnRows(rows)

		filter := models.JobFilter{
			SortBy:    "match_score",
			SortOrder: "desc",
		}
		jobs, err := repo.GetAll(context.Background(), testUserID, filter)

		require.NoError(t, err)
		require.Len(t, jobs, 2)
		assert.Equal(t, 95, *jobs[0].MatchScore)
		assert.Equal(t, 75, *jobs[1].MatchScore)
	})

	t.Run("sort by match_score asc", func(t *testing.T) {
		now := time.Now()

		rows := sqlmock.NewRows([]string{
			"j.id", "j.title", "j.description", "j.location", "j.job_type",
			"j.source_url", "j.required_skills",
			"j.application_url", "j.company_id", "j.status", "j.match_score",
			"j.notes", "j.created_at", "j.updated_at", "j.user_id", "j.first_analyzed_at",
			"c.name", "c.created_at", "c.updated_at",
		}).AddRow(
			1, "Low Match Job", "Junior role", "Remote", int(models.FULL_TIME),
			"https://example.com", `["Python"]`,
			"https://apply.example.com", 1, int(models.INTERESTED), 60,
			"Lower match", now.Add(-24*time.Hour), now, testUserID, nil,
			"Tech Corp", now, now,
		).AddRow(
			2, "High Match Job", "Senior role", "Remote", int(models.FULL_TIME),
			"https://example.com", `["Go"]`,
			"https://apply.example.com", 1, int(models.INTERESTED), 90,
			"Higher match", now.Add(-48*time.Hour), now, testUserID, nil,
			"Tech Corp", now, now,
		)

		mock.ExpectQuery("SELECT.*FROM jobs.*ORDER BY CASE WHEN j.match_score IS NULL THEN 1 ELSE 0 END, j.match_score ASC").
			WithArgs(testUserID).
			WillReturnRows(rows)

		filter := models.JobFilter{
			SortBy:    "match_score",
			SortOrder: "asc",
		}
		jobs, err := repo.GetAll(context.Background(), testUserID, filter)

		require.NoError(t, err)
		require.Len(t, jobs, 2)
		assert.Equal(t, 60, *jobs[0].MatchScore, "First job should have lowest match score")
		assert.Equal(t, 90, *jobs[1].MatchScore, "Second job should have higher match score")
	})

	t.Run("sort by created_at asc", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"j.id", "j.title", "j.description", "j.location", "j.job_type",
			"j.source_url", "j.required_skills",
			"j.application_url", "j.company_id", "j.status", "j.match_score",
			"j.notes", "j.created_at", "j.updated_at", "j.user_id", "j.first_analyzed_at",
			"c.name", "c.created_at", "c.updated_at",
		})

		mock.ExpectQuery("SELECT.*FROM jobs.*ORDER BY j.created_at ASC").
			WithArgs(testUserID).
			WillReturnRows(rows)

		filter := models.JobFilter{
			SortBy:    "created_at",
			SortOrder: "asc",
		}
		_, err := repo.GetAll(context.Background(), testUserID, filter)

		require.NoError(t, err)
	})

	t.Run("default sort (updated_at desc)", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"j.id", "j.title", "j.description", "j.location", "j.job_type",
			"j.source_url", "j.required_skills",
			"j.application_url", "j.company_id", "j.status", "j.match_score",
			"j.notes", "j.created_at", "j.updated_at", "j.user_id", "j.first_analyzed_at",
			"c.name", "c.created_at", "c.updated_at",
		})

		mock.ExpectQuery("SELECT.*FROM jobs.*ORDER BY j.updated_at DESC").
			WithArgs(testUserID).
			WillReturnRows(rows)

		filter := models.JobFilter{} // No sort specified
		_, err := repo.GetAll(context.Background(), testUserID, filter)

		require.NoError(t, err)
	})

	t.Run("sort with NULL match_scores", func(t *testing.T) {
		now := time.Now()

		rows := sqlmock.NewRows([]string{
			"j.id", "j.title", "j.description", "j.location", "j.job_type",
			"j.source_url", "j.required_skills",
			"j.application_url", "j.company_id", "j.status", "j.match_score",
			"j.notes", "j.created_at", "j.updated_at", "j.user_id", "j.first_analyzed_at",
			"c.name", "c.created_at", "c.updated_at",
		}).AddRow(
			1, "Matched Job", "Has score", "Remote", int(models.FULL_TIME),
			"https://example.com", `["Go"]`,
			"https://apply.example.com", 1, int(models.INTERESTED), 85,
			"Good match", now.Add(-48*time.Hour), now, testUserID, nil,
			"Tech Corp", now, now,
		).AddRow(
			2, "Unmatched Job", "No score", "Remote", int(models.FULL_TIME),
			"https://example.com", `["Python"]`,
			"https://apply.example.com", 1, int(models.INTERESTED), nil, // NULL match_score
			"Not analyzed", now.Add(-24*time.Hour), now, testUserID, nil,
			"Tech Corp", now, now,
		)

		mock.ExpectQuery("SELECT.*FROM jobs.*ORDER BY CASE WHEN j.match_score IS NULL THEN 1 ELSE 0 END, j.match_score DESC").
			WithArgs(testUserID).
			WillReturnRows(rows)

		filter := models.JobFilter{
			SortBy:    "match_score",
			SortOrder: "desc",
		}
		jobs, err := repo.GetAll(context.Background(), testUserID, filter)

		require.NoError(t, err)
		require.Len(t, jobs, 2)
		assert.NotNil(t, jobs[0].MatchScore, "First job should have non-NULL match score")
		assert.Equal(t, 85, *jobs[0].MatchScore)
		assert.Nil(t, jobs[1].MatchScore, "Second job should have NULL match score (sorted last)")
	})

	t.Run("invalid sort field uses default", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"j.id", "j.title", "j.description", "j.location", "j.job_type",
			"j.source_url", "j.required_skills",
			"j.application_url", "j.company_id", "j.status", "j.match_score",
			"j.notes", "j.created_at", "j.updated_at", "j.user_id", "j.first_analyzed_at",
			"c.name", "c.created_at", "c.updated_at",
		})

		// Invalid sort field should default to updated_at DESC
		mock.ExpectQuery("SELECT.*FROM jobs.*ORDER BY j.updated_at DESC").
			WithArgs(testUserID).
			WillReturnRows(rows)

		filter := models.JobFilter{
			SortBy:    "invalid_field", // Invalid field
			SortOrder: "desc",
		}
		_, err := repo.GetAll(context.Background(), testUserID, filter)

		require.NoError(t, err)
	})
}

func TestGetStatsByUserID(t *testing.T) {
	repo, mock, _ := setupJobRepositoryTest(t)

	tests := []struct {
		name        string
		userID      int
		setupMock   func(userID int)
		want        *models.JobStats
		wantErr     bool
		expectedErr string
	}{
		{
			name:   "success with jobs",
			userID: 1,
			setupMock: func(userID int) {
				rows := sqlmock.NewRows([]string{"total_jobs", "applied", "high_match"}).
					AddRow(15, 8, 5)
				mock.ExpectQuery("SELECT.*COUNT.*total_jobs.*FROM jobs").
					WithArgs(int(models.APPLIED), userID).
					WillReturnRows(rows)
			},
			want: &models.JobStats{
				TotalJobs:    15,
				TotalApplied: 8,
				HighMatch:    5,
			},
			wantErr: false,
		},
		{
			name:   "success with no jobs",
			userID: 2,
			setupMock: func(userID int) {
				rows := sqlmock.NewRows([]string{"total_jobs", "applied", "high_match"}).
					AddRow(0, 0, 0)
				mock.ExpectQuery("SELECT.*COUNT.*total_jobs.*FROM jobs").
					WithArgs(int(models.APPLIED), userID).
					WillReturnRows(rows)
			},
			want: &models.JobStats{
				TotalJobs:    0,
				TotalApplied: 0,
				HighMatch:    0,
			},
			wantErr: false,
		},
		{
			name:   "database error",
			userID: 1,
			setupMock: func(userID int) {
				mock.ExpectQuery("SELECT.*COUNT.*total_jobs.*FROM jobs").
					WithArgs(int(models.APPLIED), userID).
					WillReturnError(errors.New("database connection failed"))
			},
			wantErr:     true,
			expectedErr: "failed to get job stats",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock(tt.userID)

			got, err := repo.GetStatsByUserID(context.Background(), tt.userID)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestGetJobStatsByStatus(t *testing.T) {
	repo, mock, _ := setupJobRepositoryTest(t)

	tests := []struct {
		name        string
		userID      int
		setupMock   func(userID int)
		want        map[models.JobStatus]int
		wantErr     bool
		expectedErr string
	}{
		{
			name:   "success with mixed statuses",
			userID: 1,
			setupMock: func(userID int) {
				rows := sqlmock.NewRows([]string{"status", "count"}).
					AddRow(int(models.INTERESTED), 3).
					AddRow(int(models.APPLIED), 5).
					AddRow(int(models.INTERVIEWING), 2).
					AddRow(int(models.REJECTED), 4)
				mock.ExpectQuery("SELECT.*status.*COUNT.*FROM jobs.*GROUP BY status").
					WithArgs(userID).
					WillReturnRows(rows)
			},
			want: map[models.JobStatus]int{
				models.INTERESTED:     3,
				models.APPLIED:        5,
				models.INTERVIEWING:   2,
				models.OFFER_RECEIVED: 0, // Not in result, should be initialized to 0
				models.REJECTED:       4,
				models.NOT_INTERESTED: 0, // Not in result, should be initialized to 0
			},
			wantErr: false,
		},
		{
			name:   "success with no jobs",
			userID: 2,
			setupMock: func(userID int) {
				rows := sqlmock.NewRows([]string{"status", "count"})
				mock.ExpectQuery("SELECT.*status.*COUNT.*FROM jobs.*GROUP BY status").
					WithArgs(userID).
					WillReturnRows(rows)
			},
			want: map[models.JobStatus]int{
				models.INTERESTED:     0,
				models.APPLIED:        0,
				models.INTERVIEWING:   0,
				models.OFFER_RECEIVED: 0,
				models.REJECTED:       0,
				models.NOT_INTERESTED: 0,
			},
			wantErr: false,
		},
		{
			name:   "database error",
			userID: 1,
			setupMock: func(userID int) {
				mock.ExpectQuery("SELECT.*status.*COUNT.*FROM jobs.*GROUP BY status").
					WithArgs(userID).
					WillReturnError(errors.New("query execution failed"))
			},
			wantErr:     true,
			expectedErr: "failed to get job stats",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock(tt.userID)

			got, err := repo.GetJobStatsByStatus(context.Background(), tt.userID)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestGetRecentJobsByUserID(t *testing.T) {
	repo, mock, mockCompanyRepo := setupJobRepositoryTest(t)

	// Setup company in mock
	company := &models.Company{ID: 1, Name: "Test Company"}
	mockCompanyRepo.companies["Test Company"] = company

	tests := []struct {
		name        string
		userID      int
		limit       int
		setupMock   func(userID int)
		want        []*models.Job
		wantErr     bool
		expectedErr string
	}{
		{
			name:   "success with limit",
			userID: 1,
			limit:  2,
			setupMock: func(userID int) {
				now := time.Now()
				rows := sqlmock.NewRows([]string{
					"id", "title", "description", "location", "job_type",
					"source_url", "skills", "application_url", "company_id",
					"status", "match_score", "notes", "created_at", "updated_at", "user_id", "first_analyzed_at",
					"company_name", "company_created_at", "company_updated_at",
				}).AddRow(
					1, "Engineer", "Great job", "Remote", int(models.FULL_TIME),
					"https://example.com", `["Go"]`, "", 1, int(models.APPLIED), 85,
					"Good fit", now, now, testUserID, nil, "Test Company", now, now,
				).AddRow(
					2, "Developer", "Another job", "NYC", int(models.PART_TIME),
					"https://example2.com", `["Python"]`, "", 1, int(models.INTERESTED), 75,
					"Interesting", now, now, testUserID, nil, "Test Company", now, now,
				)

				mock.ExpectQuery("SELECT.*FROM jobs.*WHERE.*user_id.*ORDER BY.*LIMIT").
					WithArgs(testUserID, 2).
					WillReturnRows(rows)
			},
			want: []*models.Job{
				{
					ID:          1,
					UserID:      testUserID,
					Title:       "Engineer",
					Description: "Great job",
					Location:    "Remote",
					JobType:     models.FULL_TIME,
					SourceURL:   "https://example.com",
					Status:      models.APPLIED,
					MatchScore:  intPtr(85),
					Notes:       "Good fit",
					Company:     *company,
				},
				{
					ID:          2,
					UserID:      testUserID,
					Title:       "Developer",
					Description: "Another job",
					Location:    "NYC",
					JobType:     models.PART_TIME,
					SourceURL:   "https://example2.com",
					Status:      models.INTERESTED,
					MatchScore:  intPtr(75),
					Notes:       "Interesting",
					Company:     *company,
				},
			},
			wantErr: false,
		},
		{
			name:   "success with zero limit uses default",
			userID: 1,
			limit:  0,
			setupMock: func(userID int) {
				rows := sqlmock.NewRows([]string{
					"id", "title", "description", "location", "job_type",
					"source_url", "skills", "application_url", "company_id",
					"status", "match_score", "notes", "created_at", "updated_at", "user_id", "first_analyzed_at",
					"company_name", "company_created_at", "company_updated_at",
				})

				mock.ExpectQuery("SELECT.*FROM jobs.*WHERE.*user_id.*ORDER BY.*LIMIT").
					WithArgs(testUserID, 10). // Default limit
					WillReturnRows(rows)
			},
			want:    []*models.Job{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock(tt.userID)

			got, err := repo.GetRecentJobsByUserID(context.Background(), tt.userID, tt.limit)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				require.Len(t, got, len(tt.want))

				for i, job := range got {
					assert.Equal(t, tt.want[i].ID, job.ID)
					assert.Equal(t, tt.want[i].Title, job.Title)
					assert.Equal(t, tt.want[i].Status, job.Status)
					assert.Equal(t, tt.want[i].Company.Name, job.Company.Name)
				}
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// Helper function for tests
func intPtr(i int) *int {
	return &i
}

func TestSQLiteJobRepository_CreateMatchResult(t *testing.T) {
	tests := []struct {
		name        string
		matchResult *models.MatchResult
		setupMock   func(sqlmock.Sqlmock)
		wantErr     bool
		errMsg      string
	}{
		{
			name: "successful creation",
			matchResult: &models.MatchResult{
				JobID:      1,
				MatchScore: 85,
				Strengths:  []string{"Strong technical skills", "Relevant experience"},
				Weaknesses: []string{"Limited industry experience"},
				Highlights: []string{"Led similar project"},
				Feedback:   "Great match overall",
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				strengthsJSON, _ := json.Marshal([]string{"Strong technical skills", "Relevant experience"})
				weaknessesJSON, _ := json.Marshal([]string{"Limited industry experience"})
				highlightsJSON, _ := json.Marshal([]string{"Led similar project"})

				mock.ExpectExec("INSERT INTO match_results").
					WithArgs(1, 85, string(strengthsJSON), string(weaknessesJSON), string(highlightsJSON), "Great match overall", testUserID).
					WillReturnResult(sqlmock.NewResult(123, 1))
			},
			wantErr: false,
		},
		{
			name:        "nil match result",
			matchResult: nil,
			setupMock:   func(mock sqlmock.Sqlmock) {},
			wantErr:     true,
			errMsg:      "invalid job ID",
		},
		{
			name: "database error",
			matchResult: &models.MatchResult{
				JobID:      1,
				MatchScore: 85,
				Strengths:  []string{"Strong skills"},
				Weaknesses: []string{},
				Highlights: []string{},
				Feedback:   "Good match",
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO match_results").
					WillReturnError(errors.New("database error"))
			},
			wantErr: true,
			errMsg:  "Unable to save job. Please try again",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock, _ := setupJobRepositoryTest(t)

			tt.setupMock(mock)

			err := repo.CreateMatchResult(context.Background(), testUserID, tt.matchResult)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, 123, tt.matchResult.ID)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestSQLiteJobRepository_GetJobMatchHistory(t *testing.T) {
	tests := []struct {
		name      string
		jobID     int
		setupMock func(sqlmock.Sqlmock)
		want      []*models.MatchResult
		wantErr   bool
	}{
		{
			name:  "successful retrieval",
			jobID: 1,
			setupMock: func(mock sqlmock.Sqlmock) {
				strengths1, _ := json.Marshal([]string{"Good fit", "Strong skills"})
				weaknesses1, _ := json.Marshal([]string{"Needs training"})
				highlights1, _ := json.Marshal([]string{"Great experience"})

				strengths2, _ := json.Marshal([]string{"Excellent match"})
				weaknesses2, _ := json.Marshal([]string{})
				highlights2, _ := json.Marshal([]string{"Perfect fit"})

				rows := sqlmock.NewRows([]string{"id", "job_id", "match_score", "strengths", "weaknesses", "highlights", "feedback", "created_at"}).
					AddRow(2, 1, 90, string(strengths2), string(weaknesses2), string(highlights2), "Latest analysis", time.Now()).
					AddRow(1, 1, 75, string(strengths1), string(weaknesses1), string(highlights1), "First analysis", time.Now().Add(-24*time.Hour))

				mock.ExpectQuery("SELECT .* FROM match_results WHERE job_id = \\? AND user_id = \\? ORDER BY created_at DESC").
					WithArgs(1, testUserID).
					WillReturnRows(rows)
			},
			want: []*models.MatchResult{
				{
					ID:         2,
					JobID:      1,
					MatchScore: 90,
					Strengths:  []string{"Excellent match"},
					Weaknesses: []string{},
					Highlights: []string{"Perfect fit"},
					Feedback:   "Latest analysis",
				},
				{
					ID:         1,
					JobID:      1,
					MatchScore: 75,
					Strengths:  []string{"Good fit", "Strong skills"},
					Weaknesses: []string{"Needs training"},
					Highlights: []string{"Great experience"},
					Feedback:   "First analysis",
				},
			},
			wantErr: false,
		},
		{
			name:  "no results found",
			jobID: 999,
			setupMock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "job_id", "match_score", "strengths", "weaknesses", "highlights", "feedback", "created_at"})
				mock.ExpectQuery("SELECT .* FROM match_results WHERE job_id = \\? AND user_id = \\? ORDER BY created_at DESC").
					WithArgs(999, testUserID).
					WillReturnRows(rows)
			},
			want:    []*models.MatchResult{},
			wantErr: false,
		},
		{
			name:  "database error",
			jobID: 1,
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .* FROM match_results WHERE job_id = \\? AND user_id = \\? ORDER BY created_at DESC").
					WithArgs(1, testUserID).
					WillReturnError(errors.New("database error"))
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock, _ := setupJobRepositoryTest(t)

			tt.setupMock(mock)

			got, err := repo.GetJobMatchHistory(context.Background(), testUserID, tt.jobID)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, got, len(tt.want))

				for i, result := range got {
					assert.Equal(t, tt.want[i].ID, result.ID)
					assert.Equal(t, tt.want[i].JobID, result.JobID)
					assert.Equal(t, tt.want[i].MatchScore, result.MatchScore)
					assert.Equal(t, tt.want[i].Strengths, result.Strengths)
					assert.Equal(t, tt.want[i].Weaknesses, result.Weaknesses)
					assert.Equal(t, tt.want[i].Highlights, result.Highlights)
					assert.Equal(t, tt.want[i].Feedback, result.Feedback)
				}
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestSQLiteJobRepository_GetRecentMatchResults(t *testing.T) {
	tests := []struct {
		name      string
		limit     int
		setupMock func(sqlmock.Sqlmock)
		want      []*models.MatchResult
		wantErr   bool
	}{
		{
			name:  "successful retrieval with limit",
			limit: 5,
			setupMock: func(mock sqlmock.Sqlmock) {
				strengths, _ := json.Marshal([]string{"Strong match"})
				weaknesses, _ := json.Marshal([]string{"Minor gaps"})
				highlights, _ := json.Marshal([]string{"Excellent fit"})

				rows := sqlmock.NewRows([]string{"id", "job_id", "match_score", "strengths", "weaknesses", "highlights", "feedback", "created_at"}).
					AddRow(3, 2, 88, string(strengths), string(weaknesses), string(highlights), "Recent match", time.Now()).
					AddRow(2, 1, 92, string(strengths), string(weaknesses), string(highlights), "Great match", time.Now().Add(-1*time.Hour)).
					AddRow(1, 3, 75, string(strengths), string(weaknesses), string(highlights), "Good match", time.Now().Add(-2*time.Hour))

				mock.ExpectQuery("SELECT mr\\.id, mr\\.job_id, mr\\.match_score, mr\\.strengths, mr\\.weaknesses, mr\\.highlights, mr\\.feedback, mr\\.created_at FROM match_results mr WHERE mr\\.user_id = \\? ORDER BY mr\\.created_at DESC LIMIT \\?").
					WithArgs(testUserID, 5).
					WillReturnRows(rows)
			},
			want: []*models.MatchResult{
				{ID: 3, JobID: 2, MatchScore: 88, Strengths: []string{"Strong match"}, Weaknesses: []string{"Minor gaps"}, Highlights: []string{"Excellent fit"}, Feedback: "Recent match"},
				{ID: 2, JobID: 1, MatchScore: 92, Strengths: []string{"Strong match"}, Weaknesses: []string{"Minor gaps"}, Highlights: []string{"Excellent fit"}, Feedback: "Great match"},
				{ID: 1, JobID: 3, MatchScore: 75, Strengths: []string{"Strong match"}, Weaknesses: []string{"Minor gaps"}, Highlights: []string{"Excellent fit"}, Feedback: "Good match"},
			},
			wantErr: false,
		},
		{
			name:  "default limit when zero",
			limit: 0,
			setupMock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "job_id", "match_score", "strengths", "weaknesses", "highlights", "feedback", "created_at"})
				mock.ExpectQuery("SELECT mr\\.id, mr\\.job_id, mr\\.match_score, mr\\.strengths, mr\\.weaknesses, mr\\.highlights, mr\\.feedback, mr\\.created_at FROM match_results mr WHERE mr\\.user_id = \\? ORDER BY mr\\.created_at DESC LIMIT \\?").
					WithArgs(testUserID, 10). // Default limit
					WillReturnRows(rows)
			},
			want:    []*models.MatchResult{},
			wantErr: false,
		},
		{
			name:  "database error",
			limit: 5,
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT mr\\.id, mr\\.job_id, mr\\.match_score, mr\\.strengths, mr\\.weaknesses, mr\\.highlights, mr\\.feedback, mr\\.created_at FROM match_results mr WHERE mr\\.user_id = \\? ORDER BY mr\\.created_at DESC LIMIT \\?").
					WithArgs(testUserID, 5).
					WillReturnError(errors.New("database error"))
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock, _ := setupJobRepositoryTest(t)

			tt.setupMock(mock)

			got, err := repo.GetRecentMatchResults(context.Background(), testUserID, tt.limit)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, got, len(tt.want))

				for i, result := range got {
					assert.Equal(t, tt.want[i].ID, result.ID)
					assert.Equal(t, tt.want[i].JobID, result.JobID)
					assert.Equal(t, tt.want[i].MatchScore, result.MatchScore)
					assert.Equal(t, tt.want[i].Feedback, result.Feedback)
				}
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
