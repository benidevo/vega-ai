package testutil

import (
	"context"
	"time"

	"github.com/benidevo/vega/internal/ai/llm"
	"github.com/benidevo/vega/internal/ai/models"
	"github.com/stretchr/testify/mock"
)

// MockProvider provides a reusable mock implementation of llm.Provider
type MockProvider struct {
	mock.Mock
}

// Generate implements the llm.Provider interface
func (m *MockProvider) Generate(ctx context.Context, req llm.GenerateRequest) (llm.GenerateResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return llm.GenerateResponse{}, args.Error(1)
	}
	return args.Get(0).(llm.GenerateResponse), args.Error(1)
}

// SetupCVParsingMock configures the mock for CV parsing operations
func (m *MockProvider) SetupCVParsingMock(result models.CVParsingResult, err error) {
	response := llm.GenerateResponse{
		Data:     result,
		Duration: 500 * time.Millisecond,
		Tokens:   0,
		Metadata: map[string]interface{}{
			"temperature": float32(0.1),
			"model":       "gpt-4o-mini",
			"task_type":   "cv_parsing",
			"method":      "openai_cv_parsing",
		},
	}

	m.On("Generate", mock.Anything, mock.MatchedBy(func(req llm.GenerateRequest) bool {
		return req.ResponseType == llm.ResponseTypeCVParsing
	})).Return(response, err)
}

// SetupJobAnalysisMock configures the mock for job analysis operations
func (m *MockProvider) SetupJobAnalysisMock(result models.MatchResult, err error) {
	response := llm.GenerateResponse{
		Data:     result,
		Duration: 1000 * time.Millisecond,
		Tokens:   0,
		Metadata: map[string]interface{}{
			"temperature": float32(0.4),
			"enhanced":    false,
			"model":       "gpt-4o-mini",
			"task_type":   "job_analysis",
		},
	}

	m.On("Generate", mock.Anything, mock.MatchedBy(func(req llm.GenerateRequest) bool {
		return req.ResponseType == llm.ResponseTypeMatchResult
	})).Return(response, err)
}

// SetupCoverLetterMock configures the mock for cover letter generation
func (m *MockProvider) SetupCoverLetterMock(result models.CoverLetter, err error) {
	response := llm.GenerateResponse{
		Data:     result,
		Duration: 1500 * time.Millisecond,
		Tokens:   0,
		Metadata: map[string]interface{}{
			"temperature": float32(0.6),
			"enhanced":    false,
			"model":       "gpt-4o-mini",
			"task_type":   "cover_letter",
		},
	}

	m.On("Generate", mock.Anything, mock.MatchedBy(func(req llm.GenerateRequest) bool {
		return req.ResponseType == llm.ResponseTypeCoverLetter
	})).Return(response, err)
}

// SetupCVGenerationMock configures the mock for CV generation operations
func (m *MockProvider) SetupCVGenerationMock(result models.CVParsingResult, err error) {
	response := llm.GenerateResponse{
		Data:     result,
		Duration: 800 * time.Millisecond,
		Tokens:   0,
		Metadata: map[string]interface{}{
			"temperature": float32(0.55),
			"enhanced":    false,
			"model":       "gpt-4o-mini",
			"task_type":   "cv_generation",
			"method":      "openai_cv_generation",
		},
	}

	m.On("Generate", mock.Anything, mock.MatchedBy(func(req llm.GenerateRequest) bool {
		return req.ResponseType == llm.ResponseTypeCV
	})).Return(response, err)
}

// SetupGenericMock configures the mock to return any response for any request
func (m *MockProvider) SetupGenericMock(response llm.GenerateResponse, err error) {
	m.On("Generate", mock.Anything, mock.AnythingOfType("llm.GenerateRequest")).
		Return(response, err)
}

// TestData contains commonly used test data
type TestData struct{}

// ValidCVText returns a realistic CV text for testing
func (td *TestData) ValidCVText() string {
	return `SARAH JOHNSON
Senior Frontend Developer
Email: sarah.johnson@email.com | Phone: +1 (555) 234-5678
Location: Seattle, WA | LinkedIn: linkedin.com/in/sarahjohnson

PROFESSIONAL SUMMARY
Creative and detail-oriented Frontend Developer with 6+ years of experience building responsive web applications and user interfaces. Expertise in React, TypeScript, and modern CSS frameworks.

WORK EXPERIENCE

Senior Frontend Developer | WebTech Solutions | March 2021 - Present
• Lead frontend development for e-commerce platform serving 100K+ daily users
• Implemented React components using TypeScript and modern hooks
• Collaborated with UX/UI designers to create intuitive user experiences
• Optimized application performance resulting in 30% faster load times
• Mentored 3 junior developers and conducted code reviews

Frontend Developer | DigitalCorp | June 2019 - February 2021
• Developed responsive web applications using React and Redux
• Integrated REST APIs and GraphQL endpoints
• Implemented automated testing with Jest and React Testing Library
• Participated in agile development processes and sprint planning

Junior Frontend Developer | StartupTech | September 2017 - May 2019
• Built UI components using HTML5, CSS3, and JavaScript (ES6+)
• Worked on mobile-first responsive design principles
• Collaborated with backend developers on API integration
• Contributed to component library and design system

EDUCATION

Bachelor of Science in Computer Science | University of Washington | 2013 - 2017
• Graduated Cum Laude (GPA: 3.6/4.0)
• Relevant Coursework: Web Development, Human-Computer Interaction, Software Engineering

TECHNICAL SKILLS
Frontend: React, TypeScript, JavaScript, HTML5, CSS3, SASS/SCSS
Frameworks: Next.js, Gatsby, Vue.js (basic)
Styling: Tailwind CSS, Bootstrap, Material-UI, Styled Components
Tools: Webpack, Vite, Git, npm/yarn, Figma, Adobe XD
Testing: Jest, React Testing Library, Cypress
State Management: Redux, Context API, Zustand

PROJECTS
Personal Portfolio Website (2023)
• Built with Next.js and TypeScript, deployed on Vercel
• Implemented dark mode toggle and responsive design
• Integrated headless CMS for blog functionality

E-commerce Dashboard (2022)
• React admin dashboard with real-time analytics
• Chart.js integration for data visualization
• Role-based access control and user management`
}

// PartialCVText returns CV text with some missing information
func (td *TestData) PartialCVText() string {
	return `Mike Chen
Software Developer
mike.chen@email.com

SKILLS
Python, Django, PostgreSQL, Docker, AWS

EDUCATION
BS Computer Science, 2020`
}

// InvalidDocumentText returns text that should be rejected as non-CV
func (td *TestData) InvalidDocumentText() string {
	return `MEDICAL CONSULTATION REPORT
Patient: John Doe
Date of Birth: 01/15/1980
Medical Record Number: MRN-123456

CHIEF COMPLAINT
Patient presents with persistent headaches and fatigue over the past two weeks.

HISTORY OF PRESENT ILLNESS
45-year-old male reports onset of daily headaches approximately 14 days ago. Describes pain as throbbing, primarily frontal and temporal regions. Associated with mild photophobia and occasional nausea.

PAST MEDICAL HISTORY
- Hypertension (diagnosed 2015)
- Type 2 Diabetes Mellitus (diagnosed 2018)
- No known drug allergies

CURRENT MEDICATIONS
- Lisinopril 10mg daily
- Metformin 500mg twice daily
- Aspirin 81mg daily

SOCIAL HISTORY
- Non-smoker
- Occasional alcohol use (1-2 drinks per week)
- Works as an accountant, denies significant stress

PHYSICAL EXAMINATION
Vital Signs: BP 142/88, HR 72, T 98.6°F, RR 16
General: Alert and oriented, appears mildly fatigued
HEENT: PERRL, no papilledema, neck supple
Cardiovascular: Regular rate and rhythm, no murmurs
Neurological: Cranial nerves II-XII intact, no focal deficits

ASSESSMENT AND PLAN
1. Tension-type headache - likely stress-related
   - Recommend stress management techniques
   - Follow up in 2 weeks if symptoms persist
2. Hypertension - adequate control on current medication
3. Continue current diabetes management

Dr. Smith, MD
Internal Medicine`
}

// ValidCVParsingResult returns expected result for valid CV parsing
func (td *TestData) ValidCVParsingResult() models.CVParsingResult {
	return models.CVParsingResult{
		IsValid: true,
		PersonalInfo: models.PersonalInfo{
			FirstName: "Sarah",
			LastName:  "Johnson",
			Email:     "sarah.johnson@email.com",
			Phone:     "+1 (555) 234-5678",
			Location:  "Seattle, WA",
			Title:     "Senior Frontend Developer",
		},
		WorkExperience: []models.WorkExperience{
			{
				Company:     "WebTech Solutions",
				Title:       "Senior Frontend Developer",
				StartDate:   "2021-03",
				EndDate:     "Present",
				Description: []string{"Lead frontend development for e-commerce platform"},
			},
			{
				Company:     "DigitalCorp",
				Title:       "Frontend Developer",
				StartDate:   "2019-06",
				EndDate:     "2021-02",
				Description: []string{"Developed responsive web applications using React"},
			},
		},
		Education: []models.Education{
			{
				Institution:  "University of Washington",
				Degree:       "Bachelor of Science",
				FieldOfStudy: "Computer Science",
				StartDate:    "2013",
				EndDate:      "2017",
			},
		},
		Skills: []string{
			"React", "TypeScript", "JavaScript", "HTML5", "CSS3",
			"Next.js", "Redux", "Jest", "Webpack", "Git",
		},
	}
}

// PartialCVParsingResult returns result for CV with minimal information
func (td *TestData) PartialCVParsingResult() models.CVParsingResult {
	return models.CVParsingResult{
		IsValid: true,
		PersonalInfo: models.PersonalInfo{
			FirstName: "Mike",
			LastName:  "Chen",
			Email:     "mike.chen@email.com",
			Title:     "Software Developer",
		},
		WorkExperience: []models.WorkExperience{},
		Education: []models.Education{
			{
				Institution:  "Unknown University",
				Degree:       "BS",
				FieldOfStudy: "Computer Science",
				EndDate:      "2020",
			},
		},
		Skills: []string{"Python", "Django", "PostgreSQL", "Docker", "AWS"},
	}
}

// InvalidCVParsingResult returns result for rejected documents
func (td *TestData) InvalidCVParsingResult(reason string) models.CVParsingResult {
	return models.CVParsingResult{
		IsValid: false,
		Reason:  reason,
	}
}

// ValidMatchResult returns a typical job matching result
func (td *TestData) ValidMatchResult() models.MatchResult {
	return models.MatchResult{
		MatchScore: 82,
		Strengths: []string{
			"Strong frontend development experience",
			"Proficient in React and TypeScript",
			"Good leadership and mentoring skills",
		},
		Weaknesses: []string{
			"Limited backend development experience",
			"Could benefit from more cloud platform knowledge",
		},
		Highlights: []string{
			"6+ years of relevant experience",
			"Experience with modern frontend frameworks",
			"Strong educational background",
		},
		Feedback: "Excellent candidate for frontend developer positions. Strong technical skills and leadership experience make this a great fit for senior roles.",
	}
}

// ValidCoverLetter returns a sample cover letter
func (td *TestData) ValidCoverLetter() models.CoverLetter {
	return models.CoverLetter{
		Content: `Dear Hiring Manager,

I am writing to express my strong interest in the Senior Frontend Developer position at your company. With over 6 years of experience in frontend development and a proven track record of building scalable web applications, I am excited about the opportunity to contribute to your team.

In my current role at WebTech Solutions, I have led the frontend development for an e-commerce platform serving over 100,000 daily users. My expertise in React, TypeScript, and modern CSS frameworks has enabled me to deliver high-quality, responsive user interfaces that enhance user experience and drive business results.

I am particularly drawn to your company's commitment to innovation and user-centric design. My experience in mentoring junior developers and collaborating with cross-functional teams aligns well with your collaborative culture.

Thank you for considering my application. I look forward to discussing how my skills and passion for frontend development can contribute to your team's success.

Sincerely,
Sarah Johnson`,
		Format: models.CoverLetterTypePlainText,
	}
}

// ValidRequest returns a complete request for AI operations
func (td *TestData) ValidRequest() models.Request {
	return models.Request{
		ApplicantName: "Sarah Johnson",
		ApplicantProfile: `Senior Frontend Developer with 6+ years of experience building responsive web applications and user interfaces. Expertise in React, TypeScript, and modern CSS frameworks. Strong leadership skills with experience mentoring junior developers.

		Technical Skills:
		- Frontend: React, TypeScript, JavaScript, HTML5, CSS3
		- Frameworks: Next.js, Gatsby, Vue.js
		- Tools: Webpack, Git, Jest, Cypress
		- Styling: Tailwind CSS, Bootstrap, Material-UI

		Experience:
		- Senior Frontend Developer at WebTech Solutions (2021-Present)
		- Frontend Developer at DigitalCorp (2019-2021)
		- Junior Frontend Developer at StartupTech (2017-2019)

		Education:
		- BS Computer Science, University of Washington (2013-2017)`,
		JobDescription: `Senior Frontend Developer

		We are seeking a talented Senior Frontend Developer to join our growing engineering team. You will be responsible for building and maintaining our customer-facing web applications using modern technologies.

		Responsibilities:
		- Develop responsive web applications using React and TypeScript
		- Collaborate with designers and backend developers
		- Optimize application performance and user experience
		- Mentor junior developers and conduct code reviews
		- Participate in architectural decisions

		Requirements:
		- 5+ years of frontend development experience
		- Expert knowledge of React and TypeScript
		- Experience with modern CSS frameworks
		- Strong understanding of web performance optimization
		- Excellent communication and collaboration skills
		- Experience with testing frameworks (Jest, Cypress)

		Nice to have:
		- Experience with Next.js or similar SSR frameworks
		- Knowledge of GraphQL
		- Experience with cloud platforms (AWS, Vercel)
		- Leadership or mentoring experience`,
		ExtraContext: "This is a senior-level position in a fast-growing startup focused on e-commerce solutions. The company values innovation, collaboration, and continuous learning.",
	}
}

// MinimalRequest returns a request with minimal valid information
func (td *TestData) MinimalRequest() models.Request {
	return models.Request{
		ApplicantName:    "John Doe",
		ApplicantProfile: "Software Engineer with 3 years experience.",
		JobDescription:   "Software Engineer position at a tech company.",
	}
}

// ValidGeneratedCV returns a sample generated CV for testing
func (td *TestData) ValidGeneratedCV() models.CVParsingResult {
	return models.CVParsingResult{
		IsValid: true,
		PersonalInfo: models.PersonalInfo{
			FirstName: "Sarah",
			LastName:  "Johnson",
			Email:     "sarah.johnson@email.com",
			Phone:     "+1 (555) 234-5678",
			Location:  "Seattle, WA",
			Title:     "Senior Frontend Developer",
		},
		WorkExperience: []models.WorkExperience{
			{
				Company:   "WebTech Solutions",
				Title:     "Senior Frontend Developer",
				Location:  "Seattle, WA",
				StartDate: "March 2021",
				EndDate:   "Present",
				Description: []string{
					"• Lead frontend development for e-commerce platform serving 100K+ daily users",
					"• Implemented React components using TypeScript and modern hooks",
					"• Optimized application performance resulting in 30% faster load times",
					"• Mentored 3 junior developers and conducted code reviews",
				},
			},
		},
		Education: []models.Education{
			{
				Institution:  "University of Washington",
				Degree:       "Bachelor of Science",
				FieldOfStudy: "Computer Science",
				StartDate:    "Sep 2013",
				EndDate:      "Jun 2017",
			},
		},
		Skills: []string{"React", "TypeScript", "JavaScript", "CSS3", "Next.js"},
	}
}

// NewTestData creates a new TestData instance
func NewTestData() *TestData {
	return &TestData{}
}
