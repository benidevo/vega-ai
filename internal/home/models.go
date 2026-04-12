package home

import (
	"time"

	"github.com/benidevo/vega/internal/job/models"
)

// HomePageData aggregates all data needed for the homepage
type HomePageData struct {
	UserID         int             `json:"user_id"`
	Username       string          `json:"username"`
	Stats          JobStatsSummary `json:"stats"`
	RecentJobs     []JobSummary    `json:"recent_jobs"`
	TopMatches     []JobSummary    `json:"top_matches"`
	HasJobs        bool            `json:"has_jobs"`
	ShowOnboarding bool            `json:"show_onboarding"`
	Title          string          `json:"title"`
	Page           string          `json:"page"`
	QuotaStatus    *QuotaStatus    `json:"quota_status"`
}

// QuotaStatus represents the current quota status for a user
type QuotaStatus struct {
	Used       int       `json:"used"`
	Limit      int       `json:"limit"`
	Remaining  int       `json:"remaining"`
	ResetDate  time.Time `json:"reset_date"`
	Percentage int       `json:"percentage"`
}

// JobStatsSummary provides key metrics for homepage display
type JobStatsSummary struct {
	TotalJobs     int `json:"total_jobs"`
	Applied       int `json:"applied"`
	Interviewing  int `json:"interviewing"`
	ActiveJobs    int `json:"active_jobs"`
	OfferReceived int `json:"offer_received"`
	Interested    int `json:"interested"`
}

// JobSummary provides essential job info for homepage listings
type JobSummary struct {
	ID         int    `json:"id"`
	Title      string `json:"title"`
	Company    string `json:"company"`
	Location   string `json:"location"`
	Status     int    `json:"status"`
	StatusText string `json:"status_text"`
	MatchScore int    `json:"match_score"`
}

// ToJobSummary converts a models.Job to JobSummary for homepage display
func ToJobSummary(job *models.Job) JobSummary {
	matchScore := 0
	if job.MatchScore != nil {
		matchScore = *job.MatchScore
	}

	return JobSummary{
		ID:         job.ID,
		Title:      job.Title,
		Company:    getCompanyName(job),
		Location:   job.Location,
		Status:     int(job.Status),
		StatusText: job.Status.String(),
		MatchScore: matchScore,
	}
}

func getCompanyName(job *models.Job) string {
	if job.Company.Name != "" {
		return job.Company.Name
	}

	return "Unknown Company"
}

// NewHomePageData creates a new HomePageData instance with defaults
func NewHomePageData(userID int, username string) *HomePageData {
	return &HomePageData{
		UserID:         userID,
		Username:       username,
		Title:          "Home",
		Page:           "dashboard-home",
		HasJobs:        false,
		ShowOnboarding: true,
		Stats: JobStatsSummary{
			TotalJobs:     0,
			Applied:       0,
			Interviewing:  0,
			ActiveJobs:    0,
			OfferReceived: 0,
			Interested:    0,
		},
		RecentJobs: make([]JobSummary, 0),
	}
}
