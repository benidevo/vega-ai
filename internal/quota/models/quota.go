package models

import (
	"time"
)

// QuotaUsage represents a user's quota usage for a specific month
type QuotaUsage struct {
	UserID       int       `db:"user_id"`
	MonthYear    string    `db:"month_year"`
	JobsAnalyzed int       `db:"jobs_analyzed"`
	UpdatedAt    time.Time `db:"updated_at"`
}

// QuotaStatus represents the current quota status for a user
type QuotaStatus struct {
	Used      int       `json:"used"`
	Limit     int       `json:"limit"`
	ResetDate time.Time `json:"reset_date"`
}

// QuotaCheckResult represents the result of a quota check
type QuotaCheckResult struct {
	Allowed bool        `json:"allowed"`
	Reason  string      `json:"reason"`
	Status  QuotaStatus `json:"status"`
}

// Job represents a minimal job structure for quota checking
type Job struct {
	ID              int        `db:"id"`
	FirstAnalyzedAt *time.Time `db:"first_analyzed_at"`
}

// DailyQuota represents daily quota usage
type DailyQuota struct {
	UserID    int       `db:"user_id"`
	Date      string    `db:"date"`
	QuotaKey  string    `db:"quota_key"`
	Value     int       `db:"value"`
	UpdatedAt time.Time `db:"updated_at"`
}

// UnifiedQuotaStatus combines all quota statuses
type UnifiedQuotaStatus struct {
	AIAnalysis QuotaStatus `json:"ai_analysis"`
	JobCapture QuotaStatus `json:"job_capture"`
}

// QuotaConfig represents quota configuration from database
type QuotaConfig struct {
	QuotaType   string    `db:"quota_type"`
	FreeLimit   int       `db:"free_limit"`
	Description string    `db:"description"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

const (
	// Quota types
	QuotaTypeAIAnalysis = "ai_analysis"
	QuotaTypeJobCapture = "job_capture"

	// Period types
	PeriodDaily   = "daily"
	PeriodMonthly = "monthly"

	// QuotaReasonOK indicates the operation is allowed
	QuotaReasonOK = "ok"

	// QuotaReasonReanalysis indicates this is a re-analysis (always allowed)
	QuotaReasonReanalysis = "re-analysis allowed"

	// QuotaReasonLimitReached indicates the quota limit has been reached
	QuotaReasonLimitReached = "quota limit reached"

	// Daily quota keys
	QuotaKeyJobsCaptured = "jobs_captured"
)
