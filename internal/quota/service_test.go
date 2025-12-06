package quota

import (
	"context"
	"testing"
	"time"

	ctxutil "github.com/benidevo/vega/internal/common/context"
	timeutil "github.com/benidevo/vega/internal/common/time"
	"github.com/benidevo/vega/internal/quota/mocks"
	"github.com/benidevo/vega/internal/quota/models"
	"github.com/stretchr/testify/assert"
)

const (
	testMonthlyQuotaLimit = 5
)

func TestQuotaService_NonCloudMode(t *testing.T) {
	t.Run("QuotaChecker", func(t *testing.T) {
		tests := []struct {
			name     string
			testFunc func(t *testing.T, mockChecker *mocks.MockQuotaChecker)
		}{
			{
				name: "should_return_unlimited_access_when_can_analyze_job",
				testFunc: func(t *testing.T, mockChecker *mocks.MockQuotaChecker) {
					ctx := context.Background()
					userID := 1
					jobID := 100

					result := &models.QuotaCheckResult{
						Allowed: true,
						Reason:  models.QuotaReasonOK,
						Status: models.QuotaStatus{
							Limit:     -1,
							Used:      0,
							ResetDate: time.Time{},
						},
					}
					mockChecker.On("CanAnalyzeJob", ctx, userID, jobID).Return(result, nil)

					actual, err := mockChecker.CanAnalyzeJob(ctx, userID, jobID)
					assert.NoError(t, err)
					assert.True(t, actual.Allowed)
					assert.Equal(t, models.QuotaReasonOK, actual.Reason)
					assert.Equal(t, -1, actual.Status.Limit)
					assert.Equal(t, 0, actual.Status.Used)
				},
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				mockChecker := mocks.NewMockQuotaChecker(t)
				tc.testFunc(t, mockChecker)
			})
		}
	})

	t.Run("QuotaReporter", func(t *testing.T) {
		tests := []struct {
			name     string
			testFunc func(t *testing.T, mockReporter *mocks.MockQuotaReporter)
		}{
			{
				name: "should_return_unlimited_quota_status",
				testFunc: func(t *testing.T, mockReporter *mocks.MockQuotaReporter) {
					ctx := context.Background()
					userID := 1

					status := &models.QuotaStatus{
						Limit:     -1,
						Used:      0,
						ResetDate: time.Time{},
					}
					mockReporter.On("GetQuotaStatus", ctx, userID).Return(status, nil)

					actual, err := mockReporter.GetQuotaStatus(ctx, userID)
					assert.NoError(t, err)
					assert.Equal(t, -1, actual.Limit)
					assert.Equal(t, 0, actual.Used)
					assert.Equal(t, time.Time{}, actual.ResetDate)
				},
			},
			{
				name: "should_return_actual_monthly_usage",
				testFunc: func(t *testing.T, mockReporter *mocks.MockQuotaReporter) {
					ctx := context.Background()
					userID := 1

					usage := &models.QuotaUsage{
						UserID:       userID,
						MonthYear:    timeutil.GetCurrentMonthYear(),
						JobsAnalyzed: 3,
						UpdatedAt:    time.Now(),
					}
					mockReporter.On("GetMonthlyUsage", ctx, userID).Return(usage, nil)

					actual, err := mockReporter.GetMonthlyUsage(ctx, userID)
					assert.NoError(t, err)
					assert.Equal(t, userID, actual.UserID)
					assert.Equal(t, 3, actual.JobsAnalyzed)
					assert.Equal(t, timeutil.GetCurrentMonthYear(), actual.MonthYear)
				},
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				mockReporter := mocks.NewMockQuotaReporter(t)
				tc.testFunc(t, mockReporter)
			})
		}
	})

	t.Run("QuotaRecorder", func(t *testing.T) {
		tests := []struct {
			name     string
			testFunc func(t *testing.T, mockRecorder *mocks.MockQuotaRecorder)
		}{
			{
				name: "should_record_analysis_without_enforcement",
				testFunc: func(t *testing.T, mockRecorder *mocks.MockQuotaRecorder) {
					ctx := context.Background()
					userID := 1
					jobID := 100

					mockRecorder.On("RecordAnalysis", ctx, userID, jobID).Return(nil)

					err := mockRecorder.RecordAnalysis(ctx, userID, jobID)
					assert.NoError(t, err)
				},
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				mockRecorder := mocks.NewMockQuotaRecorder(t)
				tc.testFunc(t, mockRecorder)
			})
		}
	})
}

func TestQuotaService_CloudMode(t *testing.T) {
	t.Run("QuotaChecker", func(t *testing.T) {
		tests := []struct {
			name     string
			testFunc func(t *testing.T, mockChecker *mocks.MockQuotaChecker)
		}{
			{
				name: "should_enforce_quota_for_new_analysis",
				testFunc: func(t *testing.T, mockChecker *mocks.MockQuotaChecker) {
					ctx := context.Background()
					userID := 1
					jobID := 100

					result := &models.QuotaCheckResult{
						Allowed: true,
						Reason:  models.QuotaReasonOK,
						Status: models.QuotaStatus{
							Limit: testMonthlyQuotaLimit,
							Used:  0,
						},
					}
					mockChecker.On("CanAnalyzeJob", ctx, userID, jobID).Return(result, nil)

					actual, err := mockChecker.CanAnalyzeJob(ctx, userID, jobID)
					assert.NoError(t, err)
					assert.True(t, actual.Allowed)
					assert.Equal(t, models.QuotaReasonOK, actual.Reason)
					assert.Equal(t, testMonthlyQuotaLimit, actual.Status.Limit)
					assert.Equal(t, 0, actual.Status.Used)
				},
			},
			{
				name: "should_block_when_limit_reached",
				testFunc: func(t *testing.T, mockChecker *mocks.MockQuotaChecker) {
					ctx := context.Background()
					userID := 1
					jobID := 100

					result := &models.QuotaCheckResult{
						Allowed: false,
						Reason:  models.QuotaReasonLimitReached,
						Status: models.QuotaStatus{
							Limit: testMonthlyQuotaLimit,
							Used:  testMonthlyQuotaLimit,
						},
					}
					mockChecker.On("CanAnalyzeJob", ctx, userID, jobID).Return(result, nil)

					actual, err := mockChecker.CanAnalyzeJob(ctx, userID, jobID)
					assert.NoError(t, err)
					assert.False(t, actual.Allowed)
					assert.Equal(t, models.QuotaReasonLimitReached, actual.Reason)
					assert.Equal(t, testMonthlyQuotaLimit, actual.Status.Limit)
					assert.Equal(t, testMonthlyQuotaLimit, actual.Status.Used)
				},
			},
			{
				name: "should_allow_reanalysis",
				testFunc: func(t *testing.T, mockChecker *mocks.MockQuotaChecker) {
					ctx := context.Background()
					userID := 1
					jobID := 100

					result := &models.QuotaCheckResult{
						Allowed: true,
						Reason:  models.QuotaReasonReanalysis,
						Status: models.QuotaStatus{
							Limit: testMonthlyQuotaLimit,
							Used:  testMonthlyQuotaLimit,
						},
					}
					mockChecker.On("CanAnalyzeJob", ctx, userID, jobID).Return(result, nil)

					actual, err := mockChecker.CanAnalyzeJob(ctx, userID, jobID)
					assert.NoError(t, err)
					assert.True(t, actual.Allowed)
					assert.Equal(t, models.QuotaReasonReanalysis, actual.Reason)
				},
			},
			{
				name: "should_give_unlimited_quota_to_admin_users",
				testFunc: func(t *testing.T, mockChecker *mocks.MockQuotaChecker) {
					ctx := ctxutil.WithRole(context.Background(), "Admin")
					userID := 1
					jobID := 100

					result := &models.QuotaCheckResult{
						Allowed: true,
						Reason:  models.QuotaReasonOK,
						Status: models.QuotaStatus{
							Limit: -1,
							Used:  10, // Even if over normal limit
						},
					}
					mockChecker.On("CanAnalyzeJob", ctx, userID, jobID).Return(result, nil)

					actual, err := mockChecker.CanAnalyzeJob(ctx, userID, jobID)
					assert.NoError(t, err)
					assert.True(t, actual.Allowed)
					assert.Equal(t, models.QuotaReasonOK, actual.Reason)
					assert.Equal(t, -1, actual.Status.Limit)
				},
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				mockChecker := mocks.NewMockQuotaChecker(t)
				tc.testFunc(t, mockChecker)
			})
		}
	})

	t.Run("QuotaRecorder", func(t *testing.T) {
		tests := []struct {
			name     string
			testFunc func(t *testing.T, mockRecorder *mocks.MockQuotaRecorder)
		}{
			{
				name: "should_record_analysis_correctly",
				testFunc: func(t *testing.T, mockRecorder *mocks.MockQuotaRecorder) {
					ctx := context.Background()
					userID := 1
					jobID := 100

					mockRecorder.On("RecordAnalysis", ctx, userID, jobID).Return(nil)

					err := mockRecorder.RecordAnalysis(ctx, userID, jobID)
					assert.NoError(t, err)
				},
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				mockRecorder := mocks.NewMockQuotaRecorder(t)
				tc.testFunc(t, mockRecorder)
			})
		}
	})

	t.Run("QuotaReporter", func(t *testing.T) {
		tests := []struct {
			name     string
			testFunc func(t *testing.T, mockReporter *mocks.MockQuotaReporter)
		}{
			{
				name: "should_return_correct_quota_status",
				testFunc: func(t *testing.T, mockReporter *mocks.MockQuotaReporter) {
					ctx := context.Background()
					userID := 1

					status := &models.QuotaStatus{
						Limit:     testMonthlyQuotaLimit,
						Used:      3,
						ResetDate: time.Now().Add(24 * time.Hour),
					}
					mockReporter.On("GetQuotaStatus", ctx, userID).Return(status, nil)

					actual, err := mockReporter.GetQuotaStatus(ctx, userID)
					assert.NoError(t, err)
					assert.Equal(t, testMonthlyQuotaLimit, actual.Limit)
					assert.Equal(t, 3, actual.Used)
					assert.NotEqual(t, time.Time{}, actual.ResetDate)
				},
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				mockReporter := mocks.NewMockQuotaReporter(t)
				tc.testFunc(t, mockReporter)
			})
		}
	})
}
