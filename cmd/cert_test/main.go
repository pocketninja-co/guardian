package main

import (
	"fmt"
	"os"
	
	"hipaa-app/internal/pdf"
	"hipaa-app/internal/storage"
)

// Simple test to generate a sample certificate
func main() {
	pdfService := pdf.NewPDFService()
	
	// Generate a sample certificate (100 files scanned, all clean)
	hostname, _ := os.Hostname()
	
	// Create sample audit history
	auditHistory := []storage.AuditEntry{
		{
			Timestamp:  "2025-12-27T18:30:00Z",
			TotalFiles: 100,
			RiskScore:  0,
			User:       hostname,
			Status:     "PASSED",
		},
		{
			Timestamp:  "2025-12-26T18:30:00Z",
			TotalFiles: 95,
			RiskScore:  0,
			User:       hostname,
			Status:     "PASSED",
		},
	}
	
	path, err := pdfService.GenerateComplianceCertificate(100, hostname, auditHistory)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("✅ Certificate generated: %s\n", path)
}
