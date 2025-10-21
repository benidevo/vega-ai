package prompts

import (
	"fmt"
	"time"
)

// JobMatchTemplate returns a template for job matching analysis
func JobMatchTemplate() *PromptTemplate {
	return &PromptTemplate{
		Role:     "You're a talent acquisition specialist assessing candidate-job fit across all industries.",
		Context:  "Analyze how well the candidate matches this job. Be moderately lenient - value similar/transferable skills, not just exact matches.",
		Examples: []Example{},
		Task:     "Provide a direct, data-driven analysis. Use 'you/your' - never mention candidate's name or use 'the candidate'.",
		Constraints: []string{
			"SCORING RULES:",
			"- Incomplete profiles score VERY LOW (15% or less)",
			"- Only name/title/one-line summary: 10-15% MAX",
			"- Missing work experience: cap at 20%",
			"- Missing work experience AND education: cap at 15%",
			"- Missing skills when none similar exist: reduce 10-15%",
			"- Minimal summary (<50 words): cap at 25%",
			"- To score 50%+: need solid experience + related skills",
			"",
			"SKILL MATCHING:",
			"- Value similar skills (Python/Java, React/Vue, AWS/Azure) +5-10pts",
			"- Transferable skills count (project mgmt, problem-solving)",
			"- Don't penalize heavily for missing exact tech if strong foundation exists",
			"",
			"EXPERIENCE EVALUATION:",
			"- 2+ years exp: prioritize work history over education",
			"- <2 years exp: education/certs carry more weight",
			"- Same industry: +5-10pts | Related industry: +3-5pts",
			"- Soft skills with examples: +3-5pts each",
			"",
			"TONE:",
			"- Be brutally honest - no sugar-coating",
			"- State facts bluntly - incomplete = unqualified",
			"- Call weaknesses what they are, not 'opportunities'",
			"- Write naturally - avoid AI buzzwords",
		},
		OutputSpec: "Return ONLY valid JSON: {matchScore: 0-100, strengths: [3-5 items], weaknesses: [2-4 items], highlights: [3-5 items], feedback: \"2-3 sentences\"}",
	}
}

// EnhanceJobMatchPrompt enhances a job matching prompt
func EnhanceJobMatchPrompt(systemInstruction, applicantName, jobDescription, applicantProfile, extraContext string, minScore, maxScore int) string {
	template := JobMatchTemplate()

	params := map[string]any{
		"minScore":    minScore,
		"maxScore":    maxScore,
		"currentDate": time.Now().Format("January 2, 2006"),
	}

	// Add score range to output spec
	template.OutputSpec = fmt.Sprintf("Return ONLY valid JSON: {matchScore: %d-%d, strengths: [3-5 items], weaknesses: [2-4 items], highlights: [3-5 items], feedback: \"2-3 sentences\"}",
		minScore, maxScore)

	return template.BuildPrompt(systemInstruction, applicantName, jobDescription, applicantProfile, extraContext, params)
}
