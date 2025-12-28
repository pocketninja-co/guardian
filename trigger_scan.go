package main

import "fmt"

// TriggerScheduledScan manually triggers a scheduled scan immediately
func (a *App) TriggerScheduledScan() error {
	if a.scheduler == nil {
		return fmt.Errorf("scheduler not initialized")
	}
	
	// Trigger scan in a goroutine so it doesn't block
	go a.scheduler.RunNow()
	
	return nil
}
