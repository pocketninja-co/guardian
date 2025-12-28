package risk

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	
	"hipaa-app/internal/content"
)

type AuditReport struct {
	TotalFiles     int           `json:"totalFiles"`
	TotalRiskScore int           `json:"totalRiskScore"`
	PotentialLiability int       `json:"potentialLiability"`
	TopOffenders   []RiskProfile `json:"topOffenders"`
	CriticalCount  int           `json:"criticalCount"`
}

type RiskReport struct {
	RiskScore int
	Findings  []string
	IsClean   bool
}

type RiskProfile struct {
	FilePath       string `json:"filePath"`
	RiskScore      int    `json:"riskScore"`
	SSNCount       int    `json:"ssnCount"`
	HasDiagnosis   bool   `json:"hasDiagnosis"`
	RiskLabel      string `json:"riskLabel"` // "Low", "Medium", "High", "CRITICAL"
	EstimatedFine  int    `json:"estimatedFine"`
	Findings       []string `json:"findings"`
}

type RiskEngine struct {
	// HIPAA Identifier Patterns
	ssnRegex     *regexp.Regexp // #7
	ccRegex      *regexp.Regexp // PCI-DSS
	icdRegex     *regexp.Regexp
	phoneRegex   *regexp.Regexp // #4, #5
	emailRegex   *regexp.Regexp // #6
	mrnRegex     *regexp.Regexp // #8
	zipRegex     *regexp.Regexp // #2
	dateRegex    *regexp.Regexp // #3
	ipRegex      *regexp.Regexp // #15
	urlRegex     *regexp.Regexp // #14
	accountRegex *regexp.Regexp // #10
	licenseRegex *regexp.Regexp // #11
	vinRegex     *regexp.Regexp // #12
}

func NewRiskEngine() *RiskEngine {
	return &RiskEngine{
		// #7: SSN - 123-45-6789 or 123456789
		ssnRegex: regexp.MustCompile(`\b\d{3}-?\d{2}-?\d{4}\b`),
		
		// PCI-DSS: Credit Cards
		ccRegex: regexp.MustCompile(`\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`),
		
		// #4, #5: Phone/Fax - (555) 123-4567, 555-123-4567, 555.123.4567
		phoneRegex: regexp.MustCompile(`\b(?:\+?1[\s.-]?)?\(?\d{3}\)?[\s.-]?\d{3}[\s.-]?\d{4}\b`),
		
		// #6: Email addresses
		emailRegex: regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`),
		
		// #8: MRN - Medical Record Numbers (various formats)
		mrnRegex: regexp.MustCompile(`\b(?:MRN|M\.?R\.?N\.?)[:\s#]*[A-Z0-9]{6,12}\b|\b[A-Z]{2,3}\d{6,9}\b`),
		
		// #2: ZIP codes (US 5 or 9 digit)
		zipRegex: regexp.MustCompile(`\b\d{5}(?:-\d{4})?\b`),
		
		// #3: Dates - MM/DD/YYYY, MM-DD-YYYY, DOB: etc.
		dateRegex: regexp.MustCompile(`\b(?:DOB|Date of Birth|Admitted|Discharged|Born|D\.O\.B\.?)\s*:?\s*\d{1,2}[/-]\d{1,2}[/-]\d{2,4}\b|\b\d{1,2}[/-]\d{1,2}[/-]\d{2,4}\b`),
		
		// #15: IP Addresses (IPv4)
		ipRegex: regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`),
		
		// #14: URLs
		urlRegex: regexp.MustCompile(`\b(?:https?://|www\.)[-A-Za-z0-9+&@#/%?=~_|!:,.;]*[-A-Za-z0-9+&@#/%=~_|]`),
		
		// #10: Account Numbers (generic pattern)
		accountRegex: regexp.MustCompile(`\b(?:Account|Acct|Patient)\s*#?:?\s*[A-Z0-9]{6,15}\b`),
		
		// #11: License Numbers - DL, Driver License, etc.
		licenseRegex: regexp.MustCompile(`\b(?:DL|Driver'?s? License|License)\s*#?:?\s*[A-Z0-9]{6,15}\b`),
		
		// #12: VIN (Vehicle Identification Number)
		vinRegex: regexp.MustCompile(`\b[A-HJ-NPR-Z0-9]{17}\b`),
		
		icdRegex: regexp.MustCompile(`[A-Z]\d{2}\.[A-Z0-9]{1,2}`),
	}
}

// AnalyzeDirectory recursively scans a directory and returns an AuditReport
func (e *RiskEngine) AnalyzeDirectory(ctx context.Context, rootPath string, progressCallback func(path string)) (AuditReport, error) {
	report := AuditReport{
		TopOffenders: []RiskProfile{},
	}

	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		if err != nil {
			return nil // Skip errors accessing files
		}
		if d.IsDir() {
			return nil
		}
		
		// Only scan supported file types
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".txt", ".csv", ".log", ".md", ".json", ".xml", ".html", ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".rtf",
			".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".tif":
			// Allowed
		default:
			return nil
		}

		if progressCallback != nil {
			progressCallback(path)
		}

		report.TotalFiles++
		
		profile, _ := e.AnalyzeFileRisk(path)
		if profile.RiskScore > 0 {
			report.TotalRiskScore += profile.RiskScore
			report.PotentialLiability += profile.EstimatedFine
			report.TopOffenders = append(report.TopOffenders, profile)
			if profile.RiskLabel == "CRITICAL" {
				report.CriticalCount++
			}
		}

		return nil
	})

	return report, err
}


func (e *RiskEngine) AnalyzeFileRisk(path string) (RiskProfile, error) {
	// 1. Text Extraction (Supports PDF, DOCX, XLSX, etc.)
	text, err := content.ExtractText(path)
	if err != nil {
		return RiskProfile{FilePath: path}, err
	}

	profile := RiskProfile{
		FilePath: path,
		Findings: []string{},
	}

	// 2. AI Classification (Offline Naive Bayes)
	classifier := NewClassifier()
	category, confidence := classifier.Classify(text)
	if confidence > 60 && category != TypeGeneric {
		profile.Findings = append(profile.Findings, fmt.Sprintf("AI Analysis: %d%% likelihood of being %s Document", int(confidence), category))
	}

	// 3. Scan Lines (from memory buffer)
	scanner := bufio.NewScanner(strings.NewReader(text))
	
	sensitiveKeywords := []string{"hiv", "cancer", "psychotherapy", "suicide", "minor", "diagnosis", "patient"}
	
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		
		// Hard Risks (Regex Detection for all HIPAA PHI)
		
		// SSN (#7)
		ssns := e.ssnRegex.FindAllString(line, -1)
		if len(ssns) > 0 {
			profile.SSNCount += len(ssns)
			profile.Findings = append(profile.Findings, fmt.Sprintf("Line %d: %d SSN(s) found", lineNum, len(ssns)))
		}

		// Credit Cards (PCI-DSS)
		ccs := e.ccRegex.FindAllString(line, -1)
		if len(ccs) > 0 {
			profile.RiskScore += len(ccs) * 10 
			profile.Findings = append(profile.Findings, fmt.Sprintf("Line %d: Credit Card found", lineNum))
		}
		
		// Phone/Fax Numbers (#4, #5)
		phones := e.phoneRegex.FindAllString(line, -1)
		if len(phones) > 0 {
			profile.RiskScore += len(phones) * 5
			profile.Findings = append(profile.Findings, fmt.Sprintf("Line %d: %d Phone/Fax number(s) found", lineNum, len(phones)))
		}
		
		// Email Addresses (#6)
		emails := e.emailRegex.FindAllString(line, -1)
		if len(emails) > 0 {
			profile.RiskScore += len(emails) * 5
			profile.Findings = append(profile.Findings, fmt.Sprintf("Line %d: %d Email(s) found", lineNum, len(emails)))
		}
		
		// Medical Record Numbers (#8)
		mrns := e.mrnRegex.FindAllString(line, -1)
		if len(mrns) > 0 {
			profile.RiskScore += len(mrns) * 15
			profile.Findings = append(profile.Findings, fmt.Sprintf("Line %d: Medical Record Number found", lineNum))
		}
		
		// Dates (DOB, Admission, etc.) (#3)
		dates := e.dateRegex.FindAllString(line, -1)
		if len(dates) > 0 {
			profile.RiskScore += len(dates) * 10
			profile.Findings = append(profile.Findings, fmt.Sprintf("Line %d: %d Date(s) found (DOB/Admission/Discharge)", lineNum, len(dates)))
		}
		
		// IP Addresses (#15)
		ips := e.ipRegex.FindAllString(line, -1)
		if len(ips) > 0 {
			profile.RiskScore += len(ips) * 3
			profile.Findings = append(profile.Findings, fmt.Sprintf("Line %d: IP Address found", lineNum))
		}
		
		// URLs (#14)
		urls := e.urlRegex.FindAllString(line, -1)
		if len(urls) > 0 {
			profile.RiskScore += len(urls) * 5
			profile.Findings = append(profile.Findings, fmt.Sprintf("Line %d: URL found", lineNum))
		}
		
		// Account Numbers (#10)
		accounts := e.accountRegex.FindAllString(line, -1)
		if len(accounts) > 0 {
			profile.RiskScore += len(accounts) * 10
			profile.Findings = append(profile.Findings, fmt.Sprintf("Line %d: Account Number found", lineNum))
		}
		
		// License Numbers (#11)
		licenses := e.licenseRegex.FindAllString(line, -1)
		if len(licenses) > 0 {
			profile.RiskScore += len(licenses) * 8
			profile.Findings = append(profile.Findings, fmt.Sprintf("Line %d: License/ID Number found", lineNum))
		}
		
		// VIN (#12)
		vins := e.vinRegex.FindAllString(line, -1)
		if len(vins) > 0 {
			profile.RiskScore += len(vins) * 8
			profile.Findings = append(profile.Findings, fmt.Sprintf("Line %d: Vehicle ID found", lineNum))
		}
		
		// ZIP Codes (#2)
		zips := e.zipRegex.FindAllString(line, -1)
		if len(zips) > 0 {
			profile.RiskScore += len(zips) * 3
			profile.Findings = append(profile.Findings, fmt.Sprintf("Line %d: ZIP Code found", lineNum))
		}

		// Soft Risks (Context)
		lowerLine := strings.ToLower(line)
		for _, kw := range sensitiveKeywords {
			if strings.Contains(lowerLine, kw) {
				profile.HasDiagnosis = true
				break
			}
		}
	}

	// 4. Scoring Logic (Boosted by AI)
	score := profile.SSNCount * 10
	
	if profile.HasDiagnosis {
		score += 50
		profile.Findings = append(profile.Findings, "Contains Sensitive Medical Keywords")
	}

	// AI Boost: If it looks like a Medical Doc AND has SSNs, it's definitely critical
	if category == TypeMedical && confidence > 80 && profile.SSNCount > 0 {
		score += 300 // Massive boost
		profile.Findings = append(profile.Findings, "CRITICAL: AI confirmed Medical Record with SSN Exposure")
	}

	// Filename Context Penalty
	lowercasePath := strings.ToLower(filepath.Base(path))
	if (strings.Contains(lowercasePath, "marketing") || strings.Contains(lowercasePath, "public")) && profile.SSNCount > 0 {
		score += 200
		profile.Findings = append(profile.Findings, "CRITICAL: Sensitive data in Marketing/Public file")
	}

	// Final Labeling
	if score > 100 {
		profile.RiskScore = 100 
		profile.RiskLabel = "CRITICAL"
	} else if score >= 50 {
		profile.RiskScore = score
		profile.RiskLabel = "High"
	} else if score > 0 {
		profile.RiskScore = score
		profile.RiskLabel = "Low"
	} else {
		profile.RiskLabel = "Safe"
	}

	profile.EstimatedFine = (profile.SSNCount * 100) + (profile.RiskScore * 50) 

	return profile, nil
}

// Keep the old method for single file compatibility if needed, or deprecate
// We update it to strict signature for older code but redirect to new logic
func (e *RiskEngine) AnalyzeFile(path string) (RiskReport, error) {
	profile, err := e.AnalyzeFileRisk(path)
	return RiskReport{
		RiskScore: profile.RiskScore,
		Findings:  profile.Findings,
		IsClean:   profile.RiskScore == 0,
	}, err
}
// RedactContent replaces all HIPAA PHI identifiers with placeholders
func (e *RiskEngine) RedactContent(content []byte) []byte {
	text := string(content)
	
	// Redact in order of specificity (most specific first)
	text = e.ssnRegex.ReplaceAllString(text, "[REDACTED-SSN]")
	text = e.ccRegex.ReplaceAllString(text, "[REDACTED-CC]")
	text = e.phoneRegex.ReplaceAllString(text, "[REDACTED-PHONE]")
	text = e.emailRegex.ReplaceAllString(text, "[REDACTED-EMAIL]")
	text = e.mrnRegex.ReplaceAllString(text, "[REDACTED-MRN]")
	text = e.dateRegex.ReplaceAllString(text, "[REDACTED-DATE]")
	text = e.ipRegex.ReplaceAllString(text, "[REDACTED-IP]")
	text = e.urlRegex.ReplaceAllString(text, "[REDACTED-URL]")
	text = e.accountRegex.ReplaceAllString(text, "[REDACTED-ACCOUNT]")
	text = e.licenseRegex.ReplaceAllString(text, "[REDACTED-LICENSE]")
	text = e.vinRegex.ReplaceAllString(text, "[REDACTED-VIN]")
	text = e.zipRegex.ReplaceAllString(text, "[REDACTED-ZIP]")

	return []byte(text)
}

// ExtractText is a wrapper to expose content extraction to the App layer
func (e *RiskEngine) ExtractText(path string) (string, error) {
	return content.ExtractText(path)
}
