package audit

import (
	"encoding/json"
	"log"
	"os"
	"sync"
)

// Logger writes audit entries asynchronously to a .jsonl file.
// Each entry is pretty-printed and separated by a blank line for readability.
type Logger struct {
	ch   chan Entry
	done chan struct{}
	once sync.Once
}

// NewLogger creates a logger and starts the background writer goroutine.
func NewLogger(path string, bufSize int) (*Logger, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	l := &Logger{
		ch:   make(chan Entry, bufSize),
		done: make(chan struct{}),
	}

	go l.run(f)
	return l, nil
}

// Write enqueues an audit entry. Non-blocking — drops and warns if channel full.
func (l *Logger) Write(e Entry) {
	select {
	case l.ch <- e:
	default:
		log.Printf("[AUDIT] warning: channel full, dropping entry for request_id=%s", e.RequestID)
	}
}

// Close flushes all pending entries and shuts down the background writer.
func (l *Logger) Close() {
	l.once.Do(func() {
		close(l.ch)
		<-l.done
	})
}

func (l *Logger) run(f *os.File) {
	defer func() {
		f.Close()
		close(l.done)
	}()

	for entry := range l.ch {
		b, err := json.MarshalIndent(entry, "", "  ")
		if err != nil {
			log.Printf("[AUDIT] marshal error: %v", err)
			continue
		}
		// Pretty-printed block + blank line separator between entries
		if _, err := f.Write(append(b, '\n', '\n')); err != nil {
			log.Printf("[AUDIT] write error: %v", err)
		}
	}

	log.Printf("[AUDIT] flushed and closed")
}