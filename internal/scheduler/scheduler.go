package scheduler

import "runtime"

// Scheduler manages periodic sync job installation.
type Scheduler interface {
	Install(cfg Config) error
	Uninstall() error
	IsInstalled() bool
	Status() string
}

// Config holds the parameters for the scheduled sync job.
type Config struct {
	LetterheadBin string // path to the letterhead binary
	IntervalSecs  int    // sync interval in seconds
	LogDir        string // directory for stdout/stderr logs
}

// New returns the platform-appropriate scheduler.
func New() Scheduler {
	if runtime.GOOS == "darwin" {
		return &launchdScheduler{}
	}
	return &systemdScheduler{}
}
