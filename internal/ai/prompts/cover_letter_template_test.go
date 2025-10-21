package prompts

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoverLetterTemplate(t *testing.T) {
	template := CoverLetterTemplate()

	assert.NotNil(t, template)
	assert.Contains(t, template.Role, "conversational cover letters")
	assert.Contains(t, template.Context, "personalized cover letter")
	assert.Len(t, template.Examples, 0)
	assert.Contains(t, template.Task, "compelling cover letter")
	assert.Greater(t, len(template.Constraints), 5)
	assert.Contains(t, template.OutputSpec, "JSON")
	assert.Contains(t, template.Task, "Best regards")

	constraintTexts := strings.Join(template.Constraints, " ")
	assert.Contains(t, constraintTexts, "ACTUAL name")
	assert.Contains(t, constraintTexts, "em dashes")
}

func TestEnhanceCoverLetterPrompt(t *testing.T) {
	tests := []struct {
		name              string
		systemInstruction string
		applicantName     string
		jobDescription    string
		applicantProfile  string
		extraContext      string
		wordRange         string
		expectedContains  []string
	}{
		{
			name:              "should_build_complete_cover_letter_prompt",
			systemInstruction: "You are an AI assistant",
			applicantName:     "Sarah Johnson",
			jobDescription:    "Marketing Manager at Tech Startup",
			applicantProfile:  "Digital marketing expert with 8 years experience",
			extraContext:      "Passionate about sustainability",
			wordRange:         "250-350",
			expectedContains: []string{
				"You are an AI assistant",
				"Sarah Johnson",
				"Marketing Manager at Tech Startup",
				"Digital marketing expert with 8 years experience",
				"Passionate about sustainability",
				"250-350",
				"conversational cover letters",
				"cover letter",
				"Best regards",
			},
		},
		{
			name:             "should_handle_empty_optional_fields",
			applicantName:    "Bob Smith",
			jobDescription:   "Software Developer",
			applicantProfile: "Python developer",
			wordRange:        "300",
			expectedContains: []string{
				"Bob Smith",
				"Software Developer",
				"Python developer",
				"300",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EnhanceCoverLetterPrompt(
				tt.systemInstruction,
				tt.applicantName,
				tt.jobDescription,
				tt.applicantProfile,
				tt.extraContext,
				tt.wordRange,
			)

			for _, expected := range tt.expectedContains {
				assert.Contains(t, result, expected)
			}
		})
	}
}
