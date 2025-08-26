
window.PDFGenerator = (function() {
  'use strict';

  async function downloadDocument(docId, docType) {
    try {
      const response = await fetch(`/documents/${docId}/export`);
      if (!response.ok) {
        throw new Error('Failed to fetch document');
      }
      
      const data = await response.json();
      
      if (docType === 'resume') {
        let cvData;
        try {
          cvData = typeof data.content === 'string' ? JSON.parse(data.content) : data.content;
        } catch (e) {
          console.error('Invalid JSON in resume content:', e);
          throw new Error('Invalid resume format');
        }
        await generateResumePDFFromData(cvData, data.jobId, data.jobTitle, data.companyName);
      } else {
        let personalInfo = null;
        
        try {
          if (data.personalInfo) {
            personalInfo = data.personalInfo;
          }
        } catch (e) {
          console.warn('Error processing personal info:', e);
        }
        
        await generateCoverLetterPDFFromText(data.content, data.jobId, personalInfo, data.jobTitle, data.companyName);
      }
    } catch (error) {
      console.error('Download error:', error);
      if (typeof window.showNotification === 'function') {
        window.showNotification('Failed to download document. Please try again.', 'error', 'Download Failed');
      }
    }
  }

  async function generateResumePDFFromData(cvData, jobId, jobTitle, companyName) {
    if (typeof window.jspdf === 'undefined' || !window.jspdf.jsPDF) {
      throw new Error('jsPDF library not loaded');
    }
    
    if (typeof cvData === 'string') {
      try {
        cvData = JSON.parse(cvData);
      } catch (e) {
        console.error('Failed to parse CV data:', e);
        throw new Error('Invalid CV data format');
      }
    }

    const { jsPDF } = window.jspdf;
    const doc = new jsPDF({
      unit: 'pt',
      format: 'a4',
      orientation: 'portrait'
    });

    const fontFamily = 'helvetica';
    const margin = 30;
    const pageWidth = doc.internal.pageSize.getWidth();
    const pageHeight = doc.internal.pageSize.getHeight();
    const contentWidth = pageWidth - (2 * margin);
    let yPos = margin;
    const lineHeight = 14;

    try {
      doc.setFont(fontFamily, 'normal');
      doc.setFontSize(12);
      doc.getTextWidth('Test');
    } catch (metricError) {
      console.error('Font initialization failed:', metricError);
    }

    doc.setProperties({
      title: 'Resume',
      subject: 'Professional Resume',
      author: `${cvData.personalInfo?.firstName || ''} ${cvData.personalInfo?.lastName || ''}`,
      keywords: 'resume, cv, professional'
    });


    if (cvData.personalInfo) {
      const p = cvData.personalInfo;
      
      const fullName = `${p.firstName || ''} ${p.lastName || ''}`.trim();
      if (fullName) {
        doc.setFont(fontFamily, 'bold');
        doc.setFontSize(18);
        doc.setTextColor(0, 0, 0);
        doc.text(fullName, margin, yPos);
        yPos += 1;
      }
      
      if (p.title) {
        yPos += lineHeight;
        doc.setFont(fontFamily, 'normal');
        doc.setFontSize(14);
        doc.setTextColor(90, 90, 90);
        doc.text(p.title, margin, yPos);
        yPos += 2;
      }
      
      const contactParts = [];
      if (p.location) contactParts.push(p.location);
      if (p.email) contactParts.push(p.email);
      if (p.phone) contactParts.push(p.phone);
      if (p.linkedin) contactParts.push(p.linkedin);
      
      if (contactParts.length > 0) {
        yPos += lineHeight;
        doc.setFont(fontFamily, 'normal');
        doc.setFontSize(10);
        doc.setTextColor(100, 100, 100);
        
        let xPos = margin;
        contactParts.forEach((part, index) => {
          if (index > 0) {
            doc.text(' | ', xPos, yPos);
            xPos += doc.getTextWidth(' | ');
          }
          doc.text(part, xPos, yPos);
          xPos += doc.getTextWidth(part);
        });
        yPos += 18;
      }
      
      if (p.summary) {
        yPos += 8;
        doc.setFont(fontFamily, 'bold');
        doc.setFontSize(12);
        doc.setTextColor(0, 0, 0);
        doc.text('PROFESSIONAL SUMMARY', margin, yPos);
        yPos += 3;
        
        doc.setDrawColor(220, 220, 220);
        doc.setLineWidth(0.5);
        doc.line(margin, yPos, pageWidth - margin, yPos);
        yPos += 12;
        
        doc.setFont(fontFamily, 'normal');
        doc.setFontSize(10);
        doc.setTextColor(50, 50, 50);
        const summaryLines = doc.splitTextToSize(p.summary, contentWidth);
        summaryLines.forEach(line => {
          if (yPos > pageHeight - margin - 20) {
            doc.addPage();
            yPos = margin;
          }
          doc.text(line, margin, yPos);
          yPos += 10 * 1.5;
        });
        yPos += 12;
      }
    }

    if (cvData.skills && cvData.skills.length > 0) {
      yPos += 8;
      doc.setFont(fontFamily, 'bold');
      doc.setFontSize(12);
      doc.setTextColor(0, 0, 0);
      doc.text('SKILLS', margin, yPos);
      yPos += 3;
      
      doc.setDrawColor(220, 220, 220);
      doc.setLineWidth(0.5);
      doc.line(margin, yPos, pageWidth - margin, yPos);
      yPos += 12;
      
      doc.setFont(fontFamily, 'normal');
      doc.setFontSize(10);
      doc.setTextColor(50, 50, 50);
      const skillsText = cvData.skills.join(', ');
      const skillLines = doc.splitTextToSize(skillsText, contentWidth);
      skillLines.forEach(line => {
        if (yPos > pageHeight - margin - 20) {
          doc.addPage();
          yPos = margin;
        }
        doc.text(line, margin, yPos);
        yPos += 10 * 1.5;
      });
      yPos += 8;
    }

    if (cvData.workExperience && cvData.workExperience.length > 0) {
      yPos += 8;
      doc.setFont(fontFamily, 'bold');
      doc.setFontSize(12);
      doc.setTextColor(0, 0, 0);
      doc.text('WORK EXPERIENCE', margin, yPos);
      yPos += 3;
      
      doc.setDrawColor(220, 220, 220);
      doc.setLineWidth(0.5);
      doc.line(margin, yPos, pageWidth - margin, yPos);
      yPos += 12;
      
      cvData.workExperience.forEach((exp, index) => {
        doc.setFont(fontFamily, 'bold');
        doc.setFontSize(10);
        doc.setTextColor(0, 0, 0);
        doc.text(exp.title || 'Position', margin, yPos);
        
        if (exp.startDate || exp.endDate) {
          doc.setFont(fontFamily, 'normal');
          doc.setFontSize(10);
          doc.setTextColor(100, 100, 100);
          const dateText = `${exp.startDate || ''} - ${exp.endDate || ''}`;
          const dateWidth = doc.getTextWidth(dateText);
          doc.text(dateText, pageWidth - margin - dateWidth, yPos);
        }
        yPos += lineHeight * 0.9;
        
        if (exp.company) {
          doc.setFont(fontFamily, 'normal');
          doc.setFontSize(10);
          doc.setTextColor(100, 100, 100);
          
          if (exp.location) {
            doc.text(exp.company, margin, yPos);
            const companyWidth = doc.getTextWidth(exp.company);
            
            doc.text(' – ', margin + companyWidth, yPos);
            
            doc.setFont(fontFamily, 'italic');
            doc.text(exp.location, margin + companyWidth + doc.getTextWidth(' – '), yPos);
          } else {
            doc.text(exp.company, margin, yPos);
          }
          yPos += lineHeight * 0.9;
        }
        
        if (exp.description) {
          yPos += 2;
          doc.setFont(fontFamily, 'normal');
          doc.setFontSize(10);
          doc.setTextColor(50, 50, 50);
          
          const lines = exp.description.split('\n').filter(line => line.trim());
          lines.forEach(line => {
            const trimmedLine = line.trim();
            if (trimmedLine.startsWith('•') || trimmedLine.startsWith('-') || trimmedLine.startsWith('*')) {
              const bulletText = trimmedLine.substring(1).trim();
              const wrappedLines = doc.splitTextToSize(bulletText, contentWidth - 15);
              
              doc.text('•', margin, yPos);
              
              wrappedLines.forEach((wLine, idx) => {
                if (yPos > pageHeight - margin - 20) {
                  doc.addPage();
                  yPos = margin;
                }
                doc.text(wLine, margin + 10, yPos);
                yPos += 10 * 1.5;
              });
            } else {
              const wrappedLines = doc.splitTextToSize(trimmedLine, contentWidth);
              wrappedLines.forEach(wLine => {
                if (yPos > pageHeight - margin - 20) {
                  doc.addPage();
                  yPos = margin;
                }
                doc.text(wLine, margin, yPos);
                yPos += 10 * 1.5;
              });
            }
          });
        }
        
        if (index < cvData.workExperience.length - 1) {
          yPos += 8;
        }
      });
      yPos += 10;
    }

    if (cvData.education && cvData.education.length > 0) {
      yPos += 4;
      doc.setFont(fontFamily, 'bold');
      doc.setFontSize(12);
      doc.setTextColor(0, 0, 0);
      doc.text('EDUCATION', margin, yPos);
      yPos += 3;
      
      doc.setDrawColor(220, 220, 220);
      doc.setLineWidth(0.5);
      doc.line(margin, yPos, pageWidth - margin, yPos);
      yPos += 12;
      
      cvData.education.forEach((edu, index) => {
        doc.setFont(fontFamily, 'bold');
        doc.setFontSize(10);
        doc.setTextColor(0, 0, 0);
        const degreeText = edu.fieldOfStudy ? 
          `${edu.degree || ''} in ${edu.fieldOfStudy}` : 
          (edu.degree || 'Degree');
        doc.text(degreeText, margin, yPos);
        
        if (edu.endDate) {
          doc.setFont(fontFamily, 'normal');
          doc.setFontSize(10);
          doc.setTextColor(100, 100, 100);
          const dateWidth = doc.getTextWidth(edu.endDate);
          doc.text(edu.endDate, pageWidth - margin - dateWidth, yPos);
        }
        yPos += lineHeight * 0.9;
        
        if (edu.institution) {
          doc.setFont(fontFamily, 'normal');
          doc.setFontSize(10);
          doc.setTextColor(100, 100, 100);
          doc.text(edu.institution, margin, yPos);
          yPos += lineHeight;
        }
        
        if (index < cvData.education.length - 1) {
          yPos += 6;
        }
      });
      yPos += 10;
    }

    if (cvData.certifications && cvData.certifications.length > 0) {
      yPos += 4;
      doc.setFont(fontFamily, 'bold');
      doc.setFontSize(12);
      doc.setTextColor(0, 0, 0);
      doc.text('CERTIFICATIONS', margin, yPos);
      yPos += 3;
      
      doc.setDrawColor(220, 220, 220);
      doc.setLineWidth(0.5);
      doc.line(margin, yPos, pageWidth - margin, yPos);
      yPos += 12;
      
      cvData.certifications.forEach((cert, index) => {
        doc.setFont(fontFamily, 'bold');
        doc.setFontSize(11);
        doc.setTextColor(0, 0, 0);
        doc.text(cert.name || 'Certification', margin, yPos);
        
        if (cert.issueDate) {
          doc.setFont(fontFamily, 'normal');
          doc.setFontSize(10);
          doc.setTextColor(80, 80, 80);
          const dateWidth = doc.getTextWidth(cert.issueDate);
          doc.text(cert.issueDate, pageWidth - margin - dateWidth, yPos);
        }
        yPos += 14;
        
        if (cert.issuingOrg) {
          doc.setFont(fontFamily, 'normal');
          doc.setFontSize(10);
          doc.setTextColor(60, 60, 60);
          doc.text(cert.issuingOrg, margin, yPos);
          yPos += 14;
        }
        
        if (index < cvData.certifications.length - 1) {
          yPos += 6;
        }
      });
    }

    let filename = 'resume.pdf';
    if (companyName && jobTitle) {
      const sanitizedCompany = companyName.replace(/[^a-zA-Z0-9\s]/g, '').replace(/\s+/g, '-').toLowerCase();
      const sanitizedTitle = jobTitle.replace(/[^a-zA-Z0-9\s]/g, '').replace(/\s+/g, '-').toLowerCase();
      filename = `${sanitizedCompany}-${sanitizedTitle}-resume.pdf`;
    } else if (jobId) {
      filename = `resume_${jobId}.pdf`;
    }
    doc.save(filename);
  }

  async function generateCoverLetterPDFFromText(content, jobId, personalInfo = null, jobTitle, companyName) {
    if (typeof window.jspdf === 'undefined' || !window.jspdf.jsPDF) {
      throw new Error('jsPDF library not loaded');
    }

    const { jsPDF } = window.jspdf;
    const doc = new jsPDF({
      unit: 'pt',
      format: 'a4',
      orientation: 'portrait'
    });

    const fontFamily = 'helvetica';
    const margin = 30;
    const pageWidth = doc.internal.pageSize.getWidth();
    const pageHeight = doc.internal.pageSize.getHeight();
    const contentWidth = pageWidth - (2 * margin);
    let yPos = margin;
    const lineHeight = 16;

    try {
      doc.setFont(fontFamily, 'normal');
      doc.setFontSize(12);
      doc.getTextWidth('Test');
    } catch (metricError) {
      console.error('Font initialization failed:', metricError);
    }

    doc.setProperties({
      title: 'Cover Letter',
      subject: 'Professional Cover Letter',
      author: personalInfo ? `${personalInfo.firstName || ''} ${personalInfo.lastName || ''}` : 'Cover Letter',
      keywords: 'cover letter, job application'
    });

    function addText(text, fontSize, options = {}) {
      const { isBold = false, isItalic = false, color = [0, 0, 0] } = options;
      
      doc.setFontSize(fontSize);
      
      let fontStyle = 'normal';
      if (isBold && isItalic) fontStyle = 'bolditalic';
      else if (isBold) fontStyle = 'bold';
      else if (isItalic) fontStyle = 'italic';
      
      doc.setFont(fontFamily, fontStyle);
      doc.setTextColor(color[0], color[1], color[2]);
      
      if (!text || typeof text !== 'string') return;
      
      const lines = doc.splitTextToSize(text, contentWidth);
      lines.forEach(line => {
        if (yPos > pageHeight - margin - 20) {
          doc.addPage();
          yPos = margin;
        }
        doc.text(line, margin, yPos);
        yPos += fontSize * 1.5;
      });
    }

    if (personalInfo) {
      if (personalInfo.firstName || personalInfo.lastName) {
        const fullName = `${personalInfo.firstName || ''} ${personalInfo.lastName || ''}`.trim();
        addText(fullName, 18, { isBold: true });
        yPos += 1;
      }
      
      if (personalInfo.title) {
        addText(personalInfo.title, 14, { color: [60, 60, 60] });
        yPos += 6;
      }
      
      const contactParts = [];
      if (personalInfo.location) contactParts.push(personalInfo.location);
      if (personalInfo.email) contactParts.push(personalInfo.email);
      if (personalInfo.phone) contactParts.push(personalInfo.phone);
      if (personalInfo.linkedin) contactParts.push(personalInfo.linkedin);
      
      if (contactParts.length > 0) {
        addText(contactParts.join(' | '), 10, { color: [80, 80, 80] });
        yPos += 12;
        
        doc.setDrawColor(200, 200, 200);
        doc.setLineWidth(0.5);
        doc.line(margin, yPos, pageWidth - margin, yPos);
        yPos += 24;
      }
    }

    const paragraphs = content.trim().split(/\n\n+/);
    
    paragraphs.forEach((paragraph, index) => {
      if (paragraph.trim()) {
        if (index === 0 && paragraph.match(/^[A-Za-z]+ \d{1,2}, \d{4}$/)) {
          doc.setFontSize(11);
          doc.setFont(fontFamily, 'normal');
          doc.setTextColor(60, 60, 60);
          const dateWidth = doc.getTextWidth(paragraph.trim());
          doc.text(paragraph.trim(), pageWidth - margin - dateWidth, yPos);
          doc.setTextColor(0, 0, 0);
          yPos += 20;
        } else {
          addText(paragraph.trim(), 10, { color: [40, 40, 40] });
          yPos += 18;
        }
      }
    });

    let filename = 'cover-letter.pdf';
    if (companyName && jobTitle) {
      const sanitizedCompany = companyName.replace(/[^a-zA-Z0-9\s]/g, '').replace(/\s+/g, '-').toLowerCase();
      const sanitizedTitle = jobTitle.replace(/[^a-zA-Z0-9\s]/g, '').replace(/\s+/g, '-').toLowerCase();
      filename = `${sanitizedCompany}-${sanitizedTitle}-cover-letter.pdf`;
    } else if (jobId) {
      filename = `cover_letter_${jobId}.pdf`;
    }
    doc.save(filename);
  }

  return {
    downloadDocument: downloadDocument,
    generateResumePDFFromData: generateResumePDFFromData,
    generateCoverLetterPDFFromText: generateCoverLetterPDFFromText
  };
})();