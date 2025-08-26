package documents

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/benidevo/vega/internal/common/alerts"
	"github.com/benidevo/vega/internal/common/logger"
	"github.com/benidevo/vega/internal/common/render"
	"github.com/benidevo/vega/internal/config"
	"github.com/benidevo/vega/internal/documents/models"
	jobmodels "github.com/benidevo/vega/internal/job/models"
	"github.com/gin-gonic/gin"
)

type DocumentHandler struct {
	service  Service
	cfg      *config.Settings
	log      *logger.PrivacyLogger
	renderer *render.HTMLRenderer
}

func NewDocumentHandler(service Service, cfg *config.Settings, renderer *render.HTMLRenderer) *DocumentHandler {
	return &DocumentHandler{
		service:  service,
		cfg:      cfg,
		log:      logger.GetPrivacyLogger("documents_handler"),
		renderer: renderer,
	}
}

func (h *DocumentHandler) GetDocumentsHub(c *gin.Context) {
	userIDValue, exists := c.Get("userID")
	if !exists {
		alerts.RenderError(c, http.StatusUnauthorized, "Authentication required", alerts.ContextGeneral)
		return
	}
	userID := userIDValue.(int)

	tab := c.DefaultQuery("tab", "cover-letters")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize := 9

	metrics, err := h.service.GetDocumentMetrics(c.Request.Context(), userID)
	if err != nil {
		h.log.Error().Err(err).Msg("Failed to get document metrics")
		metrics = &models.DocumentMetrics{}
	}

	var documents []*models.DocumentSummary
	var total int

	if tab == "resumes" {
		documents, total, err = h.service.GetDocumentsByType(
			c.Request.Context(), userID, models.DocumentTypeResume, page, pageSize,
		)
	} else {
		tab = "cover-letters"
		documents, total, err = h.service.GetDocumentsByType(
			c.Request.Context(), userID, models.DocumentTypeCoverLetter, page, pageSize,
		)
	}

	if err != nil {
		h.log.Error().Err(err).Str("tab", tab).Msg("Failed to get documents")
		alerts.RenderError(c, http.StatusInternalServerError, "Failed to load documents", alerts.ContextGeneral)
		return
	}

	totalPages := (total + pageSize - 1) / pageSize

	data := gin.H{
		"page":             "documents",
		"activeNav":        "documents",
		"title":            "My Documents",
		"PageTitle":        "My Documents",
		"ActiveTab":        tab,
		"Documents":        documents,
		"TotalDocuments":   total,
		"CoverLetterCount": metrics.CoverLetterCount,
		"ResumeCount":      metrics.ResumeCount,
		"CurrentPage":      page,
		"TotalPages":       totalPages,
		"HasPrevPage":      page > 1,
		"HasNextPage":      page < totalPages,
		"PrevPage":         page - 1,
		"NextPage":         page + 1,
	}

	if c.GetHeader("HX-Request") == "true" && c.GetHeader("HX-Target") == "documents-content" {
		h.renderer.HTML(c, http.StatusOK, "documents/partials/document_list.html", data)
		return
	}
	h.renderer.HTML(c, http.StatusOK, "layouts/base.html", data)
}

func (h *DocumentHandler) DeleteDocument(c *gin.Context) {
	userIDValue, exists := c.Get("userID")
	if !exists {
		alerts.RenderError(c, http.StatusUnauthorized, "Authentication required", alerts.ContextGeneral)
		return
	}
	userID := userIDValue.(int)

	docIDStr := c.Param("id")
	docID, err := strconv.Atoi(docIDStr)
	if err != nil {
		alerts.RenderError(c, http.StatusBadRequest, "Invalid document ID", alerts.ContextGeneral)
		return
	}

	err = h.service.DeleteDocument(c.Request.Context(), docID, userID)
	if err != nil {
		if err == models.ErrDocumentNotFound {
			alerts.RenderError(c, http.StatusNotFound, "Document not found", alerts.ContextGeneral)
		} else {
			h.log.Error().Err(err).Int("doc_id", docID).Msg("Failed to delete document")
			alerts.RenderError(c, http.StatusInternalServerError, "Failed to delete document", alerts.ContextGeneral)
		}
		return
	}

	c.Header("HX-Trigger", "document-deleted")
	alerts.RenderSuccess(c, "Document deleted successfully", alerts.ContextGeneral)
}

func (h *DocumentHandler) ExportDocument(c *gin.Context) {
	userIDValue, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}
	userID := userIDValue.(int)

	docIDStr := c.Param("id")
	docID, err := strconv.Atoi(docIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid document ID"})
		return
	}

	doc, err := h.service.GetDocument(c.Request.Context(), docID, userID)
	if err != nil {
		if err == models.ErrDocumentNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Document not found"})
		} else {
			h.log.Error().Err(err).Int("doc_id", docID).Msg("Failed to get document for export")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to export document"})
		}
		return
	}

	var jobTitle, companyName string
	if doc.JobID > 0 {
		documents, _, err := h.service.GetDocumentsByType(
			c.Request.Context(),
			userID,
			doc.DocumentType,
			1,
			1,
		)
		if err == nil && len(documents) > 0 {
			for _, summary := range documents {
				if summary.JobID == doc.JobID {
					jobTitle = summary.JobTitle
					companyName = summary.CompanyName
					break
				}
			}
		}
	}

	responseData := gin.H{
		"content":     doc.Content,
		"type":        string(doc.DocumentType),
		"jobId":       doc.JobID,
		"jobTitle":    jobTitle,
		"companyName": companyName,
	}

	if doc.DocumentType == models.DocumentTypeCoverLetter {
		var coverLetterData struct {
			Content      string                  `json:"content"`
			PersonalInfo *jobmodels.PersonalInfo `json:"personalInfo,omitempty"`
		}

		if err := json.Unmarshal([]byte(doc.Content), &coverLetterData); err == nil && coverLetterData.PersonalInfo != nil {
			responseData["content"] = coverLetterData.Content
			responseData["personalInfo"] = gin.H{
				"firstName": coverLetterData.PersonalInfo.FirstName,
				"lastName":  coverLetterData.PersonalInfo.LastName,
				"title":     coverLetterData.PersonalInfo.Title,
				"email":     coverLetterData.PersonalInfo.Email,
				"phone":     coverLetterData.PersonalInfo.Phone,
				"location":  coverLetterData.PersonalInfo.Location,
				"linkedin":  coverLetterData.PersonalInfo.LinkedIn,
			}
		} else if doc.JobID > 0 {
			resumeDoc, err := h.service.GetDocumentByJobAndType(
				c.Request.Context(),
				userID,
				doc.JobID,
				models.DocumentTypeResume,
			)

			if err == nil && resumeDoc != nil {
				var cv jobmodels.GeneratedCV
				if err := json.Unmarshal([]byte(resumeDoc.Content), &cv); err == nil {
					responseData["personalInfo"] = gin.H{
						"firstName": cv.PersonalInfo.FirstName,
						"lastName":  cv.PersonalInfo.LastName,
						"title":     cv.PersonalInfo.Title,
						"email":     cv.PersonalInfo.Email,
						"phone":     cv.PersonalInfo.Phone,
						"location":  cv.PersonalInfo.Location,
						"linkedin":  cv.PersonalInfo.LinkedIn,
					}
				} else {
					h.log.Warn().Err(err).Msg("Failed to parse resume JSON for personal info")
				}
			} else if err != nil && err != models.ErrDocumentNotFound {
				h.log.Warn().Err(err).Int("job_id", doc.JobID).Msg("Failed to fetch resume for personal info")
			}
		}
	}

	c.JSON(http.StatusOK, responseData)
}

func (h *DocumentHandler) GetDocumentPartial(c *gin.Context) {
	userIDValue, exists := c.Get("userID")
	if !exists {
		alerts.RenderError(c, http.StatusUnauthorized, "Authentication required", alerts.ContextGeneral)
		return
	}
	userID := userIDValue.(int)

	tab := c.DefaultQuery("tab", "cover-letters")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize := 9

	var documents []*models.DocumentSummary
	var total int
	var err error

	if tab == "resumes" {
		documents, total, err = h.service.GetDocumentsByType(
			c.Request.Context(), userID, models.DocumentTypeResume, page, pageSize,
		)
	} else {
		tab = "cover-letters"
		documents, total, err = h.service.GetDocumentsByType(
			c.Request.Context(), userID, models.DocumentTypeCoverLetter, page, pageSize,
		)
	}

	if err != nil {
		h.log.Error().Err(err).Str("tab", tab).Msg("Failed to get documents")
		alerts.RenderError(c, http.StatusInternalServerError, "Failed to load documents", alerts.ContextGeneral)
		return
	}

	totalPages := (total + pageSize - 1) / pageSize

	data := gin.H{
		"ActiveTab":      tab,
		"Documents":      documents,
		"TotalDocuments": total,
		"CurrentPage":    page,
		"TotalPages":     totalPages,
		"HasPrevPage":    page > 1,
		"HasNextPage":    page < totalPages,
		"PrevPage":       page - 1,
		"NextPage":       page + 1,
	}

	h.renderer.HTML(c, http.StatusOK, "documents/partials/document_list.html", data)
}

func (h *DocumentHandler) formatCVAsHTML(cv *jobmodels.GeneratedCV) string {
	const resumeTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Resume - {{.FullName}}</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Helvetica', Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 800px;
            margin: 0 auto;
            padding: 40px 20px;
            background: white;
        }
        h1 { font-size: 28px; margin-bottom: 8px; color: #111; }
        h2 { 
            font-size: 16px; 
            margin-top: 24px; 
            margin-bottom: 12px; 
            padding-bottom: 4px;
            border-bottom: 2px solid #333;
            color: #111;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }
        h3 { font-size: 14px; margin-top: 16px; margin-bottom: 8px; color: #333; }
        .header-section { margin-bottom: 24px; }
        .contact-info { color: #666; font-size: 12px; margin-bottom: 16px; }
        .title { font-size: 14px; color: #444; margin-bottom: 8px; }
        .section { margin-bottom: 24px; }
        .experience-item, .education-item { margin-bottom: 16px; page-break-inside: avoid; }
        .date { color: #666; font-size: 11px; margin-bottom: 4px; }
        .description { font-size: 12px; line-height: 1.5; color: #444; }
        .skills { font-size: 12px; line-height: 1.8; }
        @media print {
            body { padding: 0; }
            h2 { page-break-after: avoid; }
            .experience-item, .education-item { page-break-inside: avoid; }
        }
    </style>
</head>
<body>
    <div class="header-section">
        <h1>{{.FullName}}</h1>
        {{if .PersonalInfo.Title}}
        <div class="title">{{.PersonalInfo.Title}}</div>
        {{end}}
        {{if .ContactInfo}}
        <div class="contact-info">{{.ContactInfo}}</div>
        {{end}}
    </div>

    {{if .PersonalInfo.Summary}}
    <div class="section">
        <h2>Professional Summary</h2>
        <p class="description">{{.PersonalInfo.Summary}}</p>
    </div>
    {{end}}

    {{if .Skills}}
    <div class="section">
        <h2>Skills</h2>
        <div class="skills">{{.SkillsString}}</div>
    </div>
    {{end}}

    {{if .WorkExperience}}
    <div class="section">
        <h2>Work Experience</h2>
        {{range .WorkExperience}}
        <div class="experience-item">
            <h3>{{.Title}} at {{.Company}}</h3>
            <div class="date">{{.StartDate}} - {{.EndDate}}{{if .Location}} | {{.Location}}{{end}}</div>
            <div class="description">{{.FormattedDescription}}</div>
        </div>
        {{end}}
    </div>
    {{end}}

    {{if .Education}}
    <div class="section">
        <h2>Education</h2>
        {{range .Education}}
        <div class="education-item">
            <h3>{{.Degree}}{{if .FieldOfStudy}} in {{.FieldOfStudy}}{{end}}</h3>
            <div>{{.Institution}}</div>
            <div class="date">{{.StartDate}} - {{.EndDate}}</div>
        </div>
        {{end}}
    </div>
    {{end}}

    {{if .Certifications}}
    <div class="section">
        <h2>Certifications</h2>
        {{range .Certifications}}
        <div style="margin-bottom: 8px;">
            <strong>{{.Name}}</strong>{{if .IssuingOrg}} - {{.IssuingOrg}}{{end}}{{if .IssueDate}} ({{.IssueDate}}){{end}}
        </div>
        {{end}}
    </div>
    {{end}}
</body>
</html>`

	type workExp struct {
		Title                string
		Company              string
		StartDate            string
		EndDate              string
		Location             string
		FormattedDescription template.HTML
	}

	type templateData struct {
		FullName       string
		PersonalInfo   *jobmodels.PersonalInfo
		ContactInfo    string
		Skills         []string
		SkillsString   string
		WorkExperience []workExp
		Education      []jobmodels.Education
		Certifications []jobmodels.Certification
	}

	var contactParts []string
	if cv.PersonalInfo.Location != "" {
		contactParts = append(contactParts, cv.PersonalInfo.Location)
	}
	if cv.PersonalInfo.Email != "" {
		contactParts = append(contactParts, cv.PersonalInfo.Email)
	}
	if cv.PersonalInfo.Phone != "" {
		contactParts = append(contactParts, cv.PersonalInfo.Phone)
	}
	if cv.PersonalInfo.LinkedIn != "" {
		contactParts = append(contactParts, cv.PersonalInfo.LinkedIn)
	}

	var workExperience []workExp
	for _, exp := range cv.WorkExperience {
		workExperience = append(workExperience, workExp{
			Title:                exp.Title,
			Company:              exp.Company,
			StartDate:            exp.StartDate,
			EndDate:              exp.EndDate,
			Location:             exp.Location,
			FormattedDescription: template.HTML(h.formatDescriptionSafe(exp.Description)),
		})
	}

	data := templateData{
		FullName:       fmt.Sprintf("%s %s", cv.PersonalInfo.FirstName, cv.PersonalInfo.LastName),
		PersonalInfo:   &cv.PersonalInfo,
		ContactInfo:    strings.Join(contactParts, " | "),
		Skills:         cv.Skills,
		SkillsString:   strings.Join(cv.Skills, " • "),
		WorkExperience: workExperience,
		Education:      cv.Education,
		Certifications: cv.Certifications,
	}

	tmpl, err := template.New("resume").Parse(resumeTemplate)
	if err != nil {
		h.log.Error().Err(err).Msg("Failed to parse resume template")
		return ""
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		h.log.Error().Err(err).Msg("Failed to execute resume template")
		return ""
	}

	return buf.String()
}

func (h *DocumentHandler) formatDescriptionSafe(text string) string {
	if text == "" {
		return ""
	}

	lines := strings.Split(text, "\n")
	var result strings.Builder
	inList := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "•") || strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "*") {
			if !inList {
				result.WriteString("<ul>")
				inList = true
			}
			var content string
			if strings.HasPrefix(trimmed, "•") {
				content = strings.TrimSpace(trimmed[len("•"):])
			} else {
				content = strings.TrimSpace(trimmed[1:])
			}
			escapedContent := template.HTMLEscapeString(content)
			result.WriteString(fmt.Sprintf("<li>%s</li>", escapedContent))
		} else {
			if inList {
				result.WriteString("</ul>")
				inList = false
			}
			if trimmed != "" {
				escapedContent := template.HTMLEscapeString(trimmed)
				result.WriteString(fmt.Sprintf("<p>%s</p>", escapedContent))
			}
		}
	}

	if inList {
		result.WriteString("</ul>")
	}

	formatted := result.String()
	if formatted == "" {
		return template.HTMLEscapeString(text)
	}
	return formatted
}

func (h *DocumentHandler) formatDescription(text string) string {
	return h.formatDescriptionSafe(text)
}

type SaveDocumentRequest struct {
	JobID        int    `json:"jobId" binding:"required"`
	DocumentType string `json:"documentType" binding:"required,oneof=resume cover_letter"`
	Content      string `json:"content" binding:"required"`
}

func (h *DocumentHandler) SaveDocument(c *gin.Context) {
	userIDValue, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}
	userID := userIDValue.(int)

	const maxBodySize = 2 * 1024 * 1024
	if c.Request.ContentLength > maxBodySize {
		h.log.Warn().
			Int64("content_length", c.Request.ContentLength).
			Int("user_id", userID).
			Msg("Request body exceeds maximum size")
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "Request body too large"})
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBodySize)

	var req SaveDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.Warn().Err(err).Msg("Invalid save document request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	var docType models.DocumentType
	if req.DocumentType == "resume" {
		docType = models.DocumentTypeResume
	} else if req.DocumentType == "cover_letter" {
		docType = models.DocumentTypeCoverLetter
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid document type"})
		return
	}

	if len(req.Content) > models.MaxDocumentSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Document content exceeds %dMB limit",
				models.MaxDocumentSize/(1024*1024)),
		})
		return
	}

	content := req.Content

	if docType == models.DocumentTypeResume {
		var cv jobmodels.GeneratedCV
		if err := json.Unmarshal([]byte(req.Content), &cv); err != nil {
			h.log.Error().Err(err).Msg("Failed to parse resume JSON")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid resume data"})
			return
		}

		if cv.PersonalInfo.FirstName == "" || cv.PersonalInfo.LastName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Resume must include first and last name"})
			return
		}

		cleanJSON, err := json.Marshal(cv)
		if err != nil {
			h.log.Error().Err(err).Msg("Failed to re-encode resume JSON")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process resume data"})
			return
		}
		content = string(cleanJSON)
	} else if docType == models.DocumentTypeCoverLetter {
		var coverLetterData map[string]interface{}
		if err := json.Unmarshal([]byte(req.Content), &coverLetterData); err != nil {
			wrappedContent := map[string]string{"content": req.Content}
			cleanJSON, err := json.Marshal(wrappedContent)
			if err != nil {
				h.log.Error().Err(err).Msg("Failed to marshal cover letter wrapper")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process cover letter data"})
				return
			}
			content = string(cleanJSON)
		} else {
			cleanJSON, err := json.Marshal(coverLetterData)
			if err != nil {
				h.log.Error().Err(err).Msg("Failed to re-encode cover letter JSON")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process cover letter data"})
				return
			}
			content = string(cleanJSON)
		}
	}

	doc, err := h.service.SaveGeneratedDocument(
		c.Request.Context(),
		userID,
		req.JobID,
		docType,
		content,
	)

	if err != nil {
		h.log.Error().
			Err(err).
			Int("user_id", userID).
			Int("job_id", req.JobID).
			Str("document_type", string(docType)).
			Msg("Failed to save document")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save document"})
		return
	}

	h.log.Info().
		Int("user_id", userID).
		Int("job_id", req.JobID).
		Int("document_id", doc.ID).
		Str("document_type", string(docType)).
		Msg("Document saved successfully")

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Document saved successfully",
		"documentId": doc.ID,
	})
}
