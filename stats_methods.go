package main

import "hipaa-app/internal/storage"

// UpdateStats increments cumulative statistics after a scan
func (a *App) UpdateStats(filesScanned int, risksFound int, liability int) error {
	config, err := a.store.Load()
	if err != nil {
		return err
	}
	
	config.TotalFilesScanned += filesScanned
	config.TotalRisksFound += risksFound
	config.TotalLiability += liability
	
	return a.store.Save(config)
}

// GetStats returns current cumulative statistics
func (a *App) GetStats() (storage.ScheduleConfig, error) {
	config, err := a.store.Load()
	if err != nil {
		return storage.ScheduleConfig{}, err
	}
	return *config, nil
}
