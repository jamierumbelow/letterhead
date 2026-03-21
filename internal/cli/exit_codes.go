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
)

// ExitError is an error with a specific exit code.
type ExitError struct {
	Code    int
	Message string
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
