package cli

import "fmt"

// Exit codes for letterhead commands.
const (
	ExitOK             = 0 // Success (including empty find results)
	ExitUsage          = 1 // Usage error or invalid flags
	ExitLockConflict   = 2 // Another sync is running
	ExitAuth           = 3 // Not authenticated or token expired
	ExitStore          = 4 // Database corruption or migration failure
	ExitNetwork        = 5 // Gmail API unreachable
	ExitNotInitialized = 6 // Run letterhead init first
	ExitNotFound       = 7 // Message or thread not found
)

// exitCodeName maps exit codes to short string identifiers for structured output.
var exitCodeName = map[int]string{
	ExitOK:             "ok",
	ExitUsage:          "usage",
	ExitLockConflict:   "lock_conflict",
	ExitAuth:           "auth",
	ExitStore:          "store",
	ExitNetwork:        "network",
	ExitNotInitialized: "not_initialized",
	ExitNotFound:       "not_found",
}

// ExitCodeName returns the short string identifier for an exit code.
func ExitCodeName(code int) string {
	if name, ok := exitCodeName[code]; ok {
		return name
	}
	return "unknown"
}

// ExitError is an error with a specific exit code and optional hint.
type ExitError struct {
	Code    int
	Message string
	Hint    string // machine-friendly suggestion, e.g. "run: letterhead auth"
}

func (e *ExitError) Error() string {
	return e.Message
}

// NewExitError creates an ExitError with the given code and message.
func NewExitError(code int, format string, args ...any) *ExitError {
	return &ExitError{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}
}

// NewExitErrorWithHint creates an ExitError with a hint for recovery.
func NewExitErrorWithHint(code int, hint string, format string, args ...any) *ExitError {
	return &ExitError{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
		Hint:    hint,
	}
}
