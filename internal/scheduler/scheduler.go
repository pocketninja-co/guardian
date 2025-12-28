package scheduler

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	
	"hipaa-app/internal/pdf"
	"hipaa-app/internal/risk"
	"hipaa-app/internal/storage"
)

// Scheduler manages background scan tasks
type Scheduler struct {
	riskEngine        *risk.RiskEngine
	pdfService        *pdf.PDFService
	store             *storage.Store
	ticker            *time.Ticker
	stop              chan bool
	eventEmitter      func(string, interface{})
	trigger           chan bool
	cancelCurrentScan context.CancelFunc
	mu                sync.Mutex
}

// NewScheduler creates a new scheduler instance
func NewScheduler(engine *risk.RiskEngine, pdfService *pdf.PDFService, store *storage.Store, emitEvent func(string, interface{})) *Scheduler {
	return &Scheduler{
		riskEngine:   engine,
		pdfService:   pdfService,
		store:        store,
		stop:         make(chan bool),
		eventEmitter: emitEvent,
		trigger:      make(chan bool, 1),
	}
}

// Start begins the scheduled scanning
func (s *Scheduler) Start(ctx context.Context) error {
	fmt.Println("[Scheduler] Starting...")
	config, err := s.store.Load()
	if err != nil {
		fmt.Printf("[Scheduler] Error loading config: %v\n", err)
		return err
	}
	
	fmt.Printf("[Scheduler] Initial config: Enabled=%v, Interval=%d hours, Paths=%v\n", config.Enabled, config.IntervalHours, config.ScanPaths)

	// Always start the run loop to listen for manual triggers
	// If disabled, just don't set a ticker yet, or set a long one/stop it immediately
	
	s.ticker = time.NewTicker(24 * time.Hour) // Default dummy ticker
	if !config.Enabled || config.IntervalHours == 0 {
		s.ticker.Stop() // Stop ticker if disabled, run loop will block on select but still listen to trigger
		fmt.Println("[Scheduler] Scheduled execution paused (Waiting for specific time or manual trigger)")
	} else {
		interval := time.Duration(config.IntervalHours) * time.Hour
		s.ticker.Reset(interval)
	}
	
	go s.run(ctx, config)
	
	return nil
}

// Stop gracefully stops the scheduler
func (s *Scheduler) Stop() {
	fmt.Println("[Scheduler] Stopping...")
	if s.ticker != nil {
		s.ticker.Stop()
	}
	s.CancelScan() // Cancel any running scan
	close(s.stop)
}

// CancelScan cancels the currently running scan if any
func (s *Scheduler) CancelScan() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.cancelCurrentScan != nil {
		fmt.Println("[Scheduler] Cancelling current scan")
		s.cancelCurrentScan()
		s.cancelCurrentScan = nil
	}
}

// RunNow immediately triggers a scheduled scan (manual trigger)
func (s *Scheduler) RunNow() {
	fmt.Println("[Scheduler] RunNow triggered")
	if s.trigger != nil {
		select {
		case s.trigger <- true:
			// Trigger sent successfully
		default:
			// Trigger already pending or channel full, ignore
			fmt.Println("[Scheduler] RunNow skipped (already pending)")
		}
	}
}

func (s *Scheduler) run(ctx context.Context, initialConfig *storage.ScheduleConfig) {
	fmt.Println("[Scheduler] Run loop started")
	// Run once immediately ONLY IF ENABLED
	if initialConfig.Enabled && initialConfig.IntervalHours > 0 {
		s.executeScan(ctx, initialConfig.ScanPaths)
	}
	
	for {
		select {
		case <-s.ticker.C:
			fmt.Println("[Scheduler] Ticker fired")
			// Reload config in case it changed
			config, err := s.store.Load()
			if err != nil || !config.Enabled {
				continue
			}
			s.executeScan(ctx, config.ScanPaths)
			
		case <-s.trigger:
			fmt.Println("[Scheduler] Manual trigger received in run loop")
			// Manual trigger - reload config to get latest paths, but IGNORE enabled status
			config, err := s.store.Load()
			if err != nil {
				fmt.Printf("[Scheduler] Cannot run: Error loading config=%v\n", err)
				continue
			}
			fmt.Println("[Scheduler] Executing manual scan...")
			s.executeScan(ctx, config.ScanPaths)

		case <-s.stop:
			fmt.Println("[Scheduler] Stop signal received")
			s.ticker.Stop()
			return
		}
	}
}

func (s *Scheduler) executeScan(parentCtx context.Context, paths []string) {
	fmt.Printf("[Scheduler] executeScan calling for paths: %v\n", paths)
	if len(paths) == 0 {
		fmt.Println("[Scheduler] No paths to scan")
		return
	}
	
	// Create a cancellable context for THIS scan
	ctx, cancel := context.WithCancel(parentCtx)
	s.mu.Lock()
	s.cancelCurrentScan = cancel
	s.mu.Unlock()
	
	defer func() {
		s.mu.Lock()
		s.cancelCurrentScan = nil
		s.mu.Unlock()
		cancel()
	}()
	
	// Supported file extensions (must match risk/engine.go)
	isScannable := func(ext string) bool {
		switch ext {
		case ".txt", ".csv", ".log", ".md", ".json", ".xml", ".html", ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".rtf",
			".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".tif":
			return true
		}
		return false
	}
	
	s.notify("log:info", fmt.Sprintf("Starting scan of %d directories...", len(paths)))

	// First, count total files to scan
	totalToScan := 0
	fmt.Println("[Scheduler] Counting files...")
	
	for _, path := range paths {
		// Check for cancellation during counting setup
		select {
		case <-ctx.Done():
			fmt.Println("[Scheduler] Scan cancelled during counting setup")
			s.notify("scan:scheduled:error", map[string]interface{}{"error": "Scan cancelled by user"})
			return
		default:
		}
		
		fmt.Printf("[Scheduler] Walking path: %s\n", path)
		filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
			// Check for cancellation per file
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if err != nil {
				// Log permission errors etc but continue
				fmt.Printf("[Scheduler] Error accessing %s: %v\n", p, err)
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if isScannable(strings.ToLower(filepath.Ext(p))) {
				totalToScan++
			}
			return nil
		})
		
		// If context was cancelled during WalkDir, return
		if ctx.Err() != nil {
			fmt.Println("[Scheduler] Scan cancelled during WalkDir")
			s.notify("scan:scheduled:error", map[string]interface{}{"error": "Scan cancelled by user"})
			return
		}
	}
	
	fmt.Printf("[Scheduler] Total files to scan: %d\n", totalToScan)
	s.notify("scan:scheduled:start", map[string]interface{}{
		"paths": paths,
		"total": totalToScan,
	})
	
	totalFiles := 0
	totalRisk := 0
	scannedCount := 0
	var riskyFiles []map[string]interface{}
	
	// Progress callback for per-file updates
	progressCallback := func(filePath string) {
		scannedCount++
		// Send progress every file or every N files to avoid spamming frontend? 
		// For now every file as per user request
		fmt.Printf("[Scheduler] Scanned: %s (%d/%d)\n", filepath.Base(filePath), scannedCount, totalToScan)
		s.notify("scan:scheduled:progress", map[string]interface{}{
			"current": scannedCount,
			"total":   totalToScan,
			"file":    filepath.Base(filePath),
		})
	}
	
	for _, path := range paths {
		fmt.Printf("[Scheduler] Analyzing directory: %s\n", path)
		report, err := s.riskEngine.AnalyzeDirectory(ctx, path, progressCallback)
		if err != nil {
			if err == context.Canceled {
				fmt.Println("[Scheduler] Analysis cancelled")
				s.notify("scan:scheduled:error", map[string]interface{}{"error": "Scan cancelled by user"})
				return
			}
			fmt.Printf("[Scheduler] Error analyzing %s: %v\n", path, err)
			s.notify("scan:scheduled:error", map[string]interface{}{"path": path, "error": err.Error()})
			continue
		}
		
		totalFiles += report.TotalFiles
		totalRisk += report.TotalRiskScore
		
		// Collect risky files for frontend
		for _, offender := range report.TopOffenders {
			if offender.RiskScore > 0 {
				riskyFiles = append(riskyFiles, map[string]interface{}{
					"path":      offender.FilePath,
					"riskScore": offender.RiskScore,
					"findings":  offender.Findings,
				})
			}
		}
	}
	
	// Record audit entry
	status := "PASSED"
	certPath := ""
	
	if totalRisk > 0 {
		status = "FAILED"
	} else if totalFiles > 0 {
		// Auto-generate certificate if passed
		fmt.Println("[Scheduler] Scan passed! Generating certificate...")
		hostname, _ := os.Hostname()
		
		// Get passed audits as history
		config, err := s.store.Load()
		auditHistory := []storage.AuditEntry{}
		if err == nil {
			auditHistory = config.AuditHistory
		}
		
		path, err := s.pdfService.GenerateComplianceCertificate(totalFiles, hostname, auditHistory, "")
		if err == nil {
			certPath = path
			fmt.Printf("[Scheduler] Certificate generated: %s\n", path)
		} else {
			fmt.Printf("[Scheduler] Failed to generate certificate: %v\n", err)
		}
	}
	
	hostname, _ := os.Hostname()
	entry := storage.AuditEntry{
		Timestamp:  time.Now().Format(time.RFC3339),
		TotalFiles: totalFiles,
		RiskScore:  totalRisk,
		User:       hostname,
		Status:     status,
	}
	
	s.store.AddAuditEntry(entry)
	
	// Create map for notification
	notifyData := map[string]interface{}{
		"status":      status,
		"total_files": totalFiles,
		"risk_score":  totalRisk,
		"risky_files": riskyFiles,
		"certificate": certPath,
	}

	fmt.Printf("[Scheduler] Scan complete. Status: %s, Files: %d, Risk: %d\n", status, totalFiles, totalRisk)
	s.notify("scan:scheduled:complete", notifyData)
}


func (s *Scheduler) notify(event string, data interface{}) {
	if s.eventEmitter != nil {
		s.eventEmitter(event, data)
	}
}

