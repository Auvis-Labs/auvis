package audit

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Logger writes audit entries asynchronously to a .jsonl file.
// Each entry is pretty-printed and separated by a blank line for readability.
type Logger struct {
	ch         chan Entry
	done       chan struct{}
	once       sync.Once
	path       string
	maxSize    int64 // max size in bytes
	maxAgeDays int
}

// NewLogger creates a logger with default rotation values (10MB, 30 days) and starts the background writer.
func NewLogger(path string, bufSize int) (*Logger, error) {
	return NewRotatingLogger(path, bufSize, 10, 30)
}

// NewRotatingLogger creates a logger and starts the background writer goroutine with explicit rotation parameters.
func NewRotatingLogger(path string, bufSize int, maxSizeMB int, maxAgeDays int) (*Logger, error) {
	l := &Logger{
		ch:         make(chan Entry, bufSize),
		done:       make(chan struct{}),
		path:       path,
		maxSize:    int64(maxSizeMB) * 1024 * 1024,
		maxAgeDays: maxAgeDays,
	}

	go l.run()
	return l, nil
}

// NewStdoutLogger creates a logger that writes to os.Stdout.
func NewStdoutLogger(bufSize int) *Logger {
	l := &Logger{
		ch:   make(chan Entry, bufSize),
		done: make(chan struct{}),
		path: "stdout",
	}
	go l.run()
	return l
}

// Write enqueues an audit entry. Non-blocking — drops and warns if channel full.
func (l *Logger) Write(e Entry) {
	select {
	case l.ch <- e:
	default:
		slog.Warn("Audit queue channel full — entry dropped", "request_id", e.RequestID)
	}
}

// Close flushes all pending entries and shuts down the background writer.
func (l *Logger) Close() {
	l.once.Do(func() {
		close(l.ch)
		<-l.done
	})
}

func (l *Logger) run() {
	var f *os.File
	var err error
	var currentSize int64

	if l.path == "stdout" {
		f = os.Stdout
	} else {
		// Ensure directory exists
		dir := filepath.Dir(l.path)
		if dir != "." && dir != "" {
			_ = os.MkdirAll(dir, 0755)
		}

		f, err = os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			slog.Error("Failed to open audit log file", "error", err, "path", l.path)
			close(l.done)
			return
		}
		fi, err := f.Stat()
		if err == nil {
			currentSize = fi.Size()
		}
	}

	defer func() {
		if f != nil && f != os.Stdout {
			f.Close()
		}
		close(l.done)
	}()

	for entry := range l.ch {
		b, err := json.MarshalIndent(entry, "", "  ")
		if err != nil {
			slog.Error("Failed to marshal audit log entry", "error", err, "request_id", entry.RequestID)
			continue
		}
		data := append(b, '\n', '\n')

		// If writing to a file, check if size-based rotation is required
		if f != os.Stdout && l.maxSize > 0 {
			if currentSize+int64(len(data)) > l.maxSize {
				// Rotate!
				f.Close()

				// Rename current file to timestamped version
				dir := filepath.Dir(l.path)
				ext := filepath.Ext(l.path)
				base := strings.TrimSuffix(filepath.Base(l.path), ext)
				timestamp := time.Now().Format("20060102-150405")
				rotatedPath := filepath.Join(dir, fmt.Sprintf("%s-%s%s", base, timestamp, ext))

				err = os.Rename(l.path, rotatedPath)
				if err != nil {
					slog.Error("Failed to rename audit file for rotation", "error", err, "path", l.path, "target", rotatedPath)
				} else {
					slog.Info("Audit log rotated successfully", "rotated_file", rotatedPath)
				}

				// Perform cleanup of old log files
				l.cleanupOldLogs()

				// Re-open a fresh file
				f, err = os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
				if err != nil {
					slog.Error("Failed to open fresh active audit log file", "error", err, "path", l.path)
					return
				}
				currentSize = 0
			}
		}

		if _, err := f.Write(data); err != nil {
			slog.Error("Failed to write entry to audit log", "error", err, "request_id", entry.RequestID)
		} else {
			currentSize += int64(len(data))
		}
	}

	slog.Info("Audit logger background writer flushed and terminated")
}

func (l *Logger) cleanupOldLogs() {
	if l.maxAgeDays <= 0 || l.path == "stdout" {
		return
	}

	dir := filepath.Dir(l.path)
	ext := filepath.Ext(l.path)
	base := strings.TrimSuffix(filepath.Base(l.path), ext)

	entries, err := os.ReadDir(dir)
	if err != nil {
		slog.Error("Failed to read log directory for cleanup", "error", err, "directory", dir)
		return
	}

	cutoff := time.Now().AddDate(0, 0, -l.maxAgeDays)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Match rolled files, e.g. base-YYYYMMDD-HHMMSS.ext
		if strings.HasPrefix(name, base+"-") && strings.HasSuffix(name, ext) {
			path := filepath.Join(dir, name)
			fi, err := os.Stat(path)
			if err != nil {
				continue
			}
			if fi.ModTime().Before(cutoff) {
				slog.Info("Cleaning up expired rotated audit log file", "file", name, "modification_time", fi.ModTime())
				_ = os.Remove(path)
			}
		}
	}
}