// AI Operations Loading State Management

// Store original button content for restoration
const buttonStates = new Map();

/**
 * Handle the start of an AI operation
 * @param {HTMLElement} button The button element that triggered the operation
 * @param {string} loadingText Text to display during loading (e.g., "Analyzing...", "Generating...")
 */
window.handleAIOperationStart = function(button, loadingText) {
    const aiButtons = ['analyze-button', 'cover-letter-button', 'cv-button'];
    aiButtons.forEach(buttonId => {
        const btn = document.getElementById(buttonId);
        if (btn && !btn.disabled) {
            if (!buttonStates.has(buttonId)) {
                buttonStates.set(buttonId, {
                    html: btn.innerHTML,
                    disabled: false
                });
            }
            btn.disabled = true;
            btn.classList.add('opacity-75', 'cursor-not-allowed');
        }
    });

    // Update the clicked button's text
    const textSpan = button.querySelector('.button-text');
    if (textSpan) {
        textSpan.textContent = loadingText;
    }
};

/**
 * Handle the end of an AI operation (success or error)
 * @param {HTMLElement} button The button element that triggered the operation
 */
window.handleAIOperationEnd = function(button) {
    const aiButtons = ['analyze-button', 'cover-letter-button', 'cv-button'];
    aiButtons.forEach(buttonId => {
        const btn = document.getElementById(buttonId);
        const originalState = buttonStates.get(buttonId);
        if (btn && originalState) {
            // Use textContent instead of innerHTML when possible
            if (originalState.html && !originalState.html.includes('<')) {
                btn.textContent = originalState.html;
            } else {
                // Only use innerHTML if it contains actual HTML elements
                btn.innerHTML = originalState.html;
            }
            btn.disabled = originalState.disabled;
            btn.classList.remove('opacity-75', 'cursor-not-allowed');
            buttonStates.delete(buttonId);
        }
    });
};

/**
 * Handle AI operation errors specifically
 * @param {HTMLElement} button The button element that triggered the operation
 * @param {string} errorMessage Error message to display
 */
window.handleAIOperationError = function(button, errorMessage) {
    window.handleAIOperationEnd(button);

    if (errorMessage) {
        console.error('AI Operation Error:', errorMessage);
    }
};

// Clean up on page unload to prevent memory leaks
window.addEventListener('beforeunload', function() {
    buttonStates.clear();
});