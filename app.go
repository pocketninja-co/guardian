package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	
	"hipaa-app/internal/pdf"
	"hipaa-app/internal/risk"
	"hipaa-app/internal/scheduler"
	"hipaa-app/internal/storage"
	
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx        context.Context
	cancelScan context.CancelFunc
	riskEngine *risk.RiskEngine
	pdfService *pdf.PDFService
	store      *storage.Store
	scheduler  *scheduler.Scheduler
}

// NewApp creates a new App application struct
func NewApp() *App {
	store, _ := storage.NewStore()
	engine := risk.NewRiskEngine()
	
	app := &App{
		riskEngine: engine,
		pdfService: pdf.NewPDFService(),
		store:      store,
	}
	
	return app
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	
	// Initialize scheduler with event handler
	eventHandler := func(event string, data interface{}) {
		runtime.EventsEmit(ctx, event, data)
		
		// System notifications
		if event == "scan:scheduled:complete" {
			if dataMap, ok := data.(map[string]interface{}); ok {
				if status, ok := dataMap["status"].(string); ok {
					if status == "FAILED" {
						runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
							Type:    runtime.WarningDialog,
							Title:   "Scheduled Scan Alert",
							Message: "Automated scan detected HIPAA risks. Review required.",
						})
					}
				}
			}
		}
	}
	
	a.scheduler = scheduler.NewScheduler(a.riskEngine, a.pdfService, a.store, eventHandler)
	a.scheduler.Start(ctx)
}

// shutdown is called when the app is closing
func (a *App) shutdown(ctx context.Context) {
	if a.scheduler != nil {
		a.scheduler.Stop()
	}
}

type ScanResult struct {
	RiskScore   int      `json:"riskScore"`
	Findings    []string `json:"findings"`
	IsClean     bool     `json:"isClean"`
	Certificate string   `json:"certificate,omitempty"`
}

// AnalyzeFile is exposed to the frontend
func (a *App) AnalyzeFile(path string) (ScanResult, error) {
	runtime.EventsEmit(a.ctx, "scan:start", path)
	
	report, err := a.riskEngine.AnalyzeFile(path)
	if err != nil {
		runtime.EventsEmit(a.ctx, "scan:error", err.Error())
		return ScanResult{}, err
	}
	
	result := ScanResult{
		RiskScore: report.RiskScore,
		Findings:  report.Findings,
		IsClean:   report.IsClean,
	}
	
	if report.IsClean {
		runtime.EventsEmit(a.ctx, "scan:generating_cert", path)
		// runtime.SystemTraySetIcon(a.ctx, iconGreen) // implementation requires loading the icon bytes
		certPath, err := a.pdfService.GenerateCertificate(path, report.RiskScore)
		if err == nil {
			result.Certificate = certPath
		} else {
			runtime.EventsEmit(a.ctx, "scan:error", fmt.Sprintf("Failed to generate cert: %s", err))
		}
	} else {
		// runtime.SystemTraySetIcon(a.ctx, iconRed) // Risk detected
	}
	
	runtime.EventsEmit(a.ctx, "scan:complete", result)
	return result, nil
}

// SelectFile opens a native file dialog and returns the selected path
func (a *App) SelectFile() (string, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Patient File",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Supported Files",
				Pattern:     "*.txt;*.log;*.md;*.csv;*.json;*.pdf;*.docx;*.xlsx;*.jpg;*.jpeg;*.png",
			},
			{
				DisplayName: "All Files",
				Pattern:     "*.*",
			},
		},
	})
	
	if err != nil {
		return "", err
	}
	return path, nil
}

// SelectFiles opens a native file dialog allowing multiple selection
func (a *App) SelectFiles() ([]string, error) {
	paths, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Patient Files",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Supported Files",
				Pattern:     "*.txt;*.log;*.md;*.csv;*.json;*.pdf;*.docx;*.xlsx;*.jpg;*.jpeg;*.png",
			},
			{
				DisplayName: "All Files",
				Pattern:     "*.*",
			},
		},
	})
	
	if err != nil {
		return nil, err
	}
	return paths, nil
}


// SelectDirectory opens a native directory dialog and returns the selected path
func (a *App) SelectDirectory() (string, error) {
	path, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Folder to Audit",
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

// ScanDirectory is exposed to the frontend
func (a *App) ScanDirectory(path string) (risk.AuditReport, error) {
	// Create a new context for this scan
	scanCtx, cancel := context.WithCancel(context.Background())
	a.cancelScan = cancel
	
	runtime.EventsEmit(a.ctx, "scan:dir_start", path)
	
	// Progress callback
	progress := func(currentPath string) {
		runtime.EventsEmit(a.ctx, "scan:progress", currentPath)
	}

	report, err := a.riskEngine.AnalyzeDirectory(scanCtx, path, progress)
	if err != nil {
		if err == context.Canceled {
			runtime.EventsEmit(a.ctx, "scan:cancelled", "Scan stopped by user")
			return risk.AuditReport{}, nil
		}
		runtime.EventsEmit(a.ctx, "scan:error", err.Error())
		return risk.AuditReport{}, err
	}
	
	runtime.EventsEmit(a.ctx, "scan:dir_complete", report)
	return report, nil
}

// CancelScan aborts the running directory scan
func (a *App) CancelScan() {
	if a.cancelScan != nil {
		a.cancelScan()
		a.cancelScan = nil
	}
}

// CancelScheduledScan aborts the running scheduled scan
func (a *App) CancelScheduledScan() {
	if a.scheduler != nil {
		a.scheduler.CancelScan()
	}
}

// OpenPath opens a file or directory using the system default application
func (a *App) OpenPath(path string) error {
	runtime.BrowserOpenURL(a.ctx, "file://"+path) // Try Wails native first
	// If Wails blocks file://, we might need exec.Command "open" on mac
	// But let's try to just return the path to frontend ?? 
	// No, the user wants to click and open it.
	
	// Better approach for local files on Mac:
	cmd := exec.Command("open", path)
	return cmd.Start()
}

// GenerateReport creates a PDF from the audit results
func (a *App) GenerateReport(report risk.AuditReport) (string, error) {
	// Prompt user for save location
	defaultName := fmt.Sprintf("HIPAA_Audit_Report_%d.pdf", time.Now().Unix())
	if report.TotalRiskScore == 0 {
		defaultName = fmt.Sprintf("HIPAA_Compliance_Certificate_%s.pdf", time.Now().Format("2006-01-02"))
	}
	
	// Open Save Dialog (Frontend will handle loading state)
	savePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Save Report",
		DefaultFilename: defaultName,
		Filters:         []runtime.FileFilter{{DisplayName: "PDF Files", Pattern: "*.pdf"}},
	})
	
	if err != nil {
		return "", err
	}
	if savePath == "" {
		return "", fmt.Errorf("cancelled")
	}

	// If Clean: Generate Certificate
	if report.TotalRiskScore == 0 {
		hostname, _ := os.Hostname()
		
		// Get audit history from storage (if available)
		var auditHistory []storage.AuditEntry
		if a.store != nil {
			if config, err := a.store.Load(); err == nil {
				auditHistory = config.AuditHistory
			}
		}
		
		return a.pdfService.GenerateComplianceCertificate(report.TotalFiles, hostname, auditHistory, savePath)
	}

	// If Risks: Generate Audit Report
	var offenders [][]string
	for _, off := range report.TopOffenders {
		offenders = append(offenders, []string{
			off.RiskLabel,
			off.FilePath,
			fmt.Sprintf("$%d", off.EstimatedFine),
		})
	}
	
	return a.pdfService.GenerateAuditReport(report.TotalFiles, report.CriticalCount, report.PotentialLiability, offenders, savePath)
}
// RedactFile creates a sanitized copy of the file
func (a *App) RedactFile(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	
	// Handle Binary Formats: Extract Text -> Redact -> Save as .txt
	if ext == ".pdf" || ext == ".docx" || ext == ".xlsx" {
		// 1. Extract Text
		rawText, err := a.riskEngine.ExtractText(path) // Helper we need to expose or use content package directly
		if err != nil {
			return "", fmt.Errorf("extraction failed: %w", err)
		}
		
		// 2. Redact
		redactedContent := a.riskEngine.RedactContent([]byte(rawText))
		
		// 3. Save as .txt (Safe for LLM)
		barePath := strings.TrimSuffix(path, ext)
		newPath := fmt.Sprintf("%s_CLEANED_TRANSCRIPT.txt", barePath)
		
		title := fmt.Sprintf("SAFE EXPORT FROM %s\nGENERATED BY HIPAA GUARDIAN\n----------------------------------------\n\n", filepath.Base(path))
		finalContent := append([]byte(title), redactedContent...)
		
		err = os.WriteFile(newPath, finalContent, 0644)
		if err != nil {
			return "", err
		}
		return newPath, nil
	}

	// Handle Text/CSV Formats: Preserve Format
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	
	redactedContent := a.riskEngine.RedactContent(content)
	
	barePath := strings.TrimSuffix(path, ext)
	newPath := fmt.Sprintf("%s_CLEANED%s", barePath, ext)
	
	err = os.WriteFile(newPath, redactedContent, 0644)
	if err != nil {
		return "", err
	}
	
	return newPath, nil
}
