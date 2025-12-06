package quota

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/benidevo/vega/internal/quota/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepository_GetMonthlyUsage(t *testing.T) {
	tests := []struct {
		name          string
		userID        int
		monthYear     string
		setupMock     func(sqlmock.Sqlmock)
		expectedUsage *models.QuotaUsage
		expectError   bool
		errorContains string
	}{
		{
			name:      "should_return_usage_when_exists",
			userID:    1,
			monthYear: "2024-01",
			setupMock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"user_id", "month_year", "jobs_analyzed", "updated_at"}).
					AddRow(1, "2024-01", 5, time.Now())
				mock.ExpectQuery("SELECT user_id, month_year, jobs_analyzed, updated_at FROM user_quota_usage").
					WithArgs(1, "2024-01").
					WillReturnRows(rows)
			},
			expectedUsage: &models.QuotaUsage{
				UserID:       1,
				MonthYear:    "2024-01",
				JobsAnalyzed: 5,
			},
		},
		{
			name:      "should_return_zero_usage_when_not_exists",
			userID:    2,
			monthYear: "2024-01",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT user_id, month_year, jobs_analyzed, updated_at FROM user_quota_usage").
					WithArgs(2, "2024-01").
					WillReturnError(sql.ErrNoRows)
			},
			expectedUsage: &models.QuotaUsage{
				UserID:       2,
				MonthYear:    "2024-01",
				JobsAnalyzed: 0,
			},
		},
		{
			name:      "should_return_error_when_database_fails",
			userID:    1,
			monthYear: "2024-01",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT user_id, month_year, jobs_analyzed, updated_at FROM user_quota_usage").
					WithArgs(1, "2024-01").
					WillReturnError(sql.ErrConnDone)
			},
			expectError:   true,
			errorContains: "failed to get quota usage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			if tt.setupMock != nil {
				tt.setupMock(mock)
			}

			repo := NewRepository(db)
			usage, err := repo.GetMonthlyUsage(context.Background(), tt.userID, tt.monthYear)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, usage)
				assert.Equal(t, tt.expectedUsage.UserID, usage.UserID)
				assert.Equal(t, tt.expectedUsage.MonthYear, usage.MonthYear)
				assert.Equal(t, tt.expectedUsage.JobsAnalyzed, usage.JobsAnalyzed)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestRepository_IncrementMonthlyUsage(t *testing.T) {
	tests := []struct {
		name          string
		userID        int
		monthYear     string
		setupMock     func(sqlmock.Sqlmock)
		expectError   bool
		errorContains string
	}{
		{
			name:      "should_increment_usage_when_successful",
			userID:    1,
			monthYear: "2024-01",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO user_quota_usage").
					WithArgs(1, "2024-01").
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
		{
			name:      "should_handle_upsert_when_record_exists",
			userID:    2,
			monthYear: "2024-01",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO user_quota_usage").
					WithArgs(2, "2024-01").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			name:      "should_return_error_when_database_fails",
			userID:    1,
			monthYear: "2024-01",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO user_quota_usage").
					WithArgs(1, "2024-01").
					WillReturnError(sql.ErrConnDone)
			},
			expectError:   true,
			errorContains: "failed to update quota usage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			if tt.setupMock != nil {
				tt.setupMock(mock)
			}

			repo := NewRepository(db)
			err = repo.IncrementMonthlyUsage(context.Background(), tt.userID, tt.monthYear)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				assert.NoError(t, err)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestRepository_GetDailyUsage(t *testing.T) {
	tests := []struct {
		name          string
		userID        int
		date          string
		quotaKey      string
		setupMock     func(sqlmock.Sqlmock)
		expectedValue int
		expectError   bool
		errorContains string
	}{
		{
			name:     "should_return_value_when_exists",
			userID:   1,
			date:     "2024-01-15",
			quotaKey: models.QuotaKeyJobsCaptured,
			setupMock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"value"}).AddRow(10)
				mock.ExpectQuery("SELECT value FROM user_daily_quotas").
					WithArgs(1, "2024-01-15", models.QuotaKeyJobsCaptured).
					WillReturnRows(rows)
			},
			expectedValue: 10,
		},
		{
			name:     "should_return_zero_when_not_exists",
			userID:   2,
			date:     "2024-01-15",
			quotaKey: models.QuotaKeyJobsCaptured,
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT value FROM user_daily_quotas").
					WithArgs(2, "2024-01-15", models.QuotaKeyJobsCaptured).
					WillReturnError(sql.ErrNoRows)
			},
			expectedValue: 0,
		},
		{
			name:     "should_return_error_when_database_fails",
			userID:   1,
			date:     "2024-01-15",
			quotaKey: models.QuotaKeyJobsCaptured,
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT value FROM user_daily_quotas").
					WithArgs(1, "2024-01-15", models.QuotaKeyJobsCaptured).
					WillReturnError(sql.ErrConnDone)
			},
			expectError:   true,
			errorContains: "failed to get daily quota usage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			if tt.setupMock != nil {
				tt.setupMock(mock)
			}

			repo := NewRepository(db)
			value, err := repo.GetDailyUsage(context.Background(), tt.userID, tt.date, tt.quotaKey)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedValue, value)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestRepository_IncrementDailyUsage(t *testing.T) {
	tests := []struct {
		name          string
		userID        int
		date          string
		quotaKey      string
		amount        int
		setupMock     func(sqlmock.Sqlmock)
		expectError   bool
		errorContains string
	}{
		{
			name:     "should_increment_usage_when_successful",
			userID:   1,
			date:     "2024-01-15",
			quotaKey: models.QuotaKeyJobsCaptured,
			amount:   5,
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO user_daily_quotas").
					WithArgs(1, "2024-01-15", models.QuotaKeyJobsCaptured, 5, 5).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
		{
			name:     "should_handle_upsert_when_record_exists",
			userID:   2,
			date:     "2024-01-15",
			quotaKey: models.QuotaKeyJobsCaptured,
			amount:   3,
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO user_daily_quotas").
					WithArgs(2, "2024-01-15", models.QuotaKeyJobsCaptured, 3, 3).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			name:     "should_return_error_when_database_fails",
			userID:   1,
			date:     "2024-01-15",
			quotaKey: models.QuotaKeyJobsCaptured,
			amount:   1,
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO user_daily_quotas").
					WithArgs(1, "2024-01-15", models.QuotaKeyJobsCaptured, 1, 1).
					WillReturnError(sql.ErrConnDone)
			},
			expectError:   true,
			errorContains: "failed to update daily quota usage",
		},
		{
			name:     "should_handle_zero_amount",
			userID:   3,
			date:     "2024-01-15",
			quotaKey: models.QuotaKeyJobsCaptured,
			amount:   0,
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO user_daily_quotas").
					WithArgs(3, "2024-01-15", models.QuotaKeyJobsCaptured, 0, 0).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			if tt.setupMock != nil {
				tt.setupMock(mock)
			}

			repo := NewRepository(db)
			err = repo.IncrementDailyUsage(context.Background(), tt.userID, tt.date, tt.quotaKey, tt.amount)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				assert.NoError(t, err)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestRepository_GetAllDailyUsage(t *testing.T) {
	tests := []struct {
		name          string
		userID        int
		date          string
		setupMock     func(sqlmock.Sqlmock)
		expectedUsage map[string]int
		expectError   bool
		errorContains string
	}{
		{
			name:   "should_return_all_usage_when_exists",
			userID: 1,
			date:   "2024-01-15",
			setupMock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"quota_key", "value"}).
					AddRow(models.QuotaKeyJobsCaptured, 10).
					AddRow("other_key", 5)
				mock.ExpectQuery("SELECT quota_key, value FROM user_daily_quotas").
					WithArgs(1, "2024-01-15").
					WillReturnRows(rows)
			},
			expectedUsage: map[string]int{
				models.QuotaKeyJobsCaptured: 10,
				"other_key":                 5,
			},
		},
		{
			name:   "should_return_empty_map_when_no_usage",
			userID: 2,
			date:   "2024-01-15",
			setupMock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"quota_key", "value"})
				mock.ExpectQuery("SELECT quota_key, value FROM user_daily_quotas").
					WithArgs(2, "2024-01-15").
					WillReturnRows(rows)
			},
			expectedUsage: map[string]int{},
		},
		{
			name:   "should_return_error_when_database_fails",
			userID: 1,
			date:   "2024-01-15",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT quota_key, value FROM user_daily_quotas").
					WithArgs(1, "2024-01-15").
					WillReturnError(sql.ErrConnDone)
			},
			expectError:   true,
			errorContains: "failed to query daily quota usage",
		},
		{
			name:   "should_return_error_when_scan_fails",
			userID: 1,
			date:   "2024-01-15",
			setupMock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"quota_key", "invalid"}).
					AddRow(models.QuotaKeyJobsCaptured, "not a number")
				mock.ExpectQuery("SELECT quota_key, value FROM user_daily_quotas").
					WithArgs(1, "2024-01-15").
					WillReturnRows(rows)
			},
			expectError:   true,
			errorContains: "failed to scan row",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			if tt.setupMock != nil {
				tt.setupMock(mock)
			}

			repo := NewRepository(db)
			usage, err := repo.GetAllDailyUsage(context.Background(), tt.userID, tt.date)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedUsage, usage)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestRepository_GetQuotaConfig(t *testing.T) {
	tests := []struct {
		name           string
		quotaType      string
		setupMock      func(sqlmock.Sqlmock)
		expectedConfig *models.QuotaConfig
		expectError    bool
		errorContains  string
	}{
		{
			name:      "should_return_config_when_exists",
			quotaType: models.QuotaTypeAIAnalysis,
			setupMock: func(mock sqlmock.Sqlmock) {
				createdAt := time.Now().Add(-24 * time.Hour)
				updatedAt := time.Now()
				rows := sqlmock.NewRows([]string{"quota_type", "free_limit", "description", "created_at", "updated_at"}).
					AddRow(models.QuotaTypeAIAnalysis, 10, "AI Analysis quota", createdAt, updatedAt)
				mock.ExpectQuery("SELECT quota_type, free_limit, description, created_at, updated_at FROM quota_configs").
					WithArgs(models.QuotaTypeAIAnalysis).
					WillReturnRows(rows)
			},
			expectedConfig: &models.QuotaConfig{
				QuotaType:   models.QuotaTypeAIAnalysis,
				FreeLimit:   10,
				Description: "AI Analysis quota",
			},
		},
		{
			name:      "should_return_error_when_not_found",
			quotaType: "unknown",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT quota_type, free_limit, description, created_at, updated_at FROM quota_configs").
					WithArgs("unknown").
					WillReturnError(sql.ErrNoRows)
			},
			expectError:   true,
			errorContains: "failed to get quota config",
		},
		{
			name:      "should_return_error_when_database_fails",
			quotaType: models.QuotaTypeJobCapture,
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT quota_type, free_limit, description, created_at, updated_at FROM quota_configs").
					WithArgs(models.QuotaTypeJobCapture).
					WillReturnError(sql.ErrConnDone)
			},
			expectError:   true,
			errorContains: "failed to get quota config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			if tt.setupMock != nil {
				tt.setupMock(mock)
			}

			repo := NewRepository(db)
			config, err := repo.GetQuotaConfig(context.Background(), tt.quotaType)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, config)
				assert.Equal(t, tt.expectedConfig.QuotaType, config.QuotaType)
				assert.Equal(t, tt.expectedConfig.FreeLimit, config.FreeLimit)
				assert.Equal(t, tt.expectedConfig.Description, config.Description)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
