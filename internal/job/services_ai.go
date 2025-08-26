package job

import (
	"context"
	"fmt"
	"strings"
	"time"

	aimodels "github.com/benidevo/vega/internal/ai/models"
	"github.com/benidevo/vega/internal/job/models"
	settingsmodels "github.com/benidevo/vega/internal/settings/models"
)

// AnalyzeJobMatch performs AI-powered job matching analysis between a user profile and a job.
func (s *JobService) AnalyzeJobMatch(ctx context.Context, userID, jobID int) (*models.JobMatchAnalysis, error) {
	userRef := fmt.Sprintf("user_%d", userID)

	s.log.Debug().
		Str("user_ref", userRef).
		Int("job_id", jobID).
		Str("operation", "job_match_analysis").
		Msg("Starting job match analysis")

	if s.aiService == nil {
		s.log.Error().
			Str("user_ref", userRef).
			Int("job_id", jobID).
			Str("error_type", "ai_service_unavailable").
			Msg("AI service not available")
		return nil, models.ErrAIServiceUnavailable
	}

	if s.settingsService == nil {
		s.log.Error().
			Str("user_ref", userRef).
			Int("job_id", jobID).
			Str("error_type", "settings_service_unavailable").
			Msg("Settings service not available")
		return nil, models.ErrProfileServiceRequired
	}

	// Check quota if quota service is available
	if s.quotaService != nil {
		quotaResult, err := s.quotaService.CanAnalyzeJob(ctx, userID, jobID)
		if err != nil {
			s.log.Error().Err(err).
				Str("user_ref", userRef).
				Int("job_id", jobID).
				Str("error_type", "quota_check_failed").
				Msg("Failed to check quota")
			return nil, fmt.Errorf("failed to check quota: %w", err)
		}

		if !quotaResult.Allowed {
			s.log.Warn().
				Str("user_ref", userRef).
				Int("job_id", jobID).
				Str("reason", quotaResult.Reason).
				Int("used", quotaResult.Status.Used).
				Int("limit", quotaResult.Status.Limit).
				Str("error_type", "quota_exceeded").
				Msg("Job analysis quota exceeded")
			return nil, models.ErrQuotaExceeded
		}

		s.log.Debug().
			Str("user_ref", userRef).
			Int("job_id", jobID).
			Str("reason", quotaResult.Reason).
			Int("used", quotaResult.Status.Used).
			Int("limit", quotaResult.Status.Limit).
			Msg("Quota check passed")
	}

	job, err := s.GetJob(ctx, userID, jobID)
	if err != nil {
		s.log.Error().Err(err).
			Str("user_ref", userRef).
			Int("job_id", jobID).
			Str("error_type", "job_not_found").
			Msg("Job not found for match analysis")
		return nil, err
	}

	profile, err := s.settingsService.GetProfileSettings(ctx, userID)
	if err != nil {
		s.log.Error().Err(err).
			Str("user_ref", userRef).
			Int("job_id", jobID).
			Str("error_type", "profile_fetch_failed").
			Msg("Failed to get user profile for match analysis")
		return nil, err
	}

	if err := s.ValidateProfileForAI(profile); err != nil {
		s.log.Warn().
			Str("user_ref", userRef).
			Int("job_id", jobID).
			Str("error_type", "profile_incomplete").
			Msg("Profile incomplete for AI analysis")
		return nil, err
	}

	aiRequest := s.buildAIRequest(job, profile)

	aiResult, err := s.aiService.JobMatcher.AnalyzeMatch(ctx, aiRequest)
	if err != nil {
		s.log.Error().Err(err).
			Str("user_ref", userRef).
			Int("job_id", jobID).
			Str("error_type", "ai_analysis_failed").
			Msg("Job match analysis failed")
		return nil, err
	}

	result := s.convertToJobMatchAnalysis(aiResult, userID, jobID)

	matchResult := &models.MatchResult{
		JobID:      jobID,
		MatchScore: aiResult.MatchScore,
		Strengths:  aiResult.Strengths,
		Weaknesses: aiResult.Weaknesses,
		Highlights: aiResult.Highlights,
		Feedback:   aiResult.Feedback,
	}

	if err := s.jobRepo.CreateMatchResult(ctx, userID, matchResult); err != nil {
		s.log.Warn().Err(err).
			Str("user_ref", userRef).
			Int("job_id", jobID).
			Str("error_type", "match_result_store_failed").
			Msg("Failed to store match result history, but continuing with analysis")
	}

	err = s.jobRepo.UpdateMatchScore(ctx, userID, jobID, &result.MatchScore)
	if err != nil {
		s.log.Warn().Err(err).
			Str("user_ref", userRef).
			Int("job_id", jobID).
			Int("match_score", result.MatchScore).
			Str("error_type", "match_score_update_failed").
			Msg("Failed to update job match score, but analysis completed")
		// Don't fail the entire operation if score update fails
	} else {
		s.log.Debug().
			Str("user_ref", userRef).
			Int("job_id", jobID).
			Int("match_score", result.MatchScore).
			Msg("Job match score updated successfully")
	}

	// Record the analysis in quota system if available
	if s.quotaService != nil && job.FirstAnalyzedAt == nil {
		if err := s.quotaService.RecordAnalysis(ctx, userID, jobID); err != nil {
			s.log.Warn().Err(err).
				Str("user_ref", userRef).
				Int("job_id", jobID).
				Str("error_type", "quota_record_failed").
				Msg("Failed to record analysis in quota system, but analysis completed")
		}
	}

	s.log.Info().
		Str("user_ref", userRef).
		Int("job_id", jobID).
		Int("match_score", result.MatchScore).
		Str("operation", "job_match_analysis").
		Bool("success", true).
		Msg("Job match analysis completed")

	return result, nil
}

// GenerateCoverLetter generates a cover letter for a specific job application.
func (s *JobService) GenerateCoverLetter(ctx context.Context, userID, jobID int) (*models.CoverLetterWithProfile, error) {
	userRef := fmt.Sprintf("user_%d", userID)

	s.log.Debug().
		Str("user_ref", userRef).
		Int("job_id", jobID).
		Str("operation", "cover_letter_generation").
		Msg("Starting cover letter generation")

	if s.aiService == nil {
		s.log.Error().
			Str("user_ref", userRef).
			Int("job_id", jobID).
			Str("error_type", "ai_service_unavailable").
			Msg("AI service not available")
		return nil, models.ErrAIServiceUnavailable
	}

	if s.settingsService == nil {
		s.log.Error().
			Str("user_ref", userRef).
			Int("job_id", jobID).
			Str("error_type", "settings_service_unavailable").
			Msg("Settings service not available")
		return nil, models.ErrProfileServiceRequired
	}

	job, err := s.GetJob(ctx, userID, jobID)
	if err != nil {
		s.log.Error().Err(err).
			Str("user_ref", userRef).
			Int("job_id", jobID).
			Str("error_type", "job_not_found").
			Msg("Job not found for cover letter generation")
		return nil, err
	}

	profile, err := s.settingsService.GetProfileSettings(ctx, userID)
	if err != nil {
		s.log.Error().Err(err).
			Str("user_ref", userRef).
			Int("job_id", jobID).
			Str("error_type", "profile_fetch_failed").
			Msg("Failed to get user profile for cover letter generation")
		return nil, err
	}

	if err := s.ValidateProfileForAI(profile); err != nil {
		s.log.Warn().
			Str("user_ref", userRef).
			Int("job_id", jobID).
			Str("error_type", "profile_incomplete").
			Msg("Profile incomplete for AI cover letter generation")
		return nil, err
	}

	aiRequest := s.buildAIRequest(job, profile)
	aiResult, err := s.aiService.CoverLetterGenerator.GenerateCoverLetter(ctx, aiRequest)
	if err != nil {
		s.log.Error().Err(err).
			Str("user_ref", userRef).
			Int("job_id", jobID).
			Str("error_type", "ai_generation_failed").
			Msg("Cover letter generation failed")
		return nil, err
	}

	coverLetter := s.convertToCoverLetter(aiResult, userID, jobID)

	personalInfo := &models.PersonalInfo{
		FirstName: profile.FirstName,
		LastName:  profile.LastName,
		Title:     profile.Title,
		Email:     profile.Email,
		Phone:     profile.PhoneNumber,
		Location:  profile.Location,
		LinkedIn:  profile.LinkedInProfile,
	}

	result := &models.CoverLetterWithProfile{
		CoverLetter:  coverLetter,
		PersonalInfo: personalInfo,
	}

	s.log.Info().
		Str("user_ref", userRef).
		Int("job_id", jobID).
		Str("operation", "cover_letter_generation").
		Bool("success", true).
		Msg("Cover letter generation completed")

	return result, nil
}

// buildAIRequest creates an AI request from job and profile data.
func (s *JobService) buildAIRequest(job *models.Job, profile *settingsmodels.Profile) aimodels.Request {
	applicantName := "Applicant"
	if profile.FirstName != "" || profile.LastName != "" {
		applicantName = fmt.Sprintf("%s %s", profile.FirstName, profile.LastName)
	}

	profileSummary := s.buildProfileSummary(profile)

	jobDescription := fmt.Sprintf("Position: %s\nCompany: %s\n\nDescription:\n%s",
		job.Title, job.Company.Name, job.Description)

	if len(job.RequiredSkills) > 0 {
		jobDescription += fmt.Sprintf("\n\nRequired Skills: %s", strings.Join(job.RequiredSkills, ", "))
	}

	totalYears := s.calculateTotalExperience(profile.WorkExperience)
	experienceContext := profile.Context
	if totalYears >= 2 {
		experienceContext = fmt.Sprintf("EXPERIENCED CANDIDATE (%.0f+ years): De-emphasize educational background in evaluation. Focus primarily on practical work experience, skills, and demonstrated achievements. Education should be secondary consideration. %s", totalYears, profile.Context)
	}

	return aimodels.Request{
		ApplicantName:    applicantName,
		ApplicantProfile: profileSummary,
		JobDescription:   jobDescription,
		ExtraContext:     experienceContext,

		WorkExperience:  profile.WorkExperience,
		Education:       profile.Education,
		Certifications:  profile.Certifications,
		Skills:          profile.Skills,
		YearsExperience: int(totalYears),
	}
}

// buildProfileSummary creates a comprehensive profile summary from user profile data.
func (s *JobService) buildProfileSummary(profile *settingsmodels.Profile) string {
	var summary strings.Builder

	if profile.FirstName != "" || profile.LastName != "" {
		summary.WriteString(fmt.Sprintf("Name: %s %s\n", profile.FirstName, profile.LastName))
	}

	if profile.Title != "" {
		summary.WriteString(fmt.Sprintf("Current Title: %s\n", profile.Title))
	}

	if profile.Industry.String() != "" {
		summary.WriteString(fmt.Sprintf("Industry: %s\n", profile.Industry.String()))
	}

	totalYears := s.calculateTotalExperience(profile.WorkExperience)
	if totalYears > 0 {
		summary.WriteString(fmt.Sprintf("Total Years of Experience: %.1f years\n", totalYears))
	}

	if profile.CareerSummary != "" {
		summary.WriteString(fmt.Sprintf("\nCareer Summary:\n%s\n", profile.CareerSummary))
	}

	validSkills := make([]string, 0, len(profile.Skills))
	for _, skill := range profile.Skills {
		if trimmed := strings.TrimSpace(skill); trimmed != "" {
			validSkills = append(validSkills, trimmed)
		}
	}
	if len(validSkills) > 0 {
		summary.WriteString(fmt.Sprintf("\nSkills: %s\n", strings.Join(validSkills, ", ")))
	}

	if len(profile.WorkExperience) > 0 {
		summary.WriteString("\nWork Experience:\n")
		for _, exp := range profile.WorkExperience {

			endDate := "Present"
			if exp.EndDate != nil {
				endDate = exp.EndDate.Format("January 2006")
			}

			location := ""
			if exp.Location != "" {
				location = fmt.Sprintf(", %s", exp.Location)
			}
			summary.WriteString(fmt.Sprintf("- %s at %s%s (%s - %s)\n",
				exp.Title, exp.Company, location, exp.StartDate.Format("January 2006"), endDate))

			if exp.Description != "" {
				desc := exp.Description
				summary.WriteString(fmt.Sprintf("  %s\n", desc))
			}
		}
	}

	validEducation := make([]settingsmodels.Education, 0, len(profile.Education))
	for _, edu := range profile.Education {
		if strings.TrimSpace(edu.Institution) != "" && strings.TrimSpace(edu.Degree) != "" {
			validEducation = append(validEducation, edu)
		}
	}
	if len(validEducation) > 0 {
		summary.WriteString("\nEducation:\n")
		for _, edu := range validEducation {

			fieldOfStudy := ""
			if strings.TrimSpace(edu.FieldOfStudy) != "" {
				fieldOfStudy = fmt.Sprintf(" in %s", edu.FieldOfStudy)
			}

			endDate := "Present"
			if edu.EndDate != nil {
				endDate = edu.EndDate.Format("January 2006")
			}

			summary.WriteString(fmt.Sprintf("- %s%s from %s (%s - %s)\n",
				edu.Degree, fieldOfStudy, edu.Institution, edu.StartDate.Format("January 2006"), endDate))
		}
	}

	if len(profile.Certifications) > 0 {
		summary.WriteString("\nCertifications:\n")
		for _, cert := range profile.Certifications {
			expiryInfo := ""
			if cert.ExpiryDate != nil {
				expiryInfo = fmt.Sprintf(" - %s", cert.ExpiryDate.Format("January 2006"))
			}

			summary.WriteString(fmt.Sprintf("- %s from %s (%s%s)\n",
				cert.Name, cert.IssuingOrg, cert.IssueDate.Format("January 2006"), expiryInfo))
		}
	}

	if profile.Context != "" {
		summary.WriteString(fmt.Sprintf("\nAdditional Context:\n%s\n", profile.Context))
	}

	return summary.String()
}

// calculateTotalExperience calculates the total years of work experience from work history
func (s *JobService) calculateTotalExperience(workExperience []settingsmodels.WorkExperience) float64 {
	if len(workExperience) == 0 {
		return 0
	}

	type TimePeriod struct {
		Start time.Time
		End   time.Time
	}

	var periods []TimePeriod
	now := time.Now()

	for _, exp := range workExperience {
		startDate := exp.StartDate
		endDate := now
		if exp.EndDate != nil {
			endDate = *exp.EndDate
		}

		if !startDate.IsZero() && endDate.After(startDate) {
			periods = append(periods, TimePeriod{
				Start: startDate,
				End:   endDate,
			})
		}
	}

	if len(periods) == 0 {
		return 0
	}

	for i := 0; i < len(periods); i++ {
		for j := i + 1; j < len(periods); j++ {
			if periods[i].Start.After(periods[j].Start) {
				periods[i], periods[j] = periods[j], periods[i]
			}
		}
	}

	var merged []TimePeriod
	current := periods[0]

	for i := 1; i < len(periods); i++ {
		period := periods[i]

		if current.End.After(period.Start) || current.End.Equal(period.Start) {
			if period.End.After(current.End) {
				current.End = period.End
			}
		} else {
			merged = append(merged, current)
			current = period
		}
	}

	merged = append(merged, current)

	var totalDays float64
	for _, period := range merged {
		duration := period.End.Sub(period.Start)
		totalDays += duration.Hours() / 24
	}

	totalYears := totalDays / 365.25
	return totalYears
}

// convertToJobMatchAnalysis converts AI match result to job domain model.
func (s *JobService) convertToJobMatchAnalysis(aiResult *aimodels.MatchResult, userID, jobID int) *models.JobMatchAnalysis {
	now := time.Now().UTC()

	return &models.JobMatchAnalysis{
		JobID:      jobID,
		UserID:     userID,
		MatchScore: aiResult.MatchScore,
		Strengths:  aiResult.Strengths,
		Weaknesses: aiResult.Weaknesses,
		Highlights: aiResult.Highlights,
		Feedback:   aiResult.Feedback,
		AnalyzedAt: now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// convertToCoverLetter converts AI cover letter result to job domain model.
func (s *JobService) convertToCoverLetter(aiResult *aimodels.CoverLetter, userID, jobID int) *models.CoverLetter {
	now := time.Now().UTC()

	return &models.CoverLetter{
		JobID:       jobID,
		UserID:      userID,
		Content:     aiResult.Content,
		Format:      string(aiResult.Format),
		GeneratedAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// ValidateProfileForAI validates that the user profile has sufficient data for AI operations.
func (s *JobService) ValidateProfileForAI(profile *settingsmodels.Profile) error {
	if profile.FirstName == "" && profile.LastName == "" {
		return models.ErrProfileIncomplete
	}

	hasCareerInfo := profile.CareerSummary != "" ||
		len(profile.WorkExperience) > 0 ||
		len(profile.Education) > 0

	if !hasCareerInfo {
		return models.ErrProfileSummaryRequired
	}

	if len(profile.WorkExperience) > 0 {
		hasDetailedExperience := false
		for _, exp := range profile.WorkExperience {
			if exp.Title != "" && exp.Company != "" {
				hasDetailedExperience = true
				break
			}
		}
		if !hasDetailedExperience && len(profile.Education) == 0 {
			return models.ErrProfileSummaryRequired
		}
	}

	return nil
}

// GenerateCV generates a CV for a specific job application.
func (s *JobService) GenerateCV(ctx context.Context, userID, jobID int) (*models.GeneratedCV, error) {
	userRef := fmt.Sprintf("user_%d", userID)

	s.log.Debug().
		Str("user_ref", userRef).
		Int("job_id", jobID).
		Str("operation", "cv_generation").
		Msg("Starting CV generation")

	if s.aiService == nil {
		s.log.Error().
			Str("user_ref", userRef).
			Int("job_id", jobID).
			Str("error_type", "ai_service_unavailable").
			Msg("AI service not available")
		return nil, models.ErrAIServiceUnavailable
	}

	if s.settingsService == nil {
		s.log.Error().
			Str("user_ref", userRef).
			Int("job_id", jobID).
			Str("error_type", "settings_service_unavailable").
			Msg("Settings service not available")
		return nil, models.ErrProfileServiceRequired
	}

	job, err := s.GetJob(ctx, userID, jobID)
	if err != nil {
		s.log.Error().Err(err).
			Str("user_ref", userRef).
			Int("job_id", jobID).
			Str("error_type", "job_not_found").
			Msg("Job not found for CV generation")
		return nil, err
	}

	profile, err := s.settingsService.GetProfileWithRelated(ctx, userID)
	if err != nil {
		s.log.Error().Err(err).
			Str("user_ref", userRef).
			Int("job_id", jobID).
			Str("error_type", "profile_fetch_failed").
			Msg("Failed to get user profile for CV generation")
		return nil, err
	}

	if err := s.ValidateProfileForAI(profile); err != nil {
		s.log.Warn().
			Str("user_ref", userRef).
			Int("job_id", jobID).
			Str("error_type", "profile_incomplete").
			Msg("Profile incomplete for AI CV generation")
		return nil, err
	}

	aiRequest := s.buildAIRequest(job, profile)
	aiRequest.CVText = s.buildProfileSummary(profile)

	aiResult, err := s.aiService.CVGenerator.GenerateCV(ctx, aiRequest, jobID, job.Title)
	if err != nil {
		s.log.Error().Err(err).
			Str("user_ref", userRef).
			Int("job_id", jobID).
			Str("error_type", "ai_generation_failed").
			Msg("CV generation failed")
		return nil, err
	}

	result := s.convertToGeneratedCV(aiResult, userID, jobID, profile)

	s.log.Info().
		Str("user_ref", userRef).
		Int("job_id", jobID).
		Str("operation", "cv_generation").
		Bool("success", true).
		Msg("CV generation completed")

	return result, nil
}

// convertToGeneratedCV converts AI CV result to job domain model.
func (s *JobService) convertToGeneratedCV(aiResult *aimodels.GeneratedCV, userID, jobID int, profile *settingsmodels.Profile) *models.GeneratedCV {
	now := time.Now().UTC()

	personalInfo := convertPersonalInfo(aiResult.PersonalInfo)

	if profile.PhoneNumber != "" {
		personalInfo.Phone = profile.PhoneNumber
	}
	if profile.Email != "" {
		personalInfo.Email = profile.Email
	}
	if profile.LinkedInProfile != "" {
		personalInfo.LinkedIn = profile.LinkedInProfile
	}
	if profile.Location != "" {
		personalInfo.Location = profile.Location
	}

	return &models.GeneratedCV{
		JobID:          jobID,
		UserID:         userID,
		IsValid:        aiResult.IsValid,
		Reason:         aiResult.Reason,
		PersonalInfo:   personalInfo,
		WorkExperience: convertWorkExperience(aiResult.WorkExperience),
		Education:      convertEducation(aiResult.Education),
		Certifications: convertCertifications(profile.Certifications),
		Skills:         aiResult.Skills,
		GeneratedAt:    time.Unix(aiResult.GeneratedAt, 0),
		JobTitle:       aiResult.JobTitle,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func convertPersonalInfo(ai aimodels.PersonalInfo) models.PersonalInfo {
	return models.PersonalInfo{
		FirstName: ai.FirstName,
		LastName:  ai.LastName,
		Email:     ai.Email,
		Phone:     ai.Phone,
		Location:  ai.Location,
		LinkedIn:  ai.LinkedIn,
		Title:     ai.Title,
		Summary:   ai.Summary,
	}
}

func convertWorkExperience(aiExps []aimodels.WorkExperience) []models.WorkExperience {
	exps := make([]models.WorkExperience, len(aiExps))
	for i, exp := range aiExps {
		exps[i] = models.WorkExperience{
			Company:     exp.Company,
			Title:       exp.Title,
			Location:    exp.Location,
			StartDate:   exp.StartDate,
			EndDate:     exp.EndDate,
			Description: exp.Description,
		}
	}
	return exps
}

func convertEducation(aiEdu []aimodels.Education) []models.Education {
	edu := make([]models.Education, len(aiEdu))
	for i, e := range aiEdu {
		edu[i] = models.Education{
			Institution:  e.Institution,
			Degree:       e.Degree,
			FieldOfStudy: e.FieldOfStudy,
			StartDate:    e.StartDate,
			EndDate:      e.EndDate,
		}
	}
	return edu
}

func convertCertifications(profileCerts []settingsmodels.Certification) []models.Certification {
	certs := make([]models.Certification, len(profileCerts))
	for i, c := range profileCerts {
		issueDate := c.IssueDate.Format("Jan 2006")
		expiryDate := ""
		if c.ExpiryDate != nil {
			expiryDate = c.ExpiryDate.Format("Jan 2006")
		}

		certs[i] = models.Certification{
			Name:          c.Name,
			IssuingOrg:    c.IssuingOrg,
			IssueDate:     issueDate,
			ExpiryDate:    expiryDate,
			CredentialID:  c.CredentialID,
			CredentialURL: c.CredentialURL,
		}
	}
	return certs
}

func (s *JobService) formatCVAsHTML(cv *models.GeneratedCV) string {
	var html strings.Builder
	html.WriteString("<div class='cv-content'>")

	// Personal Info
	html.WriteString("<section class='personal-info'>")
	fullName := fmt.Sprintf("%s %s", cv.PersonalInfo.FirstName, cv.PersonalInfo.LastName)
	html.WriteString(fmt.Sprintf("<h1>%s</h1>", fullName))
	html.WriteString(fmt.Sprintf("<p>%s | %s</p>", cv.PersonalInfo.Email, cv.PersonalInfo.Phone))
	html.WriteString("</section>")

	// Skills
	if len(cv.Skills) > 0 {
		html.WriteString("<section class='skills'>")
		html.WriteString("<h2>Skills</h2><p>")
		html.WriteString(strings.Join(cv.Skills, ", "))
		html.WriteString("</p></section>")
	}

	// Work Experience
	if len(cv.WorkExperience) > 0 {
		html.WriteString("<section class='experience'><h2>Work Experience</h2>")
		for _, exp := range cv.WorkExperience {
			html.WriteString(fmt.Sprintf("<div><h3>%s at %s</h3>", exp.Title, exp.Company))
			html.WriteString(fmt.Sprintf("<p>%s - %s</p>", exp.StartDate, exp.EndDate))
			html.WriteString(fmt.Sprintf("<p>%s</p></div>", exp.Description))
		}
		html.WriteString("</section>")
	}

	// Education
	if len(cv.Education) > 0 {
		html.WriteString("<section class='education'><h2>Education</h2>")
		for _, edu := range cv.Education {
			html.WriteString(fmt.Sprintf("<div><h3>%s</h3>", edu.Degree))
			html.WriteString(fmt.Sprintf("<p>%s | %s - %s</p></div>", edu.Institution, edu.StartDate, edu.EndDate))
		}
		html.WriteString("</section>")
	}

	// Certifications
	if len(cv.Certifications) > 0 {
		html.WriteString("<section class='certifications'><h2>Certifications</h2>")
		for _, cert := range cv.Certifications {
			html.WriteString(fmt.Sprintf("<div><h3>%s</h3>", cert.Name))
			if cert.IssuingOrg != "" {
				html.WriteString(fmt.Sprintf("<p>%s | %s</p></div>", cert.IssuingOrg, cert.IssueDate))
			} else {
				html.WriteString(fmt.Sprintf("<p>%s</p></div>", cert.IssueDate))
			}
		}
		html.WriteString("</section>")
	}

	html.WriteString("</div>")
	return html.String()
}
