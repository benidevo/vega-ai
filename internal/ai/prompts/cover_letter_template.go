package prompts

import "time"

// CoverLetterTemplate returns a template for cover letter generation
func CoverLetterTemplate() *PromptTemplate {
	return &PromptTemplate{
		Role:     "You write authentic, conversational cover letters that sound like real people, not templates.",
		Context:  "Create a personalized cover letter showing how the candidate's skills and experience align with the role.",
		Examples: []Example{},
		Task:     "Write a compelling cover letter. Sign with 'Best regards,' followed by the applicant's name.",
		Constraints: []string{
			"Write like a person having a professional conversation - no corporate jargon",
			"Include specific achievements with numbers, woven naturally into the story",
			"Show genuine interest in the company/role - don't just parrot their words",
			"Keep paragraphs short (3-4 sentences max)",
			"Avoid AI buzzwords (no 'leverage', 'spearheaded', 'synergies', 'cutting-edge', etc.)",
			"Mix sentence lengths like natural speech",
			"Sound excited but authentic - not trying too hard to impress",
			"End with friendly call to action",
			"Sign with applicant's ACTUAL name, never a placeholder",
			"No em dashes (—) - use commas instead",
		},
		OutputSpec: "Return ONLY valid JSON: {\"content\": \"letter text with \\n for line breaks\"}",
	}
}

// EnhanceCoverLetterPrompt enhances a cover letter prompt
func EnhanceCoverLetterPrompt(systemInstruction, applicantName, jobDescription, applicantProfile, extraContext, wordRange string) string {
	template := CoverLetterTemplate()
	params := map[string]any{
		"wordRange":   wordRange,
		"currentDate": time.Now().Format("January 2, 2006"),
	}
	return template.BuildPrompt(systemInstruction, applicantName, jobDescription, applicantProfile, extraContext, params)
}
