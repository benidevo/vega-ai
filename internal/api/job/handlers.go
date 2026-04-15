package job

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"

	apimodels "github.com/benidevo/vega/internal/api/job/models"
	ctxutil "github.com/benidevo/vega/internal/common/context"
	"github.com/benidevo/vega/internal/job/models"
	quotamodels "github.com/benidevo/vega/internal/quota/models"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// JobAPIHandler handles job-related API requests
type JobAPIHandler struct {
	jobService   jobService
	quotaService quotaService
}

// NewJobAPIHandler creates a new job API handler
func NewJobAPIHandler(jobService jobService, quotaService quotaService) *JobAPIHandler {
	return &JobAPIHandler{
		jobService:   jobService,
		quotaService: quotaService,
	}
}

// CreateJob handles the creation of a new job from API request
func (h *JobAPIHandler) CreateJob(c *gin.Context) {
	userIDValue, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized",
		})
		return
	}
	userID := userIDValue.(int)

	var req apimodels.CreateJobRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.jobService.LogError(err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": h.formatValidationError(err),
		})
		return
	}

	jobType := models.JobTypeFromString(req.JobType)

	jobOptions := []models.JobOption{
		models.WithLocation(req.Location),
		models.WithJobType(jobType),
		models.WithSourceURL(req.SourceURL),
		models.WithApplicationURL(req.ApplicationURL),
		models.WithNotes(req.Notes),
		models.WithStatus(models.INTERESTED),
	}

	// Get context with role information
	ctx := c.Request.Context()
	if roleValue, exists := c.Get("role"); exists {
		if role, ok := roleValue.(string); ok {
			ctx = ctxutil.WithRole(ctx, role)
		}
	}

	createdJob, isNew, err := h.jobService.CreateJob(
		ctx,
		userID,
		req.Title,
		req.Description,
		req.Company,
		jobOptions...,
	)

	if err != nil {
		if _, ok := err.(validator.ValidationErrors); ok {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": h.formatValidationError(err),
			})
			return
		}

		switch err {
		case models.ErrJobTitleRequired,
			models.ErrJobDescriptionRequired,
			models.ErrCompanyRequired:
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
		case models.ErrInvalidURLFormat:
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid URL provided",
			})
		case models.ErrDuplicateJob:
			c.JSON(http.StatusConflict, gin.H{
				"error": "Job already exists with this source URL",
			})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to create job",
			})
		}
		return
	}

	// Record job creation in the job search quota only if it's a new job
	// This tracks jobs captured via the extension
	if h.quotaService != nil && isNew {
		// Get context with role information
		ctx := c.Request.Context()
		if roleValue, exists := c.Get("role"); exists {
			if role, ok := roleValue.(string); ok {
				ctx = ctxutil.WithRole(ctx, role)
			}
		}

		err = h.quotaService.RecordUsage(ctx, userID, quotamodels.QuotaTypeJobCapture, map[string]interface{}{
			"count": 1,
		})
		if err != nil {
			// Log the error but don't fail the job creation
			h.jobService.LogError(err)
		}
	}

	c.JSON(http.StatusOK, apimodels.CreateJobResponse{
		Message: "Job created successfully",
		JobID:   createdJob.ID,
	})
}

// GetQuotaStatus returns the current quota status for the authenticated user
func (h *JobAPIHandler) GetQuotaStatus(c *gin.Context) {
	userIDValue, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized",
		})
		return
	}
	userID := userIDValue.(int)

	// Get context with role information
	ctx := c.Request.Context()
	if roleValue, exists := c.Get("role"); exists {
		if role, ok := roleValue.(string); ok {
			ctx = ctxutil.WithRole(ctx, role)
		}
	}

	quotaStatus, err := h.jobService.GetQuotaStatus(ctx, userID)
	if err != nil {
		h.jobService.LogError(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get quota status",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"used":  quotaStatus.Used,
		"limit": quotaStatus.Limit,
		"remaining": func() int {
			if quotaStatus.Limit < 0 {
				return -1 // Unlimited
			}
			remaining := quotaStatus.Limit - quotaStatus.Used
			if remaining < 0 {
				return 0
			}
			return remaining
		}(),
		"reset_date": quotaStatus.ResetDate.Format("2006-01-02"),
	})
}

func (h *JobAPIHandler) formatValidationError(err error) string {
	if ve, ok := err.(validator.ValidationErrors); ok {
		if len(ve) > 0 {
			e := ve[0]
			field := h.getJsonTag(e.Field())
			switch e.Tag() {
			case "required":
				return fmt.Sprintf("%s is required", field)
			case "url":
				return fmt.Sprintf("%s must be a valid URL", field)
			case "min":
				return fmt.Sprintf("%s is too short", field)
			case "max":
				return fmt.Sprintf("%s is too long", field)
			default:
				return fmt.Sprintf("%s is invalid", field)
			}
		}
	}
	return "Invalid request format: " + err.Error()
}

func (h *JobAPIHandler) getJsonTag(fieldName string) string {
	t := reflect.TypeFor[apimodels.CreateJobRequest]()
	field, found := t.FieldByName(fieldName)
	if !found {
		return fieldName
	}
	tag := field.Tag.Get("json")
	if tag == "" || tag == "-" {
		return fieldName
	}
	// Handle cases like "title,omitempty"
	return strings.Split(tag, ",")[0]
}
