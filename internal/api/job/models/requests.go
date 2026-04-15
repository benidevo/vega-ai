package models

// CreateJobRequest represents the request payload for creating a job
type CreateJobRequest struct {
	Title          string `json:"title" binding:"required,max=255"`
	Company        string `json:"company" binding:"required,max=255"`
	Location       string `json:"location" binding:"required,max=255"`
	Description    string `json:"description" binding:"required"`
	JobType        string `json:"jobType,omitempty"`
	ApplicationURL string `json:"applicationUrl,omitempty" binding:"omitempty,max=2048"`
	SourceURL      string `json:"sourceUrl" binding:"required,max=2048"`
	Notes          string `json:"notes,omitempty" binding:"omitempty,max=5000"`
}

// CreateJobResponse represents the response after creating a job
type CreateJobResponse struct {
	Message string `json:"message"`
	JobID   int    `json:"jobId,omitempty"`
}
