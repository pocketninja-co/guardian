package main

import "os"

// GetUsername returns the current OS username
func (a *App) GetUsername() string {
	// Try $USER first (macOS/Linux)
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	// Try $USERNAME (Windows)
	if user := os.Getenv("USERNAME"); user != "" {
		return user
	}
	// Fallback to hostname
	hostname, _ := os.Hostname()
	return hostname
}
