package audit

import (
	"encoding/json"
	"io"
	"sync"
	"time"

	"github.com/aporicho/lovart/internal/security/redaction"
)

// Logger writes audit events as JSONL records. A disabled logger accepts events
// without writing so callers can depend on audit logging without mandatory setup.
type Logger struct {
	mu      sync.Mutex
	writer  io.Writer
	closer  io.Closer
	enabled bool
	now     func() time.Time
}

// Disabled returns a logger that drops events without side effects.
func Disabled() *Logger {
	return &Logger{now: time.Now}
}

// NewWriter returns a JSONL logger backed by writer. A nil writer creates a
// disabled logger.
func NewWriter(writer io.Writer) *Logger {
	if writer == nil {
		return Disabled()
	}
	return &Logger{writer: writer, enabled: true, now: time.Now}
}

// Enabled reports whether this logger has an active output destination.
func (logger *Logger) Enabled() bool {
	return logger != nil && logger.enabled && logger.writer != nil
}

// Write appends one redacted JSON object followed by a newline.
func (logger *Logger) Write(event Event) error {
	if !logger.Enabled() {
		return nil
	}

	record := logger.redactedEvent(event)
	line, err := json.Marshal(record)
	if err != nil {
		return err
	}
	line = append(line, '\n')

	logger.mu.Lock()
	defer logger.mu.Unlock()
	_, err = logger.writer.Write(line)
	return err
}

// Close closes the logger output when it owns one. Writer-backed loggers do not
// close caller-owned writers.
func (logger *Logger) Close() error {
	if logger == nil || logger.closer == nil {
		return nil
	}
	return logger.closer.Close()
}

func (logger *Logger) redactedEvent(event Event) Event {
	if event.Timestamp.IsZero() {
		event.Timestamp = logger.now().UTC()
	}
	if event.Metadata != nil {
		event.Metadata = redaction.RedactMap(event.Metadata)
	}
	if event.Error != "" {
		event.Error = redaction.RedactString(event.Error)
	}
	return event
}
