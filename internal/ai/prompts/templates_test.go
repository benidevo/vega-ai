package prompts

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPromptEnhancer(t *testing.T) {
	enhancer := NewPromptEnhancer()

	assert.NotNil(t, enhancer)
	assert.NotNil(t, enhancer.templates)
	assert.Contains(t, enhancer.templates, "cover_letter")
	assert.Contains(t, enhancer.templates, "job_match")
}

func TestPromptEnhancer_EnhanceCoverLetterPrompt(t *testing.T) {
	enhancer := NewPromptEnhancer()

	result := enhancer.EnhanceCoverLetterPrompt(
		"System instruction",
		"John Doe",
		"Software Engineer position",
		"Experienced developer",
		"Extra context",
		"300-500",
	)

	assert.Contains(t, result, "John Doe")
	assert.Contains(t, result, "Software Engineer position")
	assert.Contains(t, result, "Experienced developer")
	assert.Contains(t, result, "300-500")
}

func TestPromptEnhancer_EnhanceJobMatchPrompt(t *testing.T) {
	enhancer := NewPromptEnhancer()

	result := enhancer.EnhanceJobMatchPrompt(
		"System instruction",
		"Jane Smith",
		"Data Scientist role",
		"ML expertise",
		"Additional info",
		0,
		100,
	)

	assert.Contains(t, result, "Jane Smith")
	assert.Contains(t, result, "Data Scientist role")
	assert.Contains(t, result, "ML expertise")
	assert.Contains(t, result, "0-100")
}

func TestPromptEnhancer_EnhanceCVGenerationPrompt(t *testing.T) {
	enhancer := NewPromptEnhancer()

	result := enhancer.EnhanceCVGenerationPrompt(
		"System instruction",
		"Current CV content",
		"Target job description",
		"Extra requirements",
	)

	assert.Contains(t, result, "Current CV content")
	assert.Contains(t, result, "Target job description")
	assert.Contains(t, result, "ATS-optimized")
}
