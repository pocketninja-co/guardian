package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// AuditEntry represents a single audit history record
type AuditEntry struct {
	Timestamp  string `json:"timestamp"`
	TotalFiles int    `json:"total_files"`
	RiskScore  int    `json:"risk_score"`
	User       string `json:"user"`
	Status     string `json:"status"` // "PASSED" or "FAILED"
}

// ScheduleConfig holds the scheduler configuration and cumulative stats
type ScheduleConfig struct {
	Enabled          bool         `json:"schedule_enabled"`
	IntervalHours    int          `json:"scan_interval_hours"` // Deprecated, kept for backwards compatibility
	IntervalValue    int          `json:"interval_value"`      // New: 1, 2, 3, etc.
	IntervalUnit     string       `json:"interval_unit"`       // New: "hours", "days", "weeks", "months"
	TimeOfDay        string       `json:"time_of_day"`         // New: "14:30" (24-hour format)
	Timezone         string       `json:"timezone"`            // New: "America/Chicago", etc.
	ScanPaths        []string     `json:"scan_paths"`
	AuditHistory     []AuditEntry `json:"audit_history"`
	LastNotification time.Time    `json:"last_notification,omitempty"`

	// Cumulative Stats (persist across sessions)
	TotalFilesScanned int `json:"total_files_scanned"`
	TotalRisksFound   int `json:"total_risks_found"`
	TotalLiability    int `json:"total_liability"`
}

type Store struct {
	db         *sql.DB
	dbPath     string
	configPath string // Keep for migration
}

// NewStore creates a new storage instance with SQLite
func NewStore() (*Store, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configDir := filepath.Join(homeDir, ".hipaa_guardian")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(configDir, "guardian.db")
	configPath := filepath.Join(configDir, "config.json")

	// Open SQLite database
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &Store{
		db:         db,
		dbPath:     dbPath,
		configPath: configPath,
	}

	// Initialize schema
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Migrate from JSON if exists
	if err := store.migrateFromJSON(); err != nil {
		// Log but don't fail - migration is best-effort
		fmt.Printf("Warning: JSON migration failed: %v\n", err)
	}

	return store, nil
}

// initSchema creates the necessary tables
func (s *Store) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS config (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS stats (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		total_files_scanned INTEGER DEFAULT 0,
		total_risks_found INTEGER DEFAULT 0,
		total_liability INTEGER DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS audit_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT NOT NULL,
		total_files INTEGER NOT NULL,
		risk_score INTEGER NOT NULL,
		user TEXT NOT NULL,
		status TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Initialize stats row if it doesn't exist
	INSERT OR IGNORE INTO stats (id, total_files_scanned, total_risks_found, total_liability)
	VALUES (1, 0, 0, 0);
	`

	_, err := s.db.Exec(schema)
	return err
}

// migrateFromJSON migrates data from old JSON file if it exists
func (s *Store) migrateFromJSON() error {
	// Check if JSON file exists
	if _, err := os.Stat(s.configPath); os.IsNotExist(err) {
		return nil // No migration needed
	}

	// Read JSON file
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		return err
	}

	var oldConfig ScheduleConfig
	if err := json.Unmarshal(data, &oldConfig); err != nil {
		return err
	}

	// Migrate settings
	settings := map[string]string{
		"schedule_enabled":    fmt.Sprintf("%t", oldConfig.Enabled),
		"scan_interval_hours": fmt.Sprintf("%d", oldConfig.IntervalHours),
		"interval_value":      fmt.Sprintf("%d", oldConfig.IntervalValue),
		"interval_unit":       oldConfig.IntervalUnit,
		"time_of_day":         oldConfig.TimeOfDay,
		"timezone":            oldConfig.Timezone,
	}

	// Store scan paths as JSON array
	pathsJSON, _ := json.Marshal(oldConfig.ScanPaths)
	settings["scan_paths"] = string(pathsJSON)

	for key, value := range settings {
		_, err := s.db.Exec("INSERT OR REPLACE INTO config (key, value) VALUES (?, ?)", key, value)
		if err != nil {
			return err
		}
	}

	// Migrate stats
	_, err = s.db.Exec(`UPDATE stats SET total_files_scanned = ?, total_risks_found = ?, total_liability = ? WHERE id = 1`,
		oldConfig.TotalFilesScanned, oldConfig.TotalRisksFound, oldConfig.TotalLiability)
	if err != nil {
		return err
	}

	// Migrate audit history
	for _, entry := range oldConfig.AuditHistory {
		_, err := s.db.Exec(`INSERT INTO audit_history (timestamp, total_files, risk_score, user, status) VALUES (?, ?, ?, ?, ?)`,
			entry.Timestamp, entry.TotalFiles, entry.RiskScore, entry.User, entry.Status)
		if err != nil {
			return err
		}
	}

	// Rename old file to mark as migrated
	backupPath := s.configPath + ".backup"
	os.Rename(s.configPath, backupPath)
	fmt.Println("Successfully migrated from JSON to SQLite")

	return nil
}

// Load reads the configuration from database
func (s *Store) Load() (*ScheduleConfig, error) {
	config := &ScheduleConfig{
		ScanPaths:    []string{},
		AuditHistory: []AuditEntry{},
	}

	// Load settings
	rows, err := s.db.Query("SELECT key, value FROM config")
	if err != nil {
		return config, nil // Return default if no settings yet
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}
		settings[key] = value
	}

	// Parse settings
	if val, ok := settings["schedule_enabled"]; ok {
		config.Enabled = val == "true"
	}
	if val, ok := settings["scan_interval_hours"]; ok {
		fmt.Sscanf(val, "%d", &config.IntervalHours)
	}
	if val, ok := settings["interval_value"]; ok {
		fmt.Sscanf(val, "%d", &config.IntervalValue)
	}
	if val, ok := settings["interval_unit"]; ok {
		config.IntervalUnit = val
	}
	if val, ok := settings["time_of_day"]; ok {
		config.TimeOfDay = val
	}
	if val, ok := settings["timezone"]; ok {
		config.Timezone = val
	}
	if val, ok := settings["scan_paths"]; ok {
		json.Unmarshal([]byte(val), &config.ScanPaths)
	}

	// Load stats
	err = s.db.QueryRow("SELECT total_files_scanned, total_risks_found, total_liability FROM stats WHERE id = 1").
		Scan(&config.TotalFilesScanned, &config.TotalRisksFound, &config.TotalLiability)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	// Load audit history (last 50)
	auditRows, err := s.db.Query(`SELECT timestamp, total_files, risk_score, user, status 
		FROM audit_history ORDER BY created_at DESC LIMIT 50`)
	if err == nil {
		defer auditRows.Close()
		for auditRows.Next() {
			var entry AuditEntry
			if err := auditRows.Scan(&entry.Timestamp, &entry.TotalFiles, &entry.RiskScore, &entry.User, &entry.Status); err == nil {
				config.AuditHistory = append(config.AuditHistory, entry)
			}
		}
	}

	return config, nil
}

// Save writes the configuration to database
func (s *Store) Save(config *ScheduleConfig) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Save settings
	settings := map[string]string{
		"schedule_enabled":    fmt.Sprintf("%t", config.Enabled),
		"scan_interval_hours": fmt.Sprintf("%d", config.IntervalHours),
		"interval_value":      fmt.Sprintf("%d", config.IntervalValue),
		"interval_unit":       config.IntervalUnit,
		"time_of_day":         config.TimeOfDay,
		"timezone":            config.Timezone,
	}

	pathsJSON, _ := json.Marshal(config.ScanPaths)
	settings["scan_paths"] = string(pathsJSON)

	for key, value := range settings {
		_, err := tx.Exec("INSERT OR REPLACE INTO config (key, value) VALUES (?, ?)", key, value)
		if err != nil {
			return err
		}
	}

	// Save stats
	_, err = tx.Exec(`UPDATE stats SET total_files_scanned = ?, total_risks_found = ?, total_liability = ? WHERE id = 1`,
		config.TotalFilesScanned, config.TotalRisksFound, config.TotalLiability)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// AddAuditEntry appends a new audit record to history
func (s *Store) AddAuditEntry(entry AuditEntry) error {
	_, err := s.db.Exec(`INSERT INTO audit_history (timestamp, total_files, risk_score, user, status) VALUES (?, ?, ?, ?, ?)`,
		entry.Timestamp, entry.TotalFiles, entry.RiskScore, entry.User, entry.Status)
	return err
}

// Close closes the database connection
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
