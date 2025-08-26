package models

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// JobStatus represents the status of a job application.
type JobStatus int

const (
	INTERESTED JobStatus = iota
	APPLIED
	INTERVIEWING
	OFFER_RECEIVED
	REJECTED
	NOT_INTERESTED
)

// FromString converts a string representation of a job status to its corresponding JobStatus value.
func JobStatusFromString(status string) (JobStatus, error) {
	switch strings.ToLower(status) {
	case "interested":
		return INTERESTED, nil
	case "applied":
		return APPLIED, nil
	case "interviewing":
		return INTERVIEWING, nil
	case "offered", "offer received", "offer_received":
		return OFFER_RECEIVED, nil
	case "rejected":
		return REJECTED, nil
	case "not_interested", "not interested":
		return NOT_INTERESTED, nil
	default:
		return -1, ErrInvalidJobStatus
	}
}

// String returns the string representation of the JobStatus value.
func (j JobStatus) String() string {
	switch j {
	case INTERESTED:
		return "Interested"
	case APPLIED:
		return "Applied"
	case INTERVIEWING:
		return "Interviewing"
	case OFFER_RECEIVED:
		return "Offer Received"
	case REJECTED:
		return "Rejected"
	case NOT_INTERESTED:
		return "Not Interested"
	default:
		return "Unknown"
	}
}

// JobType represents the type of job (e.g., full-time, part-time, etc.).
type JobType int

const (
	FULL_TIME JobType = iota
	PART_TIME
	CONTRACT
	INTERN
	REMOTE
	FREELANCE
	OTHER
)

// FromString converts a string representation of a job type to its corresponding JobType constant.
func JobTypeFromString(jobType string) JobType {
	switch strings.ToLower(jobType) {
	case "full_time":
		return FULL_TIME
	case "part_time":
		return PART_TIME
	case "contract":
		return CONTRACT
	case "intern":
		return INTERN
	case "remote":
		return REMOTE
	case "freelance":
		return FREELANCE
	default:
		return OTHER
	}
}

// String returns the string representation of the JobType.
func (j JobType) String() string {
	switch j {
	case FULL_TIME:
		return "Full Time"
	case PART_TIME:
		return "Part Time"
	case CONTRACT:
		return "Contract"
	case INTERN:
		return "Intern"
	case REMOTE:
		return "Remote"
	case FREELANCE:
		return "Freelance"
	default:
		return "Other"
	}
}

// Job represents a job posting.
// It includes metadata for database mapping and JSON serialization.
type Job struct {
	ID              int        `json:"id" db:"id" sql:"primary_key;auto_increment"`
	UserID          int        `json:"user_id" db:"user_id" sql:"not null;index" validate:"required"`
	Title           string     `json:"title" db:"title" sql:"type:text;not null;index" validate:"required,min=1,max=255"`
	Description     string     `json:"description" db:"description" sql:"type:text;not null" validate:"required,min=1"`
	Location        string     `json:"location" db:"location" sql:"type:text" validate:"max=255"`
	JobType         JobType    `json:"job_type" db:"job_type" sql:"type:integer;not null;default:0" validate:"min=0,max=6"`
	SourceURL       string     `json:"source_url" db:"source_url" sql:"type:text;index" validate:"omitempty,url"`
	RequiredSkills  []string   `json:"required_skills" db:"required_skills" sql:"type:text" validate:"max=50,dive,max=100"` // Stored as JSON
	ApplicationURL  string     `json:"application_url" db:"application_url" sql:"type:text" validate:"omitempty,url"`
	Company         Company    `json:"company" sql:"-" validate:"required"` // Not stored directly, company_id is used instead
	Status          JobStatus  `json:"status" db:"status" sql:"type:integer;not null;default:0;index" validate:"min=0,max=5"`
	MatchScore      *int       `json:"match_score,omitempty" db:"match_score" sql:"type:integer;index" validate:"omitempty,min=0,max=100"`
	Notes           string     `json:"notes,omitempty" db:"notes" sql:"type:text" validate:"max=5000"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at" sql:"type:timestamp;not null;default:current_timestamp"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at" sql:"type:timestamp;not null;default:current_timestamp"`
	FirstAnalyzedAt *time.Time `json:"first_analyzed_at,omitempty" db:"first_analyzed_at" sql:"type:timestamp"`

	// SQL-only fields
	CompanyID int `json:"-" db:"company_id" sql:"type:integer;not null;index;references:companies(id)"`
}

// JobOption defines a function for configuring a Job
type JobOption func(*Job)

// WithJobType sets the job type
func WithJobType(jobType JobType) JobOption {
	return func(j *Job) {
		j.JobType = jobType
	}
}

// WithLocation sets the job location
func WithLocation(location string) JobOption {
	return func(j *Job) {
		j.Location = location
	}
}

// WithSourceURL sets the job source URL
func WithSourceURL(url string) JobOption {
	return func(j *Job) {
		j.SourceURL = url
	}
}

// WithRequiredSkills sets the required skills for the job
func WithRequiredSkills(skills []string) JobOption {
	return func(j *Job) {
		j.RequiredSkills = skills
	}
}

// WithApplicationURL sets the application URL
func WithApplicationURL(url string) JobOption {
	return func(j *Job) {
		j.ApplicationURL = url
	}
}

// WithStatus sets the job application status
func WithStatus(status JobStatus) JobOption {
	return func(j *Job) {
		j.Status = status
	}
}

// WithNotes sets the notes for the job
func WithNotes(notes string) JobOption {
	return func(j *Job) {
		j.Notes = notes
	}
}

// NewJob creates a new Job instance with the required fields and applies the provided options.
// Only title, description, and company are required. All other fields can be set using options.
func NewJob(title, description string, company Company, options ...JobOption) *Job {
	now := time.Now().UTC()
	job := &Job{
		Title:          title,
		Description:    description,
		Company:        company,
		Status:         INTERESTED,
		RequiredSkills: []string{},
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	for _, option := range options {
		option(job)
	}

	return job
}

// Validate performs basic validation on the Job struct
func (j *Job) Validate() error {
	if j.Title == "" {
		return ErrJobTitleRequired
	}

	if j.Description == "" {
		return ErrJobDescriptionRequired
	}

	if j.Company.Name == "" {
		return ErrCompanyRequired
	}

	return nil
}

// IsMatched returns true if the job has a match score >= 70
func (j *Job) IsMatched() bool {
	return j.MatchScore != nil && *j.MatchScore >= 70
}

// GetMatchScoreString returns the match score as a string, or "Unmatched" if no score
func (j *Job) GetMatchScoreString() string {
	if j.MatchScore == nil {
		return "Unmatched"
	}
	return fmt.Sprintf("%d%% Match", *j.MatchScore)
}

// GetMatchStatus returns "Matched" or "Unmatched" based on the score
func (j *Job) GetMatchStatus() string {
	if j.IsMatched() {
		return "Matched"
	}
	return "Unmatched"
}

// JobFilter defines filters for querying jobs
type JobFilter struct {
	CompanyID *int
	Status    *JobStatus
	JobType   *JobType
	Matched   *bool
	Limit     int
	Offset    int
	SortBy    string // "match_score", "updated_at", etc.
	SortOrder string // "asc" or "desc"
}

type JobStats struct {
	TotalJobs    int `json:"total_jobs"`
	TotalApplied int `json:"total_applied"`
	HighMatch    int `json:"high_match"`
}

type PaginationInfo struct {
	CurrentPage  int  `json:"current_page"`
	TotalPages   int  `json:"total_pages"`
	TotalItems   int  `json:"total_items"`
	ItemsPerPage int  `json:"items_per_page"`
	HasNext      bool `json:"has_next"`
	HasPrev      bool `json:"has_prev"`
}

type JobsWithPagination struct {
	Jobs       []*Job          `json:"jobs"`
	Pagination *PaginationInfo `json:"pagination"`
}

// CoverLetter represents a generated cover letter in the job domain.
type CoverLetter struct {
	ID          int       `json:"id"`
	JobID       int       `json:"jobId"`
	UserID      int       `json:"userId"`
	Content     string    `json:"content"`
	Format      string    `json:"format"`
	GeneratedAt time.Time `json:"generatedAt"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// CoverLetterWithProfile holds a cover letter along with user profile information
type CoverLetterWithProfile struct {
	CoverLetter  *CoverLetter  `json:"coverLetter"`
	PersonalInfo *PersonalInfo `json:"personalInfo"`
}

// JobMatchAnalysis represents a job match analysis result in the job domain.
type JobMatchAnalysis struct {
	ID         int       `json:"id"`
	JobID      int       `json:"jobId"`
	UserID     int       `json:"userId"`
	MatchScore int       `json:"matchScore"`
	Strengths  []string  `json:"strengths"`
	Weaknesses []string  `json:"weaknesses"`
	Highlights []string  `json:"highlights"`
	Feedback   string    `json:"feedback"`
	AnalyzedAt time.Time `json:"analyzedAt"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// ParsePositiveInt parses a string to a positive integer, returns error if invalid or negative
func ParsePositiveInt(s string) (int, error) {
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	if i <= 0 {
		return 0, strconv.ErrRange
	}
	return i, nil
}
