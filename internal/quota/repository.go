package quota

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/benidevo/vega/internal/quota/models"
)

// Repository interface defines methods for quota data access
type Repository interface {
	// Monthly quota methods (existing functionality)
	GetMonthlyUsage(ctx context.Context, userID int, monthYear string) (*models.QuotaUsage, error)
	IncrementMonthlyUsage(ctx context.Context, userID int, monthYear string) error

	// Daily quota methods (new functionality)
	GetDailyUsage(ctx context.Context, userID int, date string, quotaKey string) (int, error)
	IncrementDailyUsage(ctx context.Context, userID int, date string, quotaKey string, amount int) error
	GetAllDailyUsage(ctx context.Context, userID int, date string) (map[string]int, error)

	// Configuration methods
	GetQuotaConfig(ctx context.Context, quotaType string) (*models.QuotaConfig, error)
}

// repository implements the Repository interface
type repository struct {
	db *sql.DB
}

// NewRepository creates a new quota repository
func NewRepository(db *sql.DB) Repository {
	return &repository{db: db}
}

// GetMonthlyUsage gets the monthly usage for a user
func (r *repository) GetMonthlyUsage(ctx context.Context, userID int, monthYear string) (*models.QuotaUsage, error) {
	usage := &models.QuotaUsage{
		UserID:       userID,
		MonthYear:    monthYear,
		JobsAnalyzed: 0,
		UpdatedAt:    time.Now(),
	}

	query := `
		SELECT user_id, month_year, jobs_analyzed, updated_at
		FROM user_quota_usage
		WHERE user_id = ? AND month_year = ?
	`

	row := r.db.QueryRowContext(ctx, query, userID, monthYear)
	err := row.Scan(&usage.UserID, &usage.MonthYear, &usage.JobsAnalyzed, &usage.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			// No usage recorded yet for this month, return zero usage
			return usage, nil
		}
		return nil, fmt.Errorf("failed to get quota usage: %w", err)
	}

	return usage, nil
}

// IncrementMonthlyUsage increments the monthly usage count
func (r *repository) IncrementMonthlyUsage(ctx context.Context, userID int, monthYear string) error {
	// Use UPSERT pattern to avoid race conditions
	upsertQuery := `
		INSERT INTO user_quota_usage (user_id, month_year, jobs_analyzed, updated_at)
		VALUES (?, ?, 1, CURRENT_TIMESTAMP)
		ON CONFLICT(user_id, month_year) DO UPDATE SET
			jobs_analyzed = jobs_analyzed + 1,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := r.db.ExecContext(ctx, upsertQuery, userID, monthYear)
	if err != nil {
		return fmt.Errorf("failed to update quota usage: %w", err)
	}

	return nil
}

// GetDailyUsage gets the daily usage for a specific quota key
func (r *repository) GetDailyUsage(ctx context.Context, userID int, date string, quotaKey string) (int, error) {
	var value int

	query := `
		SELECT value
		FROM user_daily_quotas
		WHERE user_id = ? AND date = ? AND quota_key = ?
	`

	row := r.db.QueryRowContext(ctx, query, userID, date, quotaKey)
	err := row.Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			// No usage recorded yet, return 0
			return 0, nil
		}
		return 0, fmt.Errorf("failed to get daily quota usage: %w", err)
	}

	return value, nil
}

// IncrementDailyUsage increments the daily usage for a specific quota key
func (r *repository) IncrementDailyUsage(ctx context.Context, userID int, date string, quotaKey string, amount int) error {
	// Use UPSERT pattern
	upsertQuery := `
		INSERT INTO user_daily_quotas (user_id, date, quota_key, value, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(user_id, date, quota_key) DO UPDATE SET
			value = value + ?,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := r.db.ExecContext(ctx, upsertQuery, userID, date, quotaKey, amount, amount)
	if err != nil {
		return fmt.Errorf("failed to update daily quota usage: %w", err)
	}

	return nil
}

// GetAllDailyUsage gets all daily quota usage for a user on a specific date
func (r *repository) GetAllDailyUsage(ctx context.Context, userID int, date string) (map[string]int, error) {
	query := `
		SELECT quota_key, value
		FROM user_daily_quotas
		WHERE user_id = ? AND date = ?
	`

	rows, err := r.db.QueryContext(ctx, query, userID, date)
	if err != nil {
		return nil, fmt.Errorf("failed to query daily quota usage: %w", err)
	}
	defer rows.Close()

	usage := make(map[string]int)
	for rows.Next() {
		var key string
		var value int
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		usage[key] = value
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return usage, nil
}

// GetQuotaConfig gets the quota configuration for a specific quota type
func (r *repository) GetQuotaConfig(ctx context.Context, quotaType string) (*models.QuotaConfig, error) {
	config := &models.QuotaConfig{}

	query := `
		SELECT quota_type, free_limit, description, created_at, updated_at
		FROM quota_configs
		WHERE quota_type = ?
	`

	row := r.db.QueryRowContext(ctx, query, quotaType)
	err := row.Scan(&config.QuotaType, &config.FreeLimit, &config.Description, &config.CreatedAt, &config.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get quota config for %s: %w", quotaType, err)
	}

	return config, nil
}
