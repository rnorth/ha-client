package errors

import (
	"errors"
	"strings"
)

const (
	ExitGeneral  = 1
	ExitUsage    = 2
	ExitAuth     = 3
	ExitNotFound = 4
	ExitServer   = 5
)

type CLIError struct {
	Err      error
	ExitCode int
	Code     string // machine-readable: "auth_failed", "not_found", "server_error", "usage_error", "error"
}

func (e *CLIError) Error() string { return e.Err.Error() }
func (e *CLIError) Unwrap() error { return e.Err }

// Classify inspects an error and returns a CLIError with the appropriate exit code.
// Already-classified CLIErrors pass through unchanged.
func Classify(err error) *CLIError {
	if err == nil {
		return nil
	}
	var ce *CLIError
	if errors.As(err, &ce) {
		return ce
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "unauthorized"):
		return &CLIError{Err: err, ExitCode: ExitAuth, Code: "auth_failed"}
	case strings.Contains(msg, "not found"):
		return &CLIError{Err: err, ExitCode: ExitNotFound, Code: "not_found"}
	case strings.Contains(msg, "HTTP 5"):
		return &CLIError{Err: err, ExitCode: ExitServer, Code: "server_error"}
	default:
		return &CLIError{Err: err, ExitCode: ExitGeneral, Code: "error"}
	}
}
