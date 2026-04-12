package models

import (
	"fmt"

	"github.com/benidevo/vega/internal/ai/prompts"
	"github.com/benidevo/vega/internal/ai/security"
	settingsmodels "github.com/benidevo/vega/internal/settings/models"
)

// AITaskType represents the type of AI task being performed
type AITaskType string

const (
	TaskTypeCVParsing    AITaskType = "cv_parsing"
	TaskTypeJobAnalysis  AITaskType = "job_analysis"
	TaskTypeMatchResult  AITaskType = "match_result"
	TaskTypeCoverLetter  AITaskType = "cover_letter"
	TaskTypeCVGeneration AITaskType = "cv_generation"
)

// String returns the string representation of the AITaskType
func (t AITaskType) String() string {
	return string(t)
}

// Request represents a generic request containing information needed for AI operations.
type Request struct {
	ApplicantName    string
	ApplicantProfile string
	JobDescription   string
	ExtraContext     string
	CVText           string

	WorkExperience  []settingsmodels.WorkExperience `json:"work_experience,omitempty"`
	Education       []settingsmodels.Education      `json:"education,omitempty"`
	Certifications  []settingsmodels.Certification  `json:"certifications,omitempty"`
	Skills          []string                        `json:"skills,omitempty"`
	YearsExperience int                             `json:"years_experience,omitempty"`
}

// Prompt represents the structure for a prompt used in the application.
type Prompt struct {
	Instructions string
	Request
	CVText               string
	UseEnhancedTemplates bool
	Temperature          *float32
	promptEnhancer       *prompts.PromptEnhancer
	sanitizer            *security.PromptSanitizer
}

// NewPrompt creates a new prompt with optional enhanced features
func NewPrompt(instructions string, request Request, useEnhanced bool) *Prompt {
	p := &Prompt{
		Instructions:         instructions,
		Request:              request,
		UseEnhancedTemplates: useEnhanced,
		CVText:               request.CVText, // Copy CVText from request to prompt
	}

	if useEnhanced {
		p.promptEnhancer = prompts.NewPromptEnhancer()
	}

	// Always initialize sanitizer for security
	p.sanitizer = security.NewPromptSanitizer()

	return p
}

// NewCVParsingPrompt creates a new prompt specifically for CV parsing
func NewCVParsingPrompt(cvText string) *Prompt {
	return &Prompt{
		Instructions:         "Parse CV and extract structured information",
		CVText:               cvText,
		UseEnhancedTemplates: false,
		sanitizer:            security.NewPromptSanitizer(), // Always initialize sanitizer for security
	}
}

// SetTemperature sets a custom temperature for this prompt
func (p *Prompt) SetTemperature(temp float32) {
	p.Temperature = &temp
}

// GetOptimalTemperature returns the optimal temperature for the prompt type
func (p *Prompt) GetOptimalTemperature(promptType string) float32 {
	if p.Temperature != nil {
		return *p.Temperature
	}

	// Dynamic temperature based on task type
	switch AITaskType(promptType) {
	case TaskTypeCoverLetter:
		return 0.65 // Higher creativity for writing
	case TaskTypeCVGeneration:
		return 0.55 // Higher creativity for CV content transformation
	case TaskTypeJobAnalysis:
		return 0.2 // Lower for analytical consistency
	default:
		return 0.4 // Default balanced temperature
	}
}

// ToCoverLetterPrompt builds a cover letter generation prompt.
func (p Prompt) ToCoverLetterPrompt(defaultWordRange string) string {
	sanitizedInstructions := p.Instructions
	sanitizedApplicantName := p.ApplicantName
	sanitizedJobDescription := p.JobDescription
	sanitizedApplicantProfile := p.ApplicantProfile
	sanitizedExtraContext := p.ExtraContext

	if p.sanitizer != nil {
		sanitizedInstructions = p.sanitizer.SanitizeInstructions(p.Instructions)
		sanitizedApplicantName = p.sanitizer.SanitizeText(p.ApplicantName)
		sanitizedJobDescription = p.sanitizer.SanitizeJobDescription(p.JobDescription)
		sanitizedApplicantProfile = p.sanitizer.SanitizeText(p.ApplicantProfile)
		sanitizedExtraContext = p.sanitizer.SanitizeExtraContext(p.ExtraContext)
	}

	if p.UseEnhancedTemplates && p.promptEnhancer != nil {
		return p.promptEnhancer.EnhanceCoverLetterPrompt(
			sanitizedInstructions,
			sanitizedApplicantName,
			sanitizedJobDescription,
			sanitizedApplicantProfile,
			sanitizedExtraContext,
			defaultWordRange,
		)
	}

	return fmt.Sprintf(`%s

Generate a professional cover letter with the following details:

Applicant: %s
Job Description: %s
Applicant Profile: %s
%s

Requirements:
- Write in a professional but personalized tone that reflects the candidate's personality
- Highlight relevant skills and experiences from the applicant's profile that directly match the job requirements
- Address specific requirements mentioned in the job description
- Keep it concise (%s words)
- Use proper business letter format without date/address headers
- Include a strong opening that captures attention
- Provide specific examples of achievements when possible
- End with a compelling call to action
- Avoid generic phrases and clichés
- Do not include placeholder text like [Company Name] or [Your Name]

CRITICAL - WRITE LIKE A HUMAN, NOT AI:
- BANNED PHRASES: Never use "leverage", "utilize", "spearheaded", "orchestrated", "synergies", "cutting-edge", "innovative solutions", "dynamic", "passionate", "results-driven", "detail-oriented", "team player", "go-getter", "game-changer", "disruptive", "seamless", "robust", "scalable", "streamlined", "optimized", "enhanced", "facilitated", "collaborated with stakeholders", "deep dive", "circle back", "deliverables"
- Write conversationally like explaining to a colleague, not corporate presentation style
- Use specific concrete details, not vague buzzwords
- Test: If it sounds like a template or AI wrote it, rewrite it completely
- Sound like a real person with genuine interest, not a marketing brochure

Return a JSON object with ONLY this field:
- content: the complete cover letter text (properly formatted with \n for line breaks)`,
		sanitizedInstructions,
		sanitizedApplicantName,
		sanitizedJobDescription,
		sanitizedApplicantProfile,
		sanitizedExtraContext,
		defaultWordRange)
}

// ToCVGenerationPrompt builds a CV generation prompt with security sanitization.
func (p Prompt) ToCVGenerationPrompt() string {
	// Sanitize all user inputs to prevent prompt injection
	sanitizedInstructions := p.Instructions
	sanitizedCVText := p.CVText
	sanitizedJobDescription := p.JobDescription
	sanitizedExtraContext := p.ExtraContext

	if p.sanitizer != nil {
		sanitizedInstructions = p.sanitizer.SanitizeInstructions(p.Instructions)
		sanitizedCVText = p.sanitizer.SanitizeCVText(p.CVText)
		sanitizedJobDescription = p.sanitizer.SanitizeJobDescription(p.JobDescription)
		sanitizedExtraContext = p.sanitizer.SanitizeExtraContext(p.ExtraContext)
	}

	if p.UseEnhancedTemplates && p.promptEnhancer != nil {
		return p.promptEnhancer.EnhanceCVGenerationPrompt(
			sanitizedInstructions,
			sanitizedCVText,
			sanitizedJobDescription,
			sanitizedExtraContext,
		)
	}

	sanitizedApplicantName := p.ApplicantName
	if p.sanitizer != nil {
		sanitizedApplicantName = p.sanitizer.SanitizeText(p.ApplicantName)
	}

	return fmt.Sprintf(`%s

Generate a tailored CV based on the user's profile and the job description.

APPLICANT NAME: %s

SOURCE OF TRUTH — CANDIDATE'S ACTUAL HISTORY (use this to populate work experience, education, skills):
%s

TARGET JOB — FOR TAILORING ONLY (do NOT add this as a work experience entry):
%s

%s

INSTRUCTIONS:
1. Work experience MUST come ONLY from SOURCE OF TRUTH above — never from TARGET JOB
2. NEVER add the target company or role as a work experience entry in the CV
3. The TARGET JOB is only used to decide which existing experiences and skills to emphasise
4. Use the APPLICANT NAME above for the personalInfo.firstName and personalInfo.lastName fields
5. Focus on achievements and impact in previous roles
6. Tailor the professional summary to match the job requirements
7. Use action verbs and quantify achievements where possible
8. Format work experience descriptions as an array of bullet point strings

CRITICAL - ELIMINATE ALL AI LANGUAGE:
10. BANNED WORDS/PHRASES: Never use "leverage", "utilize", "spearheaded", "orchestrated", "synergies", "cutting-edge", "innovative solutions", "dynamic", "passionate", "results-driven", "detail-oriented", "team player", "go-getter", "game-changer", "disruptive", "seamless", "robust", "scalable", "streamlined", "optimized", "enhanced", "facilitated", "collaborated with stakeholders", "deep dive", "circle back", "deliverables", "action items", "learnings", "best practices", "low-hanging fruit"
11. NATURAL WRITING: Write like a real person explaining their work to a colleague
12. HUMAN TEST: If any sentence sounds like AI/template language, rewrite it completely
13. SPECIFIC > GENERIC: Use concrete details instead of buzzwords and corporate speak
14. CONVERSATIONAL PROFESSIONAL: Sound competent but approachable, not like a press release

Generate a structured CV in JSON format following the exact schema requirements.`,
		sanitizedInstructions,
		sanitizedApplicantName,
		sanitizedCVText,
		sanitizedJobDescription,
		sanitizedExtraContext)
}

// ToMatchAnalysisPrompt builds a job match analysis prompt from this Prompt
func (p Prompt) ToMatchAnalysisPrompt(minMatchScore, maxMatchScore int) string {
	sanitizedInstructions := p.Instructions
	sanitizedApplicantName := p.ApplicantName
	sanitizedJobDescription := p.JobDescription
	sanitizedApplicantProfile := p.ApplicantProfile
	sanitizedExtraContext := p.ExtraContext

	if p.sanitizer != nil {
		sanitizedInstructions = p.sanitizer.SanitizeInstructions(p.Instructions)
		sanitizedApplicantName = p.sanitizer.SanitizeText(p.ApplicantName)
		sanitizedJobDescription = p.sanitizer.SanitizeJobDescription(p.JobDescription)
		sanitizedApplicantProfile = p.sanitizer.SanitizeText(p.ApplicantProfile)
		sanitizedExtraContext = p.sanitizer.SanitizeExtraContext(p.ExtraContext)
	}

	if p.UseEnhancedTemplates && p.promptEnhancer != nil {
		return p.promptEnhancer.EnhanceJobMatchPrompt(
			sanitizedInstructions,
			sanitizedApplicantName,
			sanitizedJobDescription,
			sanitizedApplicantProfile,
			sanitizedExtraContext,
			minMatchScore,
			maxMatchScore,
		)
	}

	return fmt.Sprintf(`%s

Analyze the match between this applicant and job opportunity:

Job Description: %s
Applicant Profile: %s
%s

CRITICAL SCORING GUIDELINES:
- Profile completeness is ESSENTIAL - incomplete profiles MUST receive VERY LOW scores (15%% or less)
- A profile with ONLY name, title, and a one-line summary should score 10-15%% MAX
- Missing work experience section: automatic cap at 20%% (even with good title match)
- Missing BOTH work experience AND education: automatic cap at 15%%
- Missing skills section when job lists required skills: reduce score by at least 20%%
- Empty or minimal career summaries (under 50 words) should cap score at 25%%
- To score above 50%%, profile MUST have substantial work experience, skills, AND either education or certifications

Provide a comprehensive analysis focusing on:
- Skills alignment with job requirements
- Experience level and relevance
- Industry knowledge fit
- Cultural fit indicators
- Growth potential
- Any concerns or red flags

IMPORTANT: In your feedback, do NOT mention the applicant's name. Use "you" and "your" directly. Be brutally honest and direct - no sugar-coating or patronizing language. State facts bluntly about what's missing or inadequate.

CRITICAL - WRITE LIKE A HUMAN RECRUITER, NOT AI:
- BANNED PHRASES: Never use "leverage", "utilize", "spearheaded", "orchestrated", "synergies", "cutting-edge", "innovative solutions", "dynamic", "passionate", "results-driven", "detail-oriented", "team player", "go-getter", "game-changer", "disruptive", "seamless", "robust", "scalable", "streamlined", "optimized", "enhanced", "facilitated", "collaborated with stakeholders", "deep dive", "circle back", "deliverables", "action items", "learnings", "best practices", "low-hanging fruit", "value-add"
- NATURAL FEEDBACK: Write like an experienced recruiter giving honest, straightforward feedback
- NO CORPORATE SPEAK: Use plain English, not HR buzzwords or template language
- CONVERSATIONAL TONE: Sound like you're talking to the person directly, not writing a formal report
- SPECIFIC EXAMPLES: Point to concrete gaps or strengths, not vague generalities
- HUMAN TEST: If it sounds like AI analysis or a form letter, rewrite it completely

Return the analysis as a JSON object with EXACTLY this structure:
- matchScore: integer from %d-%d where %d is no match and %d is perfect match
- strengths: array of 3-5 key strengths that align with job requirements
- weaknesses: array of 2-4 areas for improvement or skill gaps
- highlights: array of 3-5 standout qualifications that make this candidate attractive
- feedback: overall assessment and recommendations in 2-3 sentences (do NOT include the applicant's name)`,
		sanitizedInstructions,
		sanitizedJobDescription,
		sanitizedApplicantProfile,
		sanitizedExtraContext,
		minMatchScore,
		maxMatchScore,
		minMatchScore,
		maxMatchScore)
}
