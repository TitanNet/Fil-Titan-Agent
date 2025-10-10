package common

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// LogRotator handles log file rotation based on time intervals
type LogRotator struct {
	ctx           context.Context
	dir           string
	basename      string
	period        time.Duration // Rotation period (e.g., 1 hour, 24 hours)
	retainCount   int           // Number of log files to retain (0 = keep all)
	readers       []io.Reader
	currentFile   *os.File
	currentWriter io.Writer
	currentTime   time.Time
	mu            sync.Mutex
}

// NewLogRotator creates a new LogRotator instance
// ctx: Context for cancellation
// dir: Directory to store log files
// basename: Base name for log files
// period: Rotation period (must be positive)
// retainCount: Number of log files to retain (0 = keep all)
// readers: Input readers to collect logs from
func NewLogRotator(ctx context.Context, dir, basename string, period time.Duration, retainCount int, readers ...io.Reader) *LogRotator {
	return &LogRotator{
		ctx:         ctx,
		dir:         dir,
		basename:    strings.TrimRight(basename, ".log"),
		period:      period,
		retainCount: retainCount,
		readers:     readers,
	}
}

// Start begins the log rotation and collection process
func (lr *LogRotator) Start() error {
	if lr.period <= 0 {
		return errors.New("log rotation period must be positive")
	}

	// Align to current time slice
	lr.currentTime = lr.alignTime(time.Now())
	if err := lr.rotateLog(); err != nil {
		return err
	}

	go lr.runRotator()

	var wg sync.WaitGroup
	for _, r := range lr.readers {
		wg.Add(1)
		go func(r io.Reader) {
			defer wg.Done()
			lr.collectLogs(r)
		}(r)
	}

	wg.Wait()
	return nil
}

// alignTime rounds the given time to the nearest time slice boundary
func (lr *LogRotator) alignTime(t time.Time) time.Time {
	switch {
	case lr.period <= time.Minute:
		return t.Truncate(time.Minute) // Align to minute
	case lr.period <= time.Hour:
		return t.Truncate(time.Hour) // Align to hour
	default:
		return t.Truncate(24 * time.Hour) // Align to day
	}
}

// rotateLog performs the actual log rotation
func (lr *LogRotator) rotateLog() error {
	lr.mu.Lock()
	defer lr.mu.Unlock()

	now := time.Now()
	currentTime := lr.alignTime(now)

	// Skip rotation if still in current time slice
	if lr.currentFile != nil && !currentTime.After(lr.currentTime) {
		return nil
	}

	// Close current file if exists
	if lr.currentFile != nil {
		_ = lr.currentFile.Close()
	}

	// Create new log file with timestamp
	logFile := filepath.Join(lr.dir, fmt.Sprintf(
		"%s-%s.log",
		lr.basename,
		currentTime.Local().Format("2006-01-02T15-04"),
	))

	if err := os.MkdirAll(lr.dir, 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	lr.currentFile = f
	lr.currentWriter = f
	lr.currentTime = currentTime

	// Clean up old logs if retention policy is set
	if lr.retainCount > 0 {
		if err := lr.cleanupOldLogs(); err != nil {
			return fmt.Errorf("failed to cleanup old logs: %v", err)
		}
	}

	return nil
}

// cleanupOldLogs removes old log files beyond the retain count
func (lr *LogRotator) cleanupOldLogs() error {
	pattern := filepath.Join(lr.dir, lr.basename+"-*.log")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	// Sort files by modification time (newest first)
	sort.Slice(files, func(i, j int) bool {
		info1, _ := os.Stat(files[i])
		info2, _ := os.Stat(files[j])
		return info1.ModTime().After(info2.ModTime())
	})

	// Remove files beyond retain count
	for i := lr.retainCount; i < len(files); i++ {
		if err := os.Remove(files[i]); err != nil {
			return err
		}
	}

	return nil
}

// runRotator manages the periodic rotation
func (lr *LogRotator) runRotator() {
	// Calculate time until next rotation
	now := time.Now()
	next := lr.alignTime(now).Add(lr.period)
	initialDelay := next.Sub(now)

	// Wait until next rotation time
	select {
	case <-lr.ctx.Done():
		return
	case <-time.After(initialDelay):
	}

	// Then rotate at fixed intervals
	ticker := time.NewTicker(lr.period)
	defer ticker.Stop()

	for {
		select {
		case <-lr.ctx.Done():
			return
		case <-ticker.C:
			if err := lr.rotateLog(); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to rotate log: %v\n", err)
			}
		}
	}
}

// collectLogs reads from the input reader and writes to current log file
func (lr *LogRotator) collectLogs(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		select {
		case <-lr.ctx.Done():
			return
		default:
			line := scanner.Text()
			lr.mu.Lock()
			if lr.currentWriter != nil {
				_, _ = fmt.Fprintln(lr.currentWriter, line)
			}
			lr.mu.Unlock()
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Log collection error: %v\n", err)
	}
}
