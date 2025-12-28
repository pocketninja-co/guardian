package main

import (
	"hipaa-app/internal/scheduler"
	"hipaa-app/internal/storage"
	
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Schedule Configuration Methods

// GetScheduleConfig returns the current schedule configuration
func (a *App) GetScheduleConfig() (storage.ScheduleConfig, error) {
	config, err := a.store.Load()
	if err != nil {
		return storage.ScheduleConfig{}, err
	}
	return *config, nil
}

// UpdateScheduleConfig saves a new schedule configuration
func (a *App) UpdateScheduleConfig(config storage.ScheduleConfig) error {
	if err := a.store.Save(&config); err != nil {
		return err
	}
	
	// Restart scheduler with new config
	if a.scheduler != nil {
		a.scheduler.Stop()
	}
	
	eventHandler := func(event string, data interface{}) {
		runtime.EventsEmit(a.ctx, event, data)
	}
	
	a.scheduler = scheduler.NewScheduler(a.riskEngine, a.pdfService, a.store, eventHandler)
	return a.scheduler.Start(a.ctx)
}

// GetAuditHistory returns the stored audit history
func (a *App) GetAuditHistory() ([]storage.AuditEntry, error) {
	config, err := a.store.Load()
	if err != nil {
		return nil, err
	}
	return config.AuditHistory, nil
}

// AddSchedulePath adds a path to the scheduled scan list
func (a *App) AddSchedulePath(path string) error {
	config, err := a.store.Load()
	if err != nil {
		return err
	}
	
	// Check for duplicates
	for _, p := range config.ScanPaths {
		if p == path {
			return nil
		}
	}
	
	config.ScanPaths = append(config.ScanPaths, path)
	return a.store.Save(config)
}

// RemoveSchedulePath removes a path from the scheduled scan list
func (a *App) RemoveSchedulePath(path string) error {
	config, err := a.store.Load()
	if err != nil {
		return err
	}
	
	filtered := []string{}
	for _, p := range config.ScanPaths {
		if p != path {
			filtered = append(filtered, p)
		}
	}
	
	config.ScanPaths = filtered
	return a.store.Save(config)
}
