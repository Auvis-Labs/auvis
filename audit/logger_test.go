package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewLogger(t *testing.T) {
	l, err := NewLogger("stdout", 10)
	if err != nil {
		t.Fatalf("failed to create stdout logger: %v", err)
	}
	defer l.Close()

	if l.path != "stdout" {
		t.Errorf("expected path to be stdout, got %q", l.path)
	}
}

func TestLoggerRotationAndCleanup(t *testing.T) {
	// Create a temporary directory for logging tests
	tmpDir, err := os.MkdirTemp("", "auvis-audit-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logPath := filepath.Join(tmpDir, "audit.jsonl")

	// 1. Initialize rotating logger with a tiny maxSize limit (e.g. 1 KB) to trigger rollover quickly
	l, err := NewRotatingLogger(logPath, 50, 1, 1) // maxSizeMB = 1 (1MB in bytes, but we will mock maxSize to be tiny for the test)
	if err != nil {
		t.Fatalf("failed to create rotating logger: %v", err)
	}

	// Override maxSize in the struct directly to 50 bytes for immediate rotation test!
	l.maxSize = 250 // 250 bytes
	l.maxAgeDays = 1

	entry := Entry{
		Timestamp:       time.Now().UTC(),
		RequestID:       "req-1",
		UserID:          "usr-1",
		UserRole:        "admin",
		Resource:        "/users",
		Method:          "GET",
		PoliciesApplied: []string{"test-policy"},
		Priority:        100,
		ComplianceTags:  []string{"GDPR"},
		UpstreamStatus:  200,
		DurationMS:      45,
	}



	// We expect 2 writes of this entry to exceed our 250 bytes limit, causing a rotation!
	l.Write(entry)
	l.Write(entry)
	l.Write(entry)

	// Close the logger to flush everything and stop background worker
	l.Close()

	// Verify that the files in the directory contain the rolled-over audit file!
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read temp dir: %v", err)
	}

	var rotatedFile string
	var hasActiveFile bool

	for _, f := range files {
		name := f.Name()
		if name == "audit.jsonl" {
			hasActiveFile = true
		} else if strings.HasPrefix(name, "audit-") && strings.HasSuffix(name, ".jsonl") {
			rotatedFile = name
		}
	}

	if rotatedFile == "" {
		t.Errorf("expected to find a rotated log file starting with 'audit-', but found none in: %v", files)
	}
	if !hasActiveFile {
		t.Errorf("expected active file 'audit.jsonl' to exist, but not found")
	}

	// 2. Test Age-based Cleanup
	// Create an artificial old rolled file
	oldFile := filepath.Join(tmpDir, "audit-20260101-120000.jsonl")
	err = os.WriteFile(oldFile, []byte("dummy old log contents"), 0644)
	if err != nil {
		t.Fatalf("failed to write old log file: %v", err)
	}

	// Shift its mod time back by 2 days (limit is 1 day)
	oldTime := time.Now().Add(-48 * time.Hour)
	err = os.Chtimes(oldFile, oldTime, oldTime)
	if err != nil {
		t.Fatalf("failed to change mod time of old file: %v", err)
	}

	// Reinitialize a logger to trigger rotation/cleanup
	l2, err := NewRotatingLogger(logPath, 50, 1, 1)
	if err != nil {
		t.Fatalf("failed to re-create logger: %v", err)
	}
	l2.maxSize = 50 // tiny
	l2.maxAgeDays = 1

	// Trigger a write that exceeds maxSize to cause rotation and trigger cleanupOldLogs()
	l2.Write(entry)
	l2.Write(entry)
	l2.Close()

	// Verify that the old file was successfully cleaned up/deleted!
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Errorf("expected old log file %q to be deleted by age cleanup, but it still exists", oldFile)
	}
}
