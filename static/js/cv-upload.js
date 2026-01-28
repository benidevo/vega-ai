// CV Upload functionality

// Track upload state to prevent race conditions
let isUploading = false;

// Helper function to get CSRF token from meta tag with validation
function getCSRFToken() {
    const metaTag = document.querySelector('meta[name="csrf-token"]');
    const token = metaTag ? metaTag.getAttribute('content') : '';
    if (!token) {
        console.warn('CSRF token not found - requests may fail');
    }
    return token;
}

document.addEventListener('DOMContentLoaded', async function() {
    const uploadButton = document.getElementById('cv-upload-button');
    const fileInput = document.getElementById('cv-file-input');
    const uploadStatus = document.getElementById('cv-upload-status');
    const manualFillButton = document.getElementById('manual-fill-button');
    let pdfLibLoaded = false;

    // Load PDF.js library immediately when page loads
    if (uploadButton && fileInput) {
        await loadPDFLib();

        uploadButton.addEventListener('click', () => fileInput.click());
        fileInput.addEventListener('change', async (e) => {
            const file = e.target.files[0];
            if (file) {
                handleCVUpload(file);
            }
        });
    }

    async function loadPDFLib() {
        if (pdfLibLoaded) return;

        try {
            await new Promise((resolve, reject) => {
                const script = document.createElement('script');
                script.src = 'https://cdnjs.cloudflare.com/ajax/libs/pdf.js/3.11.174/pdf.min.js';
                script.onload = resolve;
                script.onerror = reject;
                document.head.appendChild(script);
            });

            window.pdfjsLib.GlobalWorkerOptions.workerSrc =
                'https://cdnjs.cloudflare.com/ajax/libs/pdf.js/3.11.174/pdf.worker.min.js';

            pdfLibLoaded = true;
        } catch (error) {
            console.error('Failed to load PDF.js:', error);
            throw new Error('Failed to initialize PDF processing. Please try again.');
        }
    }

    if (manualFillButton) {
        manualFillButton.addEventListener('click', () => {
            const cvSection = document.getElementById('cv-upload-section');
            if (cvSection) cvSection.style.display = 'none';
        });
    }

    async function handleCVUpload(file) {
        clearError();

        // Prevent multiple simultaneous uploads
        if (isUploading) {
            return showError('Upload already in progress. Please wait.');
        }

        if (file.type !== 'application/pdf') {
            return showError('Please upload a PDF file. Other formats are not supported.');
        }

        const maxSize = 5 * 1024 * 1024; // 5MB
        if (file.size > maxSize) {
            const fileSizeMB = (file.size / (1024 * 1024)).toFixed(1);
            return showError(`File size (${fileSizeMB}MB) exceeds the 5MB limit. Please upload a smaller file.`);
        }

        if (file.size < 1024) { // Less than 1KB
            return showError('File appears to be too small to contain a valid CV.');
        }

        // Validate CSRF token before proceeding
        const csrfToken = getCSRFToken();
        if (!csrfToken) {
            return showError('Security token missing. Please refresh the page and try again.');
        }

        isUploading = true;
        uploadButton.style.display = 'none';
        uploadStatus.classList.remove('hidden');

        try {
            const text = await extractTextFromPDF(file);

            // Basic content validation
            if (!text || text.trim().length < 50) {
                return showError('The uploaded file appears to be empty or too short to contain meaningful content.');
            }

            if (text.trim().length > 50000) {
                return showError('The document is too long. Please upload a standard CV/Resume (typically 1-3 pages).');
            }

            const response = await fetch('/settings/profile/parse-cv', {
                method: 'POST',
                headers: { 
                    'Content-Type': 'application/json',
                    'X-CSRF-Token': csrfToken
                },
                body: JSON.stringify({ cv_text: text })
            });

            const result = await response.json();
            if (result.success) {
                window.location.reload();
            } else {
                throw new Error(result.message || 'Failed to parse CV');
            }
        } catch (error) {
            console.error('CV upload error:', error);
            showError(error.message || 'Failed to process CV. Please try again or fill manually.');
        } finally {
            isUploading = false;
            uploadButton.style.display = 'inline-flex';
            uploadStatus.classList.add('hidden');
            fileInput.value = '';
        }
    }

    function showError(message) {
        let errorElement = document.getElementById('cv-error-message');
        if (!errorElement) {
            errorElement = document.createElement('div');
            errorElement.id = 'cv-error-message';
            errorElement.className = 'mt-2 text-sm text-red-400 bg-red-400/10 p-3 rounded-md whitespace-pre-wrap';
            errorElement.setAttribute('aria-live', 'polite');
            uploadStatus.parentNode.appendChild(errorElement);
        }

        errorElement.textContent = message;
        setTimeout(() => errorElement?.remove(), 10000);
    }

    function clearError() {
        const errorElement = document.getElementById('cv-error-message');
        if (errorElement) {
            errorElement.remove();
        }
    }

    async function extractTextFromPDF(file) {
        const arrayBuffer = await file.arrayBuffer();
        const pdf = await window.pdfjsLib.getDocument(arrayBuffer).promise;
        let text = '';

        for (let i = 1; i <= pdf.numPages; i++) {
            const page = await pdf.getPage(i);
            const textContent = await page.getTextContent();
            const pageText = textContent.items.map(item => item.str).join(' ');
            text += pageText + '\n';
        }

        return text.trim();
    }
});
