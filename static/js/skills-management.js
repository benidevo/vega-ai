// Common skills suggestions organized by category
const skillSuggestions = {
    'Technical Skills': ['JavaScript', 'Python', 'Java', 'SQL', 'HTML', 'CSS', 'React', 'Excel', 'PowerPoint', 'Word'],
    'Data & Analytics': ['Excel', 'SQL', 'Tableau', 'Power BI', 'Google Analytics', 'Data Analysis', 'Statistics', 'Reporting', 'Dashboard Creation'],
    'Marketing & Sales': ['Social Media Marketing', 'Content Marketing', 'SEO', 'Google Ads', 'Email Marketing', 'Sales', 'Lead Generation', 'CRM', 'Customer Service'],
    'Design & Creative': ['Photoshop', 'Illustrator', 'Figma', 'Canva', 'InDesign', 'UI/UX Design', 'Graphic Design', 'Video Editing', 'Photography'],
    'Business & Management': ['Project Management', 'Leadership', 'Team Management', 'Strategic Planning', 'Business Development', 'Negotiation', 'Budgeting', 'Operations'],
    'Finance & Accounting': ['Financial Analysis', 'Accounting', 'QuickBooks', 'Excel', 'Budgeting', 'Forecasting', 'Tax Preparation', 'Audit', 'Financial Planning'],
    'Healthcare & Medical': ['Patient Care', 'Medical Records', 'Healthcare Administration', 'Clinical Research', 'Nursing', 'Medical Coding', 'HIPAA Compliance'],
    'Education & Training': ['Teaching', 'Curriculum Development', 'Training', 'Instructional Design', 'E-learning', 'Mentoring', 'Assessment', 'Classroom Management'],
    'Human Resources': ['Recruitment', 'Employee Relations', 'Performance Management', 'Training & Development', 'HR Policies', 'Compensation', 'Benefits Administration'],
    'Legal & Compliance': ['Legal Research', 'Contract Review', 'Compliance', 'Risk Management', 'Litigation Support', 'Regulatory Affairs', 'Documentation'],
    'Manufacturing & Operations': ['Quality Control', 'Lean Manufacturing', 'Six Sigma', 'Supply Chain', 'Inventory Management', 'Production Planning', 'Safety Compliance'],
    'Customer Service': ['Customer Support', 'Problem Solving', 'Communication', 'Conflict Resolution', 'Phone Support', 'Chat Support', 'Ticketing Systems'],
    'Retail & Hospitality': ['Customer Service', 'Sales', 'Inventory Management', 'POS Systems', 'Food Service', 'Event Planning', 'Guest Relations'],
    'Administrative': ['Data Entry', 'Scheduling', 'File Management', 'Phone Support', 'Email Management', 'Calendar Management', 'Travel Coordination'],
    'Communication': ['Public Speaking', 'Writing', 'Presentation Skills', 'Social Media', 'Content Creation', 'Translation', 'Technical Writing'],
    'Soft Skills': ['Leadership', 'Communication', 'Problem Solving', 'Teamwork', 'Time Management', 'Critical Thinking', 'Adaptability', 'Creativity']
};

function addSkill(skillText = null) {
    const input = document.getElementById('skills-input');
    const skill = (skillText || input.value).trim();

    if (skill && !hasSkill(skill)) {
        createSkillTag(skill);
        input.value = '';
        updateSkills();
        hideSkillSuggestions();
    }
}

function hasSkill(skill) {
    const tags = document.querySelectorAll('.skill-tag');
    return Array.from(tags).some(tag =>
        tag.textContent.trim().toLowerCase() === skill.toLowerCase()
    );
}

function createSkillTag(skill) {
    const container = document.getElementById('skills-tags');
    const tag = document.createElement('span');
    tag.className = 'skill-tag inline-flex items-center px-1 py-0.5 rounded text-xs font-medium bg-primary bg-opacity-20 text-primary border border-primary border-opacity-30';

    const skillText = document.createTextNode(skill);
    tag.appendChild(skillText);

    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'ml-1 inline-flex items-center justify-center w-4 h-4 text-primary hover:text-red-400 focus:outline-none';
    button.onclick = function() { removeSkillTag(this); };

    button.innerHTML = `
        <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
        </svg>
    `;

    tag.appendChild(button);
    container.appendChild(tag);
}

function removeSkillTag(button) {
    const tag = button.closest('.skill-tag');
    if (tag) {
        tag.remove();
        updateSkills();
    }
}

function updateSkills() {
    const tags = document.querySelectorAll('.skill-tag');
    const skills = Array.from(tags).map(tag => tag.textContent.trim());
    document.getElementById('skills').value = skills.join(', ');
}

function initializeSkillSuggestions() {
    const input = document.getElementById('skills-input');
    if (!input) return;

    input.addEventListener('input', handleSkillInput);
    input.addEventListener('focus', handleSkillFocus);
    input.addEventListener('blur', handleSkillBlur);

    // Create suggestions container
    const container = input.parentElement;
    const suggestionsDiv = document.createElement('div');
    suggestionsDiv.id = 'skill-suggestions';
    suggestionsDiv.className = 'absolute z-10 w-full bg-slate-700 border border-slate-600 rounded-md shadow-lg max-h-60 overflow-y-auto hidden';
    suggestionsDiv.style.top = '100%';
    suggestionsDiv.style.left = '0';

    // Make input container relative
    container.style.position = 'relative';
    container.appendChild(suggestionsDiv);
}

function handleSkillInput(event) {
    const input = event.target;
    const query = input.value.toLowerCase().trim();

    if (query.length < 1) {
        hideSkillSuggestions();
        return;
    }

    showSkillSuggestions(query);
}

function handleSkillFocus(event) {
    const query = event.target.value.toLowerCase().trim();
    if (query.length >= 1) {
        showSkillSuggestions(query);
    }
}

function handleSkillBlur() {
    // Delay hiding to allow clicking on suggestions
    setTimeout(() => {
        hideSkillSuggestions();
    }, 200);
}

function showSkillSuggestions(query) {
    const suggestionsDiv = document.getElementById('skill-suggestions');
    if (!suggestionsDiv) return;

    const matches = getMatchingSkills(query);

    if (matches.length === 0) {
        hideSkillSuggestions();
        return;
    }

    // Build suggestions HTML
    let html = '';
    let currentCategory = '';

    matches.forEach(({skill, category}) => {
        if (category !== currentCategory) {
            if (currentCategory) html += '</div>';
            html += `<div class="category-group">
                        <div class="px-3 py-2 text-xs font-medium text-gray-400 bg-slate-800 border-b border-slate-600">
                            ${category}
                        </div>`;
            currentCategory = category;
        }

        const escapedSkill = skill.replace(/'/g, "\\'").replace(/</g, "&lt;").replace(/>/g, "&gt;");
        html += `<div class="skill-suggestion px-3 py-2 text-sm text-gray-200 hover:bg-slate-600 cursor-pointer"
                      onclick="selectSkill('${escapedSkill}')">${escapedSkill}</div>`;
    });

    if (currentCategory) html += '</div>';

    suggestionsDiv.textContent = '';
    const tempDiv = document.createElement('div');
    tempDiv.innerHTML = html;
    
    while (tempDiv.firstChild) {
        suggestionsDiv.appendChild(tempDiv.firstChild);
    }
    suggestionsDiv.classList.remove('hidden');
}

function hideSkillSuggestions() {
    const suggestionsDiv = document.getElementById('skill-suggestions');
    if (suggestionsDiv) {
        suggestionsDiv.classList.add('hidden');
    }
}

function getMatchingSkills(query) {
    const matches = [];

    Object.keys(skillSuggestions).forEach(category => {
        skillSuggestions[category].forEach(skill => {
            if (skill.toLowerCase().includes(query) && !hasSkill(skill)) {
                matches.push({skill, category});
            }
        });
    });

    // Sort by relevance (starts with query first, then contains)
    matches.sort((a, b) => {
        const aStarts = a.skill.toLowerCase().startsWith(query);
        const bStarts = b.skill.toLowerCase().startsWith(query);

        if (aStarts && !bStarts) return -1;
        if (!aStarts && bStarts) return 1;

        return a.skill.localeCompare(b.skill);
    });

    return matches.slice(0, 20); // Limit to 20 suggestions
}

function selectSkill(skill) {
    addSkill(skill);
    document.getElementById('skills-input').focus();
}

// Initialize when DOM is ready
document.addEventListener('DOMContentLoaded', initializeSkillSuggestions);
