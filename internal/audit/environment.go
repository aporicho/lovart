package audit

import "os"

const EnvLogPath = "LOVART_AUDIT_LOG"

// NewEnvironment returns a path-backed logger when LOVART_AUDIT_LOG is set.
// Without the environment variable it returns a disabled no-op logger.
func NewEnvironment() (*Logger, error) {
	return NewPath(os.Getenv(EnvLogPath))
}
