package settings

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/benidevo/vega/internal/ai"
	aimodels "github.com/benidevo/vega/internal/ai/models"
	authmodels "github.com/benidevo/vega/internal/auth/models"
	"github.com/benidevo/vega/internal/common/alerts"
	ctxutil "github.com/benidevo/vega/internal/common/context"
	"github.com/benidevo/vega/internal/common/render"
	"github.com/benidevo/vega/internal/settings/models"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// SettingsHandler manages settings-related HTTP requests
type SettingsHandler struct {
	service      *SettingsService
	aiService    *ai.AIService
	quotaService interface {
		GetAllQuotaStatus(ctx context.Context, userID int) (interface{}, error)
	}
	experienceHandler    *BaseSettingsHandler
	educationHandler     *BaseSettingsHandler
	certificationHandler *BaseSettingsHandler
	renderer             *render.HTMLRenderer
}

// NewSettingsHandler creates a new SettingsHandler instance
func NewSettingsHandler(service *SettingsService, aiService *ai.AIService, quotaService interface {
	GetAllQuotaStatus(ctx context.Context, userID int) (interface{}, error)
}) *SettingsHandler {
	experienceMetadata := EntityMetadata{
		Name:      "Experience",
		URLPrefix: "experience",
		CreateFunc: func() CRUDEntity {
			return &models.WorkExperience{}
		},
	}

	educationMetadata := EntityMetadata{
		Name:      "Education",
		URLPrefix: "education",
		CreateFunc: func() CRUDEntity {
			return &models.Education{}
		},
	}

	certificationMetadata := EntityMetadata{
		Name:      "Certification",
		URLPrefix: "certification",
		CreateFunc: func() CRUDEntity {
			return &models.Certification{}
		},
	}

	return &SettingsHandler{
		service:              service,
		aiService:            aiService,
		quotaService:         quotaService,
		experienceHandler:    NewBaseSettingsHandler(service, experienceMetadata),
		educationHandler:     NewBaseSettingsHandler(service, educationMetadata),
		certificationHandler: NewBaseSettingsHandler(service, certificationMetadata),
		renderer:             render.NewHTMLRenderer(service.cfg),
	}
}

// formatValidationError formats validation errors into user-friendly messages
func (h *SettingsHandler) formatValidationError(err error) string {
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		for _, e := range validationErrors {
			field := e.Field()
			tag := e.Tag()

			switch field {
			case "FirstName":
				if tag == "required" {
					return "First name is required"
				}
				if tag == "max" {
					return "First name must not exceed 100 characters"
				}
			case "LastName":
				if tag == "required" {
					return "Last name is required"
				}
				if tag == "max" {
					return "Last name must not exceed 100 characters"
				}
			case "PhoneNumber":
				if tag == "phone" {
					return "Phone number contains invalid characters or must be between 10-20 characters"
				}
			case "LinkedInProfile":
				if tag == "linkedin" {
					return "LinkedIn profile must be a valid LinkedIn URL"
				}
				if tag == "url" {
					return "LinkedIn profile must be a valid URL"
				}
			case "GitHubProfile":
				if tag == "github" {
					return "GitHub profile must be a valid GitHub URL"
				}
				if tag == "url" {
					return "GitHub profile must be a valid URL"
				}
			case "Website":
				if tag == "url" {
					return "Website must be a valid URL"
				}
			case "Skills":
				if tag == "max" {
					return "Skills must not exceed 50 items"
				}
				if tag == "dive" {
					return "Each skill must be between 1-100 characters"
				}
			default:
				switch tag {
				case "required":
					return field + " is required"
				case "min":
					return field + " is too short"
				case "max":
					return field + " is too long"
				case "url":
					return field + " must be a valid URL"
				default:
					return "Invalid " + strings.ToLower(field)
				}
			}
		}
	}
	return err.Error()
}

// GetProfileSettingsPage handles the request to display the profile settings page
func (h *SettingsHandler) GetProfileSettingsPage(c *gin.Context) {
	userIDValue, exists := c.Get("userID")
	if !exists {
		h.renderer.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}
	userID := userIDValue.(int)

	profile, err := h.service.GetProfileWithRelated(c.Request.Context(), userID)

	if err != nil {
		h.renderer.HTML(c, http.StatusInternalServerError, "layouts/base.html", gin.H{
			"title": "Something Went Wrong",
			"page":  "500",
		})
		return
	}

	if profile == nil {
		h.renderer.HTML(c, http.StatusInternalServerError, "layouts/base.html", gin.H{
			"title": "Something Went Wrong",
			"page":  "500",
		})
		return
	}

	h.renderer.HTML(c, http.StatusOK, "layouts/base.html", gin.H{
		"title":          "Profile",
		"page":           "settings-profile",
		"activeNav":      "profile",
		"activeSettings": "profile",
		"pageTitle":      "Profile",
		"profile":        profile,
		"industries":     models.GetAllIndustries(),
	})
}

// HandleCreateProfile handles the creation or update of a user's profile settings
func (h *SettingsHandler) HandleCreateProfile(c *gin.Context) {
	userIDValue, exists := c.Get("userID")
	if !exists {
		alerts.RenderError(c, http.StatusUnauthorized, "Unauthorized", alerts.ContextDashboard)
		return
	}
	userID := userIDValue.(int)

	firstName := strings.TrimSpace(c.PostForm("first_name"))
	lastName := strings.TrimSpace(c.PostForm("last_name"))
	title := strings.TrimSpace(c.PostForm("title"))
	industryStr := strings.TrimSpace(c.PostForm("industry"))
	location := strings.TrimSpace(c.PostForm("location"))
	phoneNumber := strings.TrimSpace(c.PostForm("phone_number"))
	email := strings.TrimSpace(c.PostForm("email"))
	careerSummary := strings.TrimSpace(c.PostForm("career_summary"))
	skillsStr := strings.TrimSpace(c.PostForm("skills"))

	var skills []string
	if skillsStr != "" {
		skillsParts := strings.Split(skillsStr, ",")
		for _, s := range skillsParts {
			skill := strings.TrimSpace(s)
			if skill != "" {
				skills = append(skills, skill)
			}
		}
	}

	industry := models.IndustryFromString(industryStr)

	profile, err := h.service.GetProfileSettings(c.Request.Context(), userID)
	if err != nil {
		alerts.RenderError(c, http.StatusInternalServerError, "Failed to load profile settings", alerts.ContextDashboard)
		return
	}

	profile.FirstName = firstName
	profile.LastName = lastName
	profile.Title = title
	profile.Industry = industry
	profile.Location = location
	profile.PhoneNumber = phoneNumber
	profile.Email = email
	profile.CareerSummary = careerSummary
	profile.Skills = skills

	err = h.service.UpdateProfile(c.Request.Context(), profile)
	if err != nil {
		// Check if it's a validation error
		if _, ok := err.(validator.ValidationErrors); ok {
			errorMessage := h.formatValidationError(err)
			alerts.RenderError(c, http.StatusBadRequest, errorMessage, alerts.ContextDashboard)
			return
		}
		alerts.RenderError(c, http.StatusInternalServerError, "Failed to update profile: "+err.Error(), alerts.ContextDashboard)
		return
	}

	alerts.TriggerToast(c, "Personal information updated successfully", alerts.TypeSuccess)
	c.Status(http.StatusOK)
}

// HandleUpdateOnlineProfile handles the HTTP request to update a user's online profile links
func (h *SettingsHandler) HandleUpdateOnlineProfile(c *gin.Context) {
	userID := c.GetInt("userID")

	linkedInProfile := strings.TrimSpace(c.PostForm("linkedin_profile"))
	gitHubProfile := strings.TrimSpace(c.PostForm("github_profile"))
	website := strings.TrimSpace(c.PostForm("website"))

	profile, err := h.service.GetProfileSettings(c.Request.Context(), userID)
	if err != nil {
		alerts.RenderError(c, http.StatusInternalServerError, "Failed to load profile settings", alerts.ContextDashboard)
		return
	}

	profile.LinkedInProfile = linkedInProfile
	profile.GitHubProfile = gitHubProfile
	profile.Website = website

	err = h.service.UpdateProfile(c.Request.Context(), profile)
	if err != nil {
		alerts.RenderError(c, http.StatusInternalServerError, "Failed to update online profiles", alerts.ContextDashboard)
		return
	}

	alerts.TriggerToast(c, "Online profiles updated successfully", alerts.TypeSuccess)
	c.Status(http.StatusOK)
}

// HandleUpdateContext handles the HTTP request to update a user's personal context
func (h *SettingsHandler) HandleUpdateContext(c *gin.Context) {
	userID := c.GetInt("userID")

	context := strings.TrimSpace(c.PostForm("context"))

	if err := h.service.ValidateContext(context); err != nil {
		alerts.RenderError(c, http.StatusBadRequest, err.Error(), alerts.ContextDashboard)
		return
	}

	profile, err := h.service.GetProfileSettings(c.Request.Context(), userID)
	if err != nil {
		alerts.RenderError(c, http.StatusInternalServerError, "Failed to load profile settings", alerts.ContextDashboard)
		return
	}

	profile.Context = context

	err = h.service.UpdateProfile(c.Request.Context(), profile)
	if err != nil {
		alerts.TriggerToast(c, "Failed to update personal context", alerts.TypeError)
		c.Status(http.StatusBadRequest)
		return
	}

	alerts.TriggerToast(c, "Personal context updated successfully", alerts.TypeSuccess)
	c.Status(http.StatusOK)
}

// HandleCVUpload handles the HTTP request to parse and save CV data
func (h *SettingsHandler) HandleCVUpload(c *gin.Context) {
	userID := c.GetInt("userID")

	var requestData struct {
		CVText string `json:"cv_text"`
	}

	if err := c.ShouldBindJSON(&requestData); err != nil {
		h.service.log.Error().Err(err).Msg("Failed to parse CV upload request")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request format",
		})
		return
	}

	if requestData.CVText == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "CV text is required",
		})
		return
	}

	if h.aiService == nil {
		h.service.log.Error().Msg("AI service not available for CV parsing")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"message": "CV parsing service is currently unavailable",
		})
		return
	}

	cvResult, err := h.aiService.CVParser.ParseCV(c.Request.Context(), requestData.CVText)
	if err != nil {
		h.service.log.Error().Err(err).Msg("Failed to parse CV with AI")

		if strings.Contains(err.Error(), "invalid document:") {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid document format",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to parse CV content",
			})
		}
		return
	}

	profile, err := h.service.GetProfileSettings(c.Request.Context(), userID)
	if err != nil {
		profile = &models.Profile{UserID: userID}
	}

	if cvResult.PersonalInfo.FirstName != "" {
		profile.FirstName = cvResult.PersonalInfo.FirstName
	}
	if cvResult.PersonalInfo.LastName != "" {
		profile.LastName = cvResult.PersonalInfo.LastName
	}
	if cvResult.PersonalInfo.Title != "" {
		profile.Title = cvResult.PersonalInfo.Title
	}
	if cvResult.PersonalInfo.Phone != "" {
		profile.PhoneNumber = cvResult.PersonalInfo.Phone
	}
	if cvResult.PersonalInfo.Location != "" {
		profile.Location = cvResult.PersonalInfo.Location
	}
	if len(cvResult.Skills) > 0 {
		profile.Skills = cvResult.Skills
	}

	// Save profile
	if err := h.service.UpdateProfile(c.Request.Context(), profile); err != nil {
		h.service.log.Error().Err(err).Msg("Failed to save parsed CV data")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update profile",
		})
		return
	}

	if len(cvResult.WorkExperience) > 0 {
		if err := h.service.DeleteAllWorkExperience(c.Request.Context(), profile.ID); err != nil {
			h.service.log.Error().Err(err).Msg("Failed to clear existing work experience")
		}

		for i, aiExp := range cvResult.WorkExperience {
			exp := h.convertAIWorkExperienceToModel(aiExp, profile.ID)
			if err := h.service.CreateEntity(c, &exp); err != nil {
				h.service.log.Error().Err(err).
					Int("experience_index", i).
					Str("company", exp.Company).
					Str("title", exp.Title).
					Msg("Failed to save work experience from AI-parsed CV")
			} else {
				h.service.log.Info().
					Str("company", exp.Company).
					Str("title", exp.Title).
					Msg("Successfully saved work experience from AI-parsed CV")
			}
		}
	}

	if len(cvResult.Education) > 0 {
		if err := h.service.DeleteAllEducation(c.Request.Context(), profile.ID); err != nil {
			h.service.log.Error().Err(err).Msg("Failed to clear existing education")
		}

		for i, aiEdu := range cvResult.Education {
			edu := h.convertAIEducationToModel(aiEdu, profile.ID)
			if err := h.service.CreateEntity(c, &edu); err != nil {
				h.service.log.Error().Err(err).
					Int("education_index", i).
					Str("institution", edu.Institution).
					Str("degree", edu.Degree).
					Msg("Failed to save education from AI-parsed CV")
			} else {
				h.service.log.Info().
					Str("institution", edu.Institution).
					Str("degree", edu.Degree).
					Msg("Successfully saved education from AI-parsed CV")
			}
		}
	}

	if len(cvResult.Certifications) > 0 {
		if err := h.service.DeleteAllCertifications(c.Request.Context(), profile.ID); err != nil {
			h.service.log.Error().Err(err).Msg("Failed to clear existing certifications")
		}

		for i, aiCert := range cvResult.Certifications {
			cert := h.convertAICertificationToModel(aiCert, profile.ID)
			if err := h.service.CreateEntity(c, &cert); err != nil {
				h.service.log.Error().Err(err).
					Int("certification_index", i).
					Str("name", cert.Name).
					Str("issuing_org", cert.IssuingOrg).
					Msg("Failed to save certification from AI-parsed CV")
			} else {
				h.service.log.Info().
					Str("name", cert.Name).
					Str("issuing_org", cert.IssuingOrg).
					Msg("Successfully saved certification from AI-parsed CV")
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "CV parsed and profile updated successfully",
	})
}

// convertAIWorkExperienceToModel converts AI-parsed work experience to database model
func (h *SettingsHandler) convertAIWorkExperienceToModel(aiExp aimodels.WorkExperience, profileID int) models.WorkExperience {
	startDate := h.parseAIDate(aiExp.StartDate)
	var endDate *time.Time
	current := false

	if aiExp.EndDate != "" && strings.ToLower(aiExp.EndDate) != "present" {
		endDateTime := h.parseAIDate(aiExp.EndDate)
		endDate = &endDateTime
	} else if strings.ToLower(aiExp.EndDate) == "present" {
		current = true
	}

	return models.WorkExperience{
		ProfileID:   profileID,
		Company:     aiExp.Company,
		Title:       aiExp.Title,
		Location:    aiExp.Location,
		StartDate:   startDate,
		EndDate:     endDate,
		Description: aiExp.Description,
		Current:     current,
	}
}

// convertAIEducationToModel converts AI-parsed education to database model
func (h *SettingsHandler) convertAIEducationToModel(aiEdu aimodels.Education, profileID int) models.Education {
	startDate := h.parseAIDate(aiEdu.StartDate)
	var endDate *time.Time

	if aiEdu.EndDate != "" {
		endDateTime := h.parseAIDate(aiEdu.EndDate)
		endDate = &endDateTime
	}

	return models.Education{
		ProfileID:    profileID,
		Institution:  aiEdu.Institution,
		Degree:       aiEdu.Degree,
		FieldOfStudy: aiEdu.FieldOfStudy,
		StartDate:    startDate,
		EndDate:      endDate,
	}
}

// convertAICertificationToModel converts AI-parsed certification to database model
func (h *SettingsHandler) convertAICertificationToModel(aiCert aimodels.Certification, profileID int) models.Certification {
	issueDate := h.parseAIDate(aiCert.IssueDate)

	var expiryDate *time.Time
	if aiCert.ExpiryDate != "" {
		expiryDateTime := h.parseAIDate(aiCert.ExpiryDate)
		expiryDate = &expiryDateTime
	}

	return models.Certification{
		ProfileID:     profileID,
		Name:          aiCert.Name,
		IssuingOrg:    aiCert.IssuingOrg,
		IssueDate:     issueDate,
		ExpiryDate:    expiryDate,
		CredentialID:  aiCert.CredentialID,
		CredentialURL: aiCert.CredentialURL,
	}
}

// parseAIDate parses AI-formatted dates (YYYY-MM or YYYY) to time.Time
func (h *SettingsHandler) parseAIDate(dateStr string) time.Time {
	if dateStr == "" {
		return time.Now().AddDate(-4, 0, 0) // Default to 4 years ago
	}

	// Try YYYY-MM format first
	if t, err := time.Parse("2006-01", dateStr); err == nil {
		return t
	}

	// Try YYYY format
	if t, err := time.Parse("2006", dateStr); err == nil {
		return t
	}

	// Default fallback
	return time.Now().AddDate(-4, 0, 0)
}

// GetAccountSettingsPage handles the request to display the account settings page
func (h *SettingsHandler) GetAccountSettingsPage(c *gin.Context) {
	username, _ := c.Get("username")
	userIDValue, _ := c.Get("userID")
	userID := userIDValue.(int)

	user, err := h.service.userRepo.FindByID(c.Request.Context(), userID)
	if err != nil {
		h.renderer.HTML(c, http.StatusInternalServerError, "layouts/base.html", gin.H{
			"title": "Something Went Wrong",
			"page":  "500",
		})
		return
	}

	data := gin.H{
		"title":          "Account",
		"page":           "settings-account",
		"activeNav":      "account",
		"activeSettings": "account",
		"pageTitle":      "Account",
		"user":           user,
		"isCloudMode":    h.service.cfg.IsCloudMode,
	}

	// For cloud mode, get security settings (Google account info) and check if extension password is set
	if h.service.cfg.IsCloudMode {
		security, err := h.service.GetSecuritySettings(c.Request.Context(), username.(string))
		if err == nil {
			data["security"] = security
		}
		data["hasExtensionPassword"] = user.Password != ""
	}

	h.renderer.HTML(c, http.StatusOK, "layouts/base.html", data)
}

// GetAddExperiencePage handles the HTTP request to render the page for adding a new experience.
func (h *SettingsHandler) GetAddExperiencePage(c *gin.Context) {
	h.experienceHandler.GetAddPage(c)
}

// GetEditExperiencePage handles the HTTP request to render the edit experience page.
func (h *SettingsHandler) GetEditExperiencePage(c *gin.Context) {
	h.experienceHandler.GetEditPage(c)
}

// HandleExperienceForm handles the HTTP request for creating a new experience entry.
func (h *SettingsHandler) HandleExperienceForm(c *gin.Context) {
	h.experienceHandler.HandleCreate(c)
}

// HandleUpdateExperienceForm handles the HTTP request for updating an experience form.
func (h *SettingsHandler) HandleUpdateExperienceForm(c *gin.Context) {
	h.experienceHandler.HandleUpdate(c)
}

// HandleDeleteWorkExperience handles the HTTP request to delete a work experience entry.
func (h *SettingsHandler) HandleDeleteWorkExperience(c *gin.Context) {
	h.experienceHandler.HandleDelete(c)
}

// GetAddEducationPage handles the HTTP request to display the page for adding a new education entry.
func (h *SettingsHandler) GetAddEducationPage(c *gin.Context) {
	h.educationHandler.GetAddPage(c)
}

// GetEditEducationPage handles the HTTP request to render the edit education page.
func (h *SettingsHandler) GetEditEducationPage(c *gin.Context) {
	h.educationHandler.GetEditPage(c)
}

// CreateEducationForm handles the HTTP request for creating a new education form.
func (h *SettingsHandler) CreateEducationForm(c *gin.Context) {
	h.educationHandler.HandleCreate(c)
}

// HandleUpdateEducationForm handles the HTTP request for updating an education form.
func (h *SettingsHandler) HandleUpdateEducationForm(c *gin.Context) {
	h.educationHandler.HandleUpdate(c)
}

// HandleDeleteEducation handles HTTP DELETE requests for deleting an education record.
func (h *SettingsHandler) HandleDeleteEducation(c *gin.Context) {
	h.educationHandler.HandleDelete(c)
}

// GetAddCertificationPage handles the HTTP request to render the page for adding a new certification.
func (h *SettingsHandler) GetAddCertificationPage(c *gin.Context) {
	h.certificationHandler.GetAddPage(c)
}

// GetEditCertificationPage handles the HTTP request to render the edit certification page.
func (h *SettingsHandler) GetEditCertificationPage(c *gin.Context) {
	h.certificationHandler.GetEditPage(c)
}

// CreateCertificationForm handles the HTTP request for creating a new certification form.
func (h *SettingsHandler) CreateCertificationForm(c *gin.Context) {
	h.certificationHandler.HandleCreate(c)
}

// HandleUpdateCertificationForm handles the HTTP request for updating a certification form.
func (h *SettingsHandler) HandleUpdateCertificationForm(c *gin.Context) {
	h.certificationHandler.HandleUpdate(c)
}

// HandleDeleteCertification handles HTTP DELETE requests for removing a certification.
func (h *SettingsHandler) HandleDeleteCertification(c *gin.Context) {
	h.certificationHandler.HandleDelete(c)
}

// Quota Page Handler

// GetQuotasPage displays the quotas page with usage statistics
func (h *SettingsHandler) GetQuotasPage(c *gin.Context) {
	userIDValue, exists := c.Get("userID")
	if !exists {
		h.renderer.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}
	userID := userIDValue.(int)

	// Get quota status
	var quotaStatus interface{}
	var hasQuotaData bool
	if h.quotaService != nil {
		// Create context with role for quota checking
		ctx := c.Request.Context()
		if roleValue, exists := c.Get("role"); exists {
			if role, ok := roleValue.(string); ok {
				ctx = ctxutil.WithRole(ctx, role)
			}
		}
		quotaStatus, _ = h.quotaService.GetAllQuotaStatus(ctx, userID)
		hasQuotaData = quotaStatus != nil
	}

	data := gin.H{
		"title":          "Usage & Quotas",
		"activeNav":      "quotas",
		"page":           "settings-quotas",
		"activeSettings": "quotas",
		"pageTitle":      "Usage & Quotas",
		"quotaStatus":    quotaStatus,
		"hasQuotaData":   hasQuotaData,
		"isCloudMode":    h.service.cfg.IsCloudMode,
	}

	h.renderer.HTML(c, http.StatusOK, "layouts/base.html", data)
}

// HandleUpdateAccount handles updating username and/or password for self-hosted users
func (h *SettingsHandler) HandleUpdateAccount(c *gin.Context) {
	if h.service.cfg.IsCloudMode {
		alerts.TriggerToast(c, "Account management is not available in cloud mode", alerts.TypeError)
		c.Status(http.StatusForbidden)
		return
	}

	userIDValue, exists := c.Get("userID")
	if !exists {
		alerts.TriggerToast(c, "Unauthorized", alerts.TypeError)
		c.Status(http.StatusUnauthorized)
		return
	}
	userID := userIDValue.(int)

	// Get form data
	currentPassword := c.PostForm("current_password")
	newUsername := strings.TrimSpace(c.PostForm("new_username"))
	newPassword := c.PostForm("new_password")
	confirmPassword := c.PostForm("confirm_password")

	// Validate current password is provided
	if currentPassword == "" {
		alerts.TriggerToast(c, "Current password is required", alerts.TypeError)
		c.Status(http.StatusBadRequest)
		return
	}

	user, err := h.service.userRepo.FindByID(c.Request.Context(), userID)
	if err != nil {
		alerts.TriggerToast(c, "Failed to retrieve user information", alerts.TypeError)
		c.Status(http.StatusInternalServerError)
		return
	}

	if !h.service.authService.VerifyPassword(user.Password, currentPassword) {
		alerts.TriggerToast(c, "Current password is incorrect", alerts.TypeError)
		c.Status(http.StatusUnauthorized)
		return
	}

	usernameUpdated := false
	passwordUpdated := false

	if newUsername != "" && newUsername != user.Username {
		if err := authmodels.ValidateUsername(newUsername); err != nil {
			alerts.TriggerToast(c, err.Error(), alerts.TypeError)
			c.Status(http.StatusBadRequest)
			return
		}

		// Check if new username already exists
		existingUser, _ := h.service.userRepo.FindByUsername(c.Request.Context(), newUsername)
		if existingUser != nil {
			alerts.TriggerToast(c, "Username already exists", alerts.TypeError)
			c.Status(http.StatusBadRequest)
			return
		}
		user.Username = newUsername
		usernameUpdated = true
	}

	// Update password if provided
	if newPassword != "" {
		if newPassword != confirmPassword {
			alerts.TriggerToast(c, "New passwords do not match", alerts.TypeError)
			c.Status(http.StatusBadRequest)
			return
		}

		// Validate password strength (minimum 8 characters)
		if len(newPassword) < 8 {
			alerts.TriggerToast(c, "Password must be at least 8 characters long", alerts.TypeError)
			c.Status(http.StatusBadRequest)
			return
		}

		err = h.service.authService.ChangePassword(c.Request.Context(), user.ID, newPassword)
		if err != nil {
			alerts.TriggerToast(c, "Failed to update password", alerts.TypeError)
			c.Status(http.StatusInternalServerError)
			return
		}
		passwordUpdated = true
	}

	if usernameUpdated {
		_, err = h.service.userRepo.UpdateUser(c.Request.Context(), user)
		if err != nil {
			alerts.TriggerToast(c, "Failed to update username", alerts.TypeError)
			c.Status(http.StatusInternalServerError)
			return
		}
	}

	if usernameUpdated || passwordUpdated {
		alerts.TriggerToast(c, "Account updated successfully", alerts.TypeSuccess)
		c.Status(http.StatusOK)
	} else {
		alerts.TriggerToast(c, "No changes were made", alerts.TypeInfo)
		c.Status(http.StatusOK)
	}
}

// HandleUpdateExtensionPassword handles updating the browser extension password for cloud users
func (h *SettingsHandler) HandleUpdateExtensionPassword(c *gin.Context) {
	if !h.service.cfg.IsCloudMode {
		alerts.TriggerToast(c, "Extension password is only available in cloud mode", alerts.TypeError)
		c.Status(http.StatusForbidden)
		return
	}

	userIDValue, exists := c.Get("userID")
	if !exists {
		alerts.TriggerToast(c, "Unauthorized", alerts.TypeError)
		c.Status(http.StatusUnauthorized)
		return
	}
	userID := userIDValue.(int)

	newPassword := c.PostForm("extension_password")
	confirmPassword := c.PostForm("confirm_extension_password")

	if newPassword != confirmPassword {
		alerts.TriggerToast(c, "Passwords do not match", alerts.TypeError)
		c.Status(http.StatusBadRequest)
		return
	}

	if len(newPassword) < 8 || len(newPassword) > 128 {
		alerts.TriggerToast(c, "Password must be between 8 and 128 characters", alerts.TypeError)
		c.Status(http.StatusBadRequest)
		return
	}

	err := h.service.authService.ChangePassword(c.Request.Context(), userID, newPassword)
	if err != nil {
		alerts.TriggerToast(c, "Failed to update extension password", alerts.TypeError)
		c.Status(http.StatusInternalServerError)
		return
	}

	alerts.TriggerToast(c, "Browser extension password updated successfully", alerts.TypeSuccess)
	c.Status(http.StatusOK)
}

// DeleteAccount handles the account deletion request
func (h *SettingsHandler) DeleteAccount(c *gin.Context) {
	userIDValue, _ := c.Get("userID")
	userID := userIDValue.(int)

	// Call auth service to delete account
	if err := h.service.authService.DeleteAccount(c.Request.Context(), userID); err != nil {
		alerts.TriggerToast(c, "Failed to delete account", alerts.TypeError)
		c.Status(http.StatusInternalServerError)
		return
	}

	// Clear auth cookies
	h.clearAuthCookies(c)

	// HTMX will handle the redirect - just send it
	c.Header("HX-Redirect", "/")
	c.Status(http.StatusOK)
}

// clearAuthCookies removes authentication cookies
func (h *SettingsHandler) clearAuthCookies(c *gin.Context) {
	// Set cookies with MaxAge -1 to delete them
	c.SetCookie("token", "", -1, "/", h.service.cfg.CookieDomain, h.service.cfg.CookieSecure, true)
	c.SetCookie("refresh_token", "", -1, "/", h.service.cfg.CookieDomain, h.service.cfg.CookieSecure, true)
}
