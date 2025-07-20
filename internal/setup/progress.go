package setup

import (
	"fmt"
	"strings"
	"time"
)

type ProgressInfo struct {
	Stage        string    // Name of the operation/stage
	Total        int64     // Total number of items/bytes to process
	Completed    int64     // Number of items/bytes completed
	TotalBytes   int64     // Alias for Total (backwards compatibility)
	CurrentBytes int64     // Alias for Completed (backwards compatibility)
	Percentage   float64   // Completion percentage (0-100)
	CurrentItem  string    // Current item being processed
	StartTime    time.Time // When the operation started
	LastUpdate   time.Time // Last progress update time
	Status       string    // Current status message
}

func NewProgressInfo(stage string, total int64) *ProgressInfo {
	now := time.Now()
	return &ProgressInfo{
		Stage:        stage,
		Total:        total,
		Completed:    0,
		TotalBytes:   total, // Keep in sync for backwards compatibility
		CurrentBytes: 0,     // Keep in sync for backwards compatibility
		Percentage:   0.0,
		StartTime:    now,
		LastUpdate:   now,
	}
}

func (p *ProgressInfo) Update(completed int64, currentItem string) {
	p.Completed = completed
	p.CurrentBytes = completed // Keep in sync for backwards compatibility
	p.CurrentItem = currentItem
	p.LastUpdate = time.Now()

	if p.Total > 0 {
		p.Percentage = float64(completed) / float64(p.Total) * 100.0
		if p.Percentage > 100.0 {
			p.Percentage = 100.0
		}
	}
}

func (p *ProgressInfo) GetETA() time.Duration {
	if p.Completed == 0 || p.Total == 0 {
		return 0
	}

	elapsed := time.Since(p.StartTime)
	if elapsed == 0 {
		return 0
	}

	rate := float64(p.Completed) / elapsed.Seconds()
	if rate == 0 {
		return 0
	}

	remaining := p.Total - p.Completed
	etaSeconds := float64(remaining) / rate

	return time.Duration(etaSeconds) * time.Second
}

func (p *ProgressInfo) IsComplete() bool {
	return p.Completed >= p.Total
}

func (p *ProgressInfo) String() string {
	if p.Total > 0 {
		return fmt.Sprintf("%s: %.1f%% (%d/%d)", p.Stage, p.Percentage, p.Completed, p.Total)
	}
	return fmt.Sprintf("%s: %d items", p.Stage, p.Completed)
}

func extractFilename(urlOrPath string) string {
	if urlOrPath == "" {
		return "unknown"
	}

	if urlOrPath[len(urlOrPath)-1] == '/' {
		urlOrPath = urlOrPath[:len(urlOrPath)-1]
	}

	lastSlash := strings.LastIndex(urlOrPath, "/")
	if lastSlash >= 0 {
		return urlOrPath[lastSlash+1:]
	}

	return urlOrPath
}

func generateSessionID() string {
	return fmt.Sprintf("session-%d", time.Now().UnixNano())
}
