package prompts

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJobMatchTemplate(t *testing.T) {
	template := JobMatchTemplate()

	assert.NotNil(t, template)
	assert.Contains(t, template.Role, "talent acquisition specialist")
	assert.Contains(t, template.Context, "Analyze how well")
	assert.Len(t, template.Examples, 0)
	assert.Contains(t, template.Task, "data-driven analysis")
	assert.Greater(t, len(template.Constraints), 5)
	assert.Contains(t, template.OutputSpec, "JSON")
}

func TestEnhanceJobMatchPrompt(t *testing.T) {
	tests := []struct {
		name              string
		systemInstruction string
		applicantName     string
		jobDescription    string
		applicantProfile  string
		extraContext      string
		minScore          int
		maxScore          int
		expectedContains  []string
	}{
		{
			name:              "should_build_complete_job_match_prompt",
			systemInstruction: "System prompt",
			applicantName:     "Alice Brown",
			jobDescription:    "Senior Data Scientist at AI Company",
			applicantProfile:  "ML Engineer with PhD in AI",
			extraContext:      "Published research papers",
			minScore:          0,
			maxScore:          100,
			expectedContains: []string{
				"System prompt",
				"Alice Brown",
				"Senior Data Scientist at AI Company",
				"ML Engineer with PhD in AI",
				"Published research papers",
				"0-100",
				"talent acquisition specialist",
				"matchScore: 0-100",
			},
		},
		{
			name:             "should_handle_custom_score_range",
			applicantName:    "Test User",
			jobDescription:   "Test Job",
			applicantProfile: "Test Profile",
			minScore:         10,
			maxScore:         90,
			expectedContains: []string{
				"10-90",
			},
		},
		{
			name:             "should_handle_empty_optional_fields",
			applicantName:    "User",
			jobDescription:   "Job",
			applicantProfile: "Profile",
			minScore:         0,
			maxScore:         100,
			expectedContains: []string{
				"User",
				"Job",
				"Profile",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EnhanceJobMatchPrompt(
				tt.systemInstruction,
				tt.applicantName,
				tt.jobDescription,
				tt.applicantProfile,
				tt.extraContext,
				tt.minScore,
				tt.maxScore,
			)

			for _, expected := range tt.expectedContains {
				assert.Contains(t, result, expected)
			}
			assert.Contains(t, result, "SCORING RULES")
		})
	}
}
