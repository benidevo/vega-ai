package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMatchResult(t *testing.T) {
	t.Run("should_serialize_match_result_correctly_when_marshaling_to_json", func(t *testing.T) {
		matchResult := MatchResult{
			MatchScore: 85,
			Strengths:  []string{"Strong technical skills", "Relevant experience"},
			Weaknesses: []string{"Limited leadership experience"},
			Highlights: []string{"10 years in similar role", "Domain expertise"},
			Feedback:   "Excellent candidate with minor gaps",
		}

		jsonData, err := json.Marshal(matchResult)
		assert.NoError(t, err)

		var decoded MatchResult
		err = json.Unmarshal(jsonData, &decoded)
		assert.NoError(t, err)
		assert.Equal(t, matchResult, decoded)
	})

	t.Run("should_handle_empty_slices_when_marshaling", func(t *testing.T) {
		matchResult := MatchResult{
			MatchScore: 50,
			Strengths:  []string{},
			Weaknesses: []string{},
			Highlights: []string{},
			Feedback:   "Average match",
		}

		jsonData, err := json.Marshal(matchResult)
		assert.NoError(t, err)
		assert.Contains(t, string(jsonData), `"strengths":[]`)
		assert.Contains(t, string(jsonData), `"weaknesses":[]`)
		assert.Contains(t, string(jsonData), `"highlights":[]`)
	})

	t.Run("should_handle_nil_slices_when_unmarshaling", func(t *testing.T) {
		jsonStr := `{"matchScore":60,"feedback":"Good match"}`

		var matchResult MatchResult
		err := json.Unmarshal([]byte(jsonStr), &matchResult)
		assert.NoError(t, err)
		assert.Equal(t, 60, matchResult.MatchScore)
		assert.Equal(t, "Good match", matchResult.Feedback)
		assert.Nil(t, matchResult.Strengths)
		assert.Nil(t, matchResult.Weaknesses)
		assert.Nil(t, matchResult.Highlights)
	})
}

func TestCoverLetterFormat(t *testing.T) {
	tests := []struct {
		name     string
		format   CoverLetterFormat
		expected string
	}{
		{
			name:     "should_have_html_format_constant",
			format:   CoverLetterTypeHtml,
			expected: "html",
		},
		{
			name:     "should_have_markdown_format_constant",
			format:   CoverLetterTypeMarkdown,
			expected: "markdown",
		},
		{
			name:     "should_have_plain_text_format_constant",
			format:   CoverLetterTypePlainText,
			expected: "plain_text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.format))
		})
	}
}

func TestCoverLetter(t *testing.T) {
	tests := []struct {
		name        string
		coverLetter CoverLetter
	}{
		{
			name: "should_serialize_html_cover_letter_when_marshaling",
			coverLetter: CoverLetter{
				Format:  CoverLetterTypeHtml,
				Content: "<p>Dear Hiring Manager,</p><p>I am writing to apply...</p>",
			},
		},
		{
			name: "should_serialize_markdown_cover_letter_when_marshaling",
			coverLetter: CoverLetter{
				Format:  CoverLetterTypeMarkdown,
				Content: "# Cover Letter\n\nDear Hiring Manager,\n\nI am writing to apply...",
			},
		},
		{
			name: "should_serialize_plain_text_cover_letter_when_marshaling",
			coverLetter: CoverLetter{
				Format:  CoverLetterTypePlainText,
				Content: "Dear Hiring Manager,\n\nI am writing to apply...",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData, err := json.Marshal(tt.coverLetter)
			assert.NoError(t, err)

			var decoded CoverLetter
			err = json.Unmarshal(jsonData, &decoded)
			assert.NoError(t, err)
			assert.Equal(t, tt.coverLetter, decoded)
		})
	}
}

func TestCVParsingResult(t *testing.T) {
	t.Run("should_serialize_complete_cv_parsing_result_when_valid", func(t *testing.T) {
		result := CVParsingResult{
			IsValid: true,
			PersonalInfo: PersonalInfo{
				FirstName: "John",
				LastName:  "Doe",
				Email:     "john.doe@example.com",
				Phone:     "+1234567890",
				Location:  "New York, NY",
				LinkedIn:  "https://linkedin.com/in/johndoe",
				Title:     "Senior Software Engineer",
				Summary:   "Experienced software engineer with 10+ years",
			},
			WorkExperience: []WorkExperience{
				{
					Company:     "Tech Corp",
					Title:       "Senior Developer",
					Location:    "San Francisco, CA",
					StartDate:   "2020-01",
					EndDate:     "Present",
					Description: []string{"Led development of microservices"},
				},
			},
			Education: []Education{
				{
					Institution:  "MIT",
					Degree:       "Bachelor of Science",
					FieldOfStudy: "Computer Science",
					StartDate:    "2006",
					EndDate:      "2010",
				},
			},
			Certifications: []Certification{
				{
					Name:          "AWS Solutions Architect",
					IssuingOrg:    "Amazon",
					IssueDate:     "2022-03",
					ExpiryDate:    "2025-03",
					CredentialID:  "ABC123",
					CredentialURL: "https://aws.amazon.com/verify/ABC123",
				},
			},
			Skills: []string{"Go", "Python", "Kubernetes", "AWS"},
		}

		jsonData, err := json.Marshal(result)
		assert.NoError(t, err)

		var decoded CVParsingResult
		err = json.Unmarshal(jsonData, &decoded)
		assert.NoError(t, err)
		assert.Equal(t, result, decoded)
	})

	t.Run("should_serialize_invalid_cv_parsing_result_when_validation_fails", func(t *testing.T) {
		result := CVParsingResult{
			IsValid: false,
			Reason:  "Unable to extract personal information",
		}

		jsonData, err := json.Marshal(result)
		assert.NoError(t, err)
		assert.Contains(t, string(jsonData), `"isValid":false`)
		assert.Contains(t, string(jsonData), `"reason":"Unable to extract personal information"`)

		var decoded CVParsingResult
		err = json.Unmarshal(jsonData, &decoded)
		assert.NoError(t, err)
		assert.False(t, decoded.IsValid)
		assert.Equal(t, "Unable to extract personal information", decoded.Reason)
	})

	t.Run("should_omit_empty_fields_when_marshaling_with_omitempty_tags", func(t *testing.T) {
		result := CVParsingResult{
			IsValid: true,
			PersonalInfo: PersonalInfo{
				FirstName: "Jane",
				LastName:  "Smith",
				// Email, Phone, Location, LinkedIn, Title, Summary are omitted
			},
		}

		jsonData, err := json.Marshal(result)
		assert.NoError(t, err)

		jsonStr := string(jsonData)
		assert.Contains(t, jsonStr, `"firstName":"Jane"`)
		assert.Contains(t, jsonStr, `"lastName":"Smith"`)
		assert.NotContains(t, jsonStr, `"email":""`)
		assert.NotContains(t, jsonStr, `"phone":""`)
		assert.NotContains(t, jsonStr, `"location":""`)
	})
}

func TestPersonalInfo(t *testing.T) {
	t.Run("should_handle_partial_personal_info_when_some_fields_missing", func(t *testing.T) {
		info := PersonalInfo{
			FirstName: "Alice",
			LastName:  "Johnson",
			Email:     "alice@example.com",
			// Other fields omitted
		}

		jsonData, err := json.Marshal(info)
		assert.NoError(t, err)

		var decoded PersonalInfo
		err = json.Unmarshal(jsonData, &decoded)
		assert.NoError(t, err)
		assert.Equal(t, "Alice", decoded.FirstName)
		assert.Equal(t, "Johnson", decoded.LastName)
		assert.Equal(t, "alice@example.com", decoded.Email)
		assert.Empty(t, decoded.Phone)
		assert.Empty(t, decoded.Location)
	})
}

func TestWorkExperience(t *testing.T) {
	tests := []struct {
		name       string
		experience WorkExperience
	}{
		{
			name: "should_serialize_current_position_when_end_date_is_present",
			experience: WorkExperience{
				Company:     "Current Corp",
				Title:       "Lead Engineer",
				Location:    "Remote",
				StartDate:   "2022-06",
				EndDate:     "Present",
				Description: []string{"Leading a team of 5 engineers"},
			},
		},
		{
			name: "should_serialize_past_position_when_end_date_provided",
			experience: WorkExperience{
				Company:   "Previous Inc",
				Title:     "Software Developer",
				StartDate: "2019-01",
				EndDate:   "2022-05",
			},
		},
		{
			name: "should_handle_year_only_dates_when_month_not_specified",
			experience: WorkExperience{
				Company:   "Old Company",
				Title:     "Junior Developer",
				StartDate: "2017",
				EndDate:   "2019",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData, err := json.Marshal(tt.experience)
			assert.NoError(t, err)

			var decoded WorkExperience
			err = json.Unmarshal(jsonData, &decoded)
			assert.NoError(t, err)
			assert.Equal(t, tt.experience, decoded)
		})
	}
}

func TestWorkExperienceDescriptionUnmarshal(t *testing.T) {
	t.Run("should_split_string_description_on_newlines", func(t *testing.T) {
		raw := `{"company":"Acme","title":"Dev","startDate":"2020","description":"• Built things\n• Shipped stuff"}`
		var w WorkExperience
		err := json.Unmarshal([]byte(raw), &w)
		assert.NoError(t, err)
		assert.Equal(t, []string{"• Built things", "• Shipped stuff"}, w.Description)
	})

	t.Run("should_preserve_array_description_from_local_model", func(t *testing.T) {
		raw := `{"company":"Acme","title":"Dev","startDate":"2020","description":["• Built things","• Shipped stuff"]}`
		var w WorkExperience
		err := json.Unmarshal([]byte(raw), &w)
		assert.NoError(t, err)
		assert.Equal(t, []string{"• Built things", "• Shipped stuff"}, w.Description)
	})

	t.Run("should_handle_missing_description", func(t *testing.T) {
		raw := `{"company":"Acme","title":"Dev","startDate":"2020"}`
		var w WorkExperience
		err := json.Unmarshal([]byte(raw), &w)
		assert.NoError(t, err)
		assert.Nil(t, w.Description)
	})
}

func TestEducation(t *testing.T) {
	t.Run("should_serialize_education_with_all_fields_when_complete", func(t *testing.T) {
		edu := Education{
			Institution:  "Harvard University",
			Degree:       "Master of Science",
			FieldOfStudy: "Computer Science",
			StartDate:    "2010-09",
			EndDate:      "2012-06",
		}

		jsonData, err := json.Marshal(edu)
		assert.NoError(t, err)

		var decoded Education
		err = json.Unmarshal(jsonData, &decoded)
		assert.NoError(t, err)
		assert.Equal(t, edu, decoded)
	})

	t.Run("should_handle_education_without_field_of_study_when_omitted", func(t *testing.T) {
		edu := Education{
			Institution: "Community College",
			Degree:      "Associate Degree",
			StartDate:   "2008",
			EndDate:     "2010",
		}

		jsonData, err := json.Marshal(edu)
		assert.NoError(t, err)
		assert.NotContains(t, string(jsonData), `"fieldOfStudy":""`)
	})
}

func TestCertification(t *testing.T) {
	t.Run("should_serialize_certification_with_expiry_when_provided", func(t *testing.T) {
		cert := Certification{
			Name:          "Kubernetes Administrator",
			IssuingOrg:    "CNCF",
			IssueDate:     "2023-01",
			ExpiryDate:    "2026-01",
			CredentialID:  "CKA-2023-001",
			CredentialURL: "https://cncf.io/verify/CKA-2023-001",
		}

		jsonData, err := json.Marshal(cert)
		assert.NoError(t, err)

		var decoded Certification
		err = json.Unmarshal(jsonData, &decoded)
		assert.NoError(t, err)
		assert.Equal(t, cert, decoded)
	})

	t.Run("should_serialize_certification_without_expiry_when_non_expiring", func(t *testing.T) {
		cert := Certification{
			Name:       "Project Management Professional",
			IssuingOrg: "PMI",
			IssueDate:  "2022-05",
			// No expiry date
		}

		jsonData, err := json.Marshal(cert)
		assert.NoError(t, err)
		assert.NotContains(t, string(jsonData), `"expiryDate":""`)
		assert.NotContains(t, string(jsonData), `"credentialId":""`)
		assert.NotContains(t, string(jsonData), `"credentialUrl":""`)
	})
}

func TestGeneratedCV(t *testing.T) {
	t.Run("should_embed_cv_parsing_result_when_creating_generated_cv", func(t *testing.T) {
		now := time.Now().Unix()
		generatedCV := GeneratedCV{
			CVParsingResult: CVParsingResult{
				IsValid: true,
				PersonalInfo: PersonalInfo{
					FirstName: "Test",
					LastName:  "User",
					Email:     "test@example.com",
				},
				Skills: []string{"Go", "Docker"},
			},
			GeneratedAt: now,
			JobID:       12345,
			JobTitle:    "Senior Go Developer",
		}

		jsonData, err := json.Marshal(generatedCV)
		assert.NoError(t, err)

		var decoded GeneratedCV
		err = json.Unmarshal(jsonData, &decoded)
		assert.NoError(t, err)

		assert.Equal(t, generatedCV.GeneratedAt, decoded.GeneratedAt)
		assert.Equal(t, generatedCV.JobID, decoded.JobID)
		assert.Equal(t, generatedCV.JobTitle, decoded.JobTitle)
		assert.Equal(t, generatedCV.PersonalInfo.FirstName, decoded.PersonalInfo.FirstName)
		assert.Equal(t, generatedCV.Skills, decoded.Skills)
	})

	t.Run("should_maintain_all_cv_fields_when_embedded", func(t *testing.T) {
		generatedCV := GeneratedCV{
			CVParsingResult: CVParsingResult{
				IsValid: true,
				PersonalInfo: PersonalInfo{
					FirstName: "John",
					LastName:  "Doe",
				},
				WorkExperience: []WorkExperience{
					{Company: "Tech Co", Title: "Developer", StartDate: "2020"},
				},
				Education: []Education{
					{Institution: "University", Degree: "BS", StartDate: "2015", EndDate: "2019"},
				},
			},
			GeneratedAt: 1234567890,
			JobID:       999,
			JobTitle:    "Full Stack Developer",
		}

		assert.True(t, generatedCV.IsValid)
		assert.Equal(t, "John", generatedCV.PersonalInfo.FirstName)
		assert.Len(t, generatedCV.WorkExperience, 1)
		assert.Len(t, generatedCV.Education, 1)
		assert.Equal(t, int64(1234567890), generatedCV.GeneratedAt)
	})
}
