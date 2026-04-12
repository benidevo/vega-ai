package models

import (
	"encoding/json"
	"strings"
)

// MatchResult represents the result of a matching process, including the match score,
// identified strengths and weaknesses, key highlights, and overall feedback.
type MatchResult struct {
	MatchScore int      `json:"matchScore"`
	Strengths  []string `json:"strengths"`
	Weaknesses []string `json:"weaknesses"`
	Highlights []string `json:"highlights"`
	Feedback   string   `json:"feedback"`
}

// CoverLetterFormat defines the format type for a cover letter, such as HTML, Markdown, or plain text.
// It is used to specify how the cover letter content is structured and presented.
type CoverLetterFormat string

const (
	CoverLetterTypeHtml      CoverLetterFormat = "html"
	CoverLetterTypeMarkdown  CoverLetterFormat = "markdown"
	CoverLetterTypePlainText CoverLetterFormat = "plain_text"
)

// CoverLetter represents a cover letter with its format and content.
type CoverLetter struct {
	Format  CoverLetterFormat `json:"format"`
	Content string            `json:"content"`
}

// CVParsingResult represents the structured data extracted from a CV/resume
type CVParsingResult struct {
	IsValid        bool             `json:"isValid"`
	Reason         string           `json:"reason,omitempty"`
	PersonalInfo   PersonalInfo     `json:"personalInfo,omitempty"`
	WorkExperience []WorkExperience `json:"workExperience,omitempty"`
	Education      []Education      `json:"education,omitempty"`
	Certifications []Certification  `json:"certifications,omitempty"`
	Skills         []string         `json:"skills,omitempty"`
}

// PersonalInfo contains basic personal information from a CV
type PersonalInfo struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Name      string `json:"name,omitempty"` // fallback used by some models instead of firstName/lastName
	Email     string `json:"email,omitempty"`
	Phone     string `json:"phone,omitempty"`
	Location  string `json:"location,omitempty"`
	LinkedIn  string `json:"linkedin,omitempty"`
	Title     string `json:"title,omitempty"`
	Summary   string `json:"summary,omitempty"`
}

// NormalizeName splits the Name fallback field into FirstName/LastName when a model
// returns a single combined name instead of separate fields.
func (p *PersonalInfo) NormalizeName() {
	if p.FirstName != "" || p.LastName != "" || p.Name == "" {
		return
	}
	parts := strings.SplitN(strings.TrimSpace(p.Name), " ", 2)
	p.FirstName = parts[0]
	if len(parts) == 2 {
		p.LastName = parts[1]
	}
}

// WorkExperience represents a work experience entry from a CV
type WorkExperience struct {
	Company     string   `json:"company"`
	Title       string   `json:"title"`
	Location    string   `json:"location,omitempty"`
	StartDate   string   `json:"startDate"`         // Format: "YYYY-MM" or "YYYY"
	EndDate     string   `json:"endDate,omitempty"` // Format: "YYYY-MM" or "YYYY" or "Present"
	Description []string `json:"description,omitempty"`
}

// UnmarshalJSON normalises the description field so both string and []string from
// any provider result in a consistent []string. A plain string is split on newlines.
func (w *WorkExperience) UnmarshalJSON(data []byte) error {
	type Alias WorkExperience
	aux := &struct {
		Description json.RawMessage `json:"description,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(w),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if aux.Description == nil {
		return nil
	}
	// Try array first (preferred format)
	var arr []string
	if err := json.Unmarshal(aux.Description, &arr); err == nil {
		for _, line := range arr {
			if trimmed := strings.TrimSpace(line); trimmed != "" {
				w.Description = append(w.Description, trimmed)
			}
		}
		return nil
	}
	// Fall back to string: split on newlines
	var s string
	if err := json.Unmarshal(aux.Description, &s); err == nil {
		for _, line := range strings.Split(s, "\n") {
			if trimmed := strings.TrimSpace(line); trimmed != "" {
				w.Description = append(w.Description, trimmed)
			}
		}
		return nil
	}
	return nil
}

// Education represents an education entry from a CV
type Education struct {
	Institution  string `json:"institution"`
	Degree       string `json:"degree"`
	FieldOfStudy string `json:"fieldOfStudy,omitempty"`
	StartDate    string `json:"startDate"`         // Format: "YYYY-MM" or "YYYY"
	EndDate      string `json:"endDate,omitempty"` // Format: "YYYY-MM" or "YYYY"
}

// Certification represents a certification entry from a CV
type Certification struct {
	Name          string `json:"name"`
	IssuingOrg    string `json:"issuingOrg"`
	IssueDate     string `json:"issueDate"`            // Format: "YYYY-MM" or "YYYY"
	ExpiryDate    string `json:"expiryDate,omitempty"` // Format: "YYYY-MM" or "YYYY"
	CredentialID  string `json:"credentialId,omitempty"`
	CredentialURL string `json:"credentialUrl,omitempty"`
}

// GeneratedCV represents a CV generated for a specific job application
type GeneratedCV struct {
	CVParsingResult
	GeneratedAt int64  `json:"generatedAt"` // Unix timestamp
	JobID       int    `json:"jobId"`
	JobTitle    string `json:"jobTitle"`
}
