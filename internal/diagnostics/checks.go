package diagnostics

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jamierumbelow/letterhead/internal/auth"
	"github.com/jamierumbelow/letterhead/internal/config"
	"github.com/jamierumbelow/letterhead/internal/gmail"
	"github.com/jamierumbelow/letterhead/internal/scheduler"
	"github.com/jamierumbelow/letterhead/internal/store"
)

// CheckResult represents the outcome of a single health check.
type CheckResult struct {
	Name    string
	Status  Status
	Message string
}

// Status is the outcome category.
type Status string

const (
	StatusPass Status = "pass"
	StatusWarn Status = "warn"
	StatusFail Status = "fail"
	StatusSkip Status = "skip"
)

// RunAll executes all health checks and returns their results.
func RunAll(ctx context.Context, cfg *config.Config, s *store.Store, fix bool) []CheckResult {
	var results []CheckResult

	results = append(results, checkConfig(cfg))
	results = append(results, checkToken(cfg))
	results = append(results, checkGmailAccess(ctx, cfg))
	results = append(results, checkSQLiteIntegrity(ctx, s))
	results = append(results, checkFTSConsistency(ctx, s))
	results = append(results, checkCheckpointHealth(ctx, s, cfg))
	results = append(results, checkScheduler())
	results = append(results, checkDiskSpace(cfg))

	if fix {
		results = append(results, attemptFixes(ctx, cfg, s)...)
	}

	return results
}

func checkConfig(cfg *config.Config) CheckResult {
	if cfg == nil {
		return CheckResult{Name: "config", Status: StatusFail, Message: "config not loaded"}
	}
	if err := cfg.Validate(); err != nil {
		return CheckResult{Name: "config", Status: StatusFail, Message: err.Error()}
	}
	return CheckResult{Name: "config", Status: StatusPass, Message: "valid"}
}

func checkToken(cfg *config.Config) CheckResult {
	if cfg == nil || cfg.AccountEmail == "" {
		return CheckResult{Name: "token", Status: StatusSkip, Message: "no account configured"}
	}

	has := auth.IsAuthenticated(cfg.AccountEmail)
	if !has {
		return CheckResult{Name: "token", Status: StatusFail, Message: "no token file found"}
	}
	return CheckResult{Name: "token", Status: StatusPass, Message: "present"}
}

func checkGmailAccess(ctx context.Context, cfg *config.Config) CheckResult {
	if cfg == nil || cfg.AccountEmail == "" {
		return CheckResult{Name: "gmail_access", Status: StatusSkip, Message: "no account configured"}
	}

	result, err := auth.GetClient(ctx, cfg.AccountEmail)
	if err != nil {
		return CheckResult{Name: "gmail_access", Status: StatusFail, Message: fmt.Sprintf("auth error: %v", err)}
	}

	client, err := gmail.NewClient(ctx, result.Client)
	if err != nil {
		return CheckResult{Name: "gmail_access", Status: StatusFail, Message: fmt.Sprintf("client error: %v", err)}
	}

	profile, err := client.GetProfile(ctx)
	if err != nil {
		return CheckResult{Name: "gmail_access", Status: StatusFail, Message: fmt.Sprintf("profile error: %v", err)}
	}

	return CheckResult{Name: "gmail_access", Status: StatusPass, Message: fmt.Sprintf("connected as %s", profile.Email)}
}

func checkSQLiteIntegrity(ctx context.Context, s *store.Store) CheckResult {
	if s == nil {
		return CheckResult{Name: "sqlite_integrity", Status: StatusSkip, Message: "no database"}
	}

	var result string
	err := s.DB().QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&result)
	if err != nil {
		return CheckResult{Name: "sqlite_integrity", Status: StatusFail, Message: err.Error()}
	}
	if result != "ok" {
		return CheckResult{Name: "sqlite_integrity", Status: StatusFail, Message: result}
	}
	return CheckResult{Name: "sqlite_integrity", Status: StatusPass, Message: "ok"}
}

func checkFTSConsistency(ctx context.Context, s *store.Store) CheckResult {
	if s == nil {
		return CheckResult{Name: "fts_consistency", Status: StatusSkip, Message: "no database"}
	}

	var msgCount, ftsCount int
	if err := s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM messages`).Scan(&msgCount); err != nil {
		return CheckResult{Name: "fts_consistency", Status: StatusFail, Message: err.Error()}
	}
	if err := s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM messages_fts`).Scan(&ftsCount); err != nil {
		return CheckResult{Name: "fts_consistency", Status: StatusFail, Message: err.Error()}
	}

	if msgCount != ftsCount {
		return CheckResult{Name: "fts_consistency", Status: StatusWarn,
			Message: fmt.Sprintf("messages=%d fts=%d (run rebuild to fix)", msgCount, ftsCount)}
	}
	return CheckResult{Name: "fts_consistency", Status: StatusPass,
		Message: fmt.Sprintf("%d rows in sync", msgCount)}
}

func checkCheckpointHealth(ctx context.Context, s *store.Store, cfg *config.Config) CheckResult {
	if s == nil || cfg == nil {
		return CheckResult{Name: "checkpoint", Status: StatusSkip, Message: "no database or config"}
	}

	st, err := s.GetSyncState(ctx, cfg.AccountEmail)
	if err == sql.ErrNoRows {
		return CheckResult{Name: "checkpoint", Status: StatusWarn, Message: "no sync state yet"}
	}
	if err != nil {
		return CheckResult{Name: "checkpoint", Status: StatusFail, Message: err.Error()}
	}

	if st.BootstrapComplete && st.HistoryID == 0 {
		return CheckResult{Name: "checkpoint", Status: StatusFail, Message: "bootstrap complete but history_id is 0"}
	}

	if !st.BootstrapComplete {
		return CheckResult{Name: "checkpoint", Status: StatusWarn,
			Message: fmt.Sprintf("bootstrap incomplete (%d messages synced)", st.MessagesSynced)}
	}

	return CheckResult{Name: "checkpoint", Status: StatusPass,
		Message: fmt.Sprintf("history_id=%d, %d messages", st.HistoryID, st.MessagesSynced)}
}

func checkScheduler() CheckResult {
	sched := scheduler.New()
	if sched.IsInstalled() {
		return CheckResult{Name: "scheduler", Status: StatusPass, Message: sched.Status()}
	}
	return CheckResult{Name: "scheduler", Status: StatusWarn, Message: "not installed (run sync install)"}
}

func checkDiskSpace(cfg *config.Config) CheckResult {
	if cfg == nil {
		return CheckResult{Name: "disk_space", Status: StatusSkip, Message: "no config"}
	}

	free, err := freeDiskBytes(cfg.ArchiveRoot)
	if err != nil {
		return CheckResult{Name: "disk_space", Status: StatusSkip, Message: err.Error()}
	}

	mb := free / (1024 * 1024)
	if mb < 500 {
		return CheckResult{Name: "disk_space", Status: StatusWarn,
			Message: fmt.Sprintf("%d MB free (< 500 MB)", mb)}
	}
	return CheckResult{Name: "disk_space", Status: StatusPass,
		Message: fmt.Sprintf("%d MB free", mb)}
}

func attemptFixes(ctx context.Context, cfg *config.Config, s *store.Store) []CheckResult {
	var results []CheckResult

	if s != nil {
		if err := s.RebuildFTS(ctx); err != nil {
			results = append(results, CheckResult{Name: "fix_fts", Status: StatusFail, Message: err.Error()})
		} else {
			results = append(results, CheckResult{Name: "fix_fts", Status: StatusPass, Message: "FTS index rebuilt"})
		}
	}

	return results
}
