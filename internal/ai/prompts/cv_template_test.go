package prompts

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnhanceCVGenerationPrompt(t *testing.T) {
	tests := []struct {
		name              string
		systemInstruction string
		cvText            string
		jobDescription    string
		extraContext      string
		expectedContains  []string
	}{
		{
			name:              "should_build_complete_cv_generation_prompt",
			systemInstruction: "You are a CV expert",
			cvText:            "John Doe\nSoftware Engineer\n10 years experience",
			jobDescription:    "Senior Developer at Tech Corp",
			extraContext:      "Focus on leadership experience",
			expectedContains: []string{
				"You are a CV expert",
				"John Doe\nSoftware Engineer\n10 years experience",
				"Senior Developer at Tech Corp",
				"senior CV writer",
				"ATS-optimized",
			},
		},
		{
			name:           "should_handle_empty_system_instruction",
			cvText:         "Basic CV content",
			jobDescription: "Basic job",
			extraContext:   "Some context",
			expectedContains: []string{
				"Basic CV content",
				"Basic job",
				"senior CV writer",
			},
		},
		{
			name:           "should_handle_empty_extra_context",
			cvText:         "CV content",
			jobDescription: "Job description",
			expectedContains: []string{
				"CV content",
				"Job description",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EnhanceCVGenerationPrompt(
				tt.systemInstruction,
				tt.cvText,
				tt.jobDescription,
				tt.extraContext,
			)

			for _, expected := range tt.expectedContains {
				assert.Contains(t, result, expected)
			}
		})
	}
}
