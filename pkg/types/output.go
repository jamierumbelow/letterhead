package types

import "time"

type ReadView string

const (
	ReadViewSummary ReadView = "summary"
	ReadViewText    ReadView = "text"
	ReadViewFull    ReadView = "full"
)

// AccountStatus represents the status of a single account in multi-account mode.
type AccountStatus struct {
	Email         string     `json:"email"`
	AuthMethod    string     `json:"auth_method"`
	Authenticated bool       `json:"authenticated"`
	MessageCount  int        `json:"message_count"`
	LastSyncAt    *time.Time `json:"last_sync_at"`
}

// StatusOutput is the stable machine-readable contract for `letterhead status`.
type StatusOutput struct {
	Account           string          `json:"account"`
	ArchivePath       string          `json:"archive_path"`
	SyncMode          string          `json:"sync_mode"`
	MessageCount      int             `json:"message_count"`
	ThreadCount       int             `json:"thread_count"`
	BootstrapComplete bool            `json:"bootstrap_complete"`
	BootstrapProgress float64         `json:"bootstrap_progress"`
	LastSyncAt        *time.Time      `json:"last_sync_at"`
	SchedulerState    string          `json:"scheduler_state"`
	DBHealth          string          `json:"db_health"`
	Accounts          []AccountStatus `json:"accounts,omitempty"`
	ArchiveSize       int64           `json:"archive_size,omitempty"`
}

// FindResult is the stable machine-readable contract for one `letterhead find` result.
type FindResult struct {
	ResultID      string    `json:"result_id"`
	AccountID     string    `json:"account_id,omitempty"`
	ThreadID      string    `json:"thread_id"`
	Subject       string    `json:"subject"`
	Participants  []string  `json:"participants"`
	LatestAt      time.Time `json:"latest_at"`
	MessageCount  int       `json:"message_count"`
	Snippet       string    `json:"snippet"`
	MatchedFields []string  `json:"matched_fields"`
	ReadHandle    string    `json:"read_handle"`
}

// FindOutput is the stable machine-readable contract for `letterhead find`.
type FindOutput struct {
	Results    []FindResult `json:"results"`
	TotalCount int          `json:"total_count"`
	Limit      int          `json:"limit,omitempty"`
	Offset     int          `json:"offset,omitempty"`
	QueryMS    int64        `json:"query_ms"`
}

// MessageSummary is the thread-safe summary shape reused by `letterhead read`.
type MessageSummary struct {
	AccountID       string    `json:"account_id"`
	MessageID       string    `json:"message_id"`
	ThreadID        string    `json:"thread_id"`
	Subject         string    `json:"subject"`
	From            Address   `json:"from"`
	Date            time.Time `json:"date"`
	Participants    []string  `json:"participants"`
	Snippet         string    `json:"snippet"`
	LabelNames      []string  `json:"label_names"`
	AttachmentCount int       `json:"attachment_count"`
}

// ReadOutput is the stable machine-readable contract for `letterhead read`.
type ReadOutput struct {
	View         ReadView         `json:"view"`
	AccountID    string           `json:"account_id"`
	MessageID    string           `json:"message_id"`
	ThreadID     string           `json:"thread_id"`
	Subject      string           `json:"subject"`
	From         Address          `json:"from"`
	Date         time.Time        `json:"date"`
	Participants []string         `json:"participants"`
	Body         string           `json:"body,omitempty"`
	Messages     []MessageSummary `json:"messages,omitempty"`
}

// ErrorOutput is the structured error envelope for robot mode.
type ErrorOutput struct {
	OK    bool       `json:"ok"`
	Error ErrorInfo  `json:"error"`
}

// ErrorInfo is the detail inside an ErrorOutput.
type ErrorInfo struct {
	Code     string `json:"code"`
	ExitCode int    `json:"exit_code"`
	Message  string `json:"message"`
	Hint     string `json:"hint,omitempty"`
}

// DoctorCheckResult is one health check from `letterhead doctor`.
type DoctorCheckResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // pass, warn, fail, skip
	Message string `json:"message"`
}

// DoctorOutput is the stable machine-readable contract for `letterhead doctor`.
type DoctorOutput struct {
	OK     bool                `json:"ok"`
	Checks []DoctorCheckResult `json:"checks"`
}

// SyncOutput is the stable machine-readable contract for `letterhead sync`.
type SyncOutput struct {
	Account   string  `json:"account"`
	Mode      string  `json:"mode"` // bootstrap, incremental, repair
	Added     int     `json:"added"`
	Deleted   int     `json:"deleted,omitempty"`
	Labels    int     `json:"labels,omitempty"`
	ElapsedMS int64   `json:"elapsed_ms"`
}

// SyncAllOutput wraps results when syncing multiple accounts.
type SyncAllOutput struct {
	Results []SyncOutput `json:"results"`
	Errors  []string     `json:"errors,omitempty"`
}

// AuthOutput is the stable machine-readable contract for `letterhead auth`.
type AuthOutput struct {
	Account       string `json:"account"`
	Authenticated bool   `json:"authenticated"`
	Method        string `json:"method,omitempty"`
}

// RebuildOutput is the stable machine-readable contract for `letterhead rebuild`.
type RebuildOutput struct {
	MessageCount int   `json:"message_count"`
	ElapsedMS    int64 `json:"elapsed_ms"`
}

// HelpCommand describes a subcommand for compact JSON help.
type HelpCommand struct {
	Name  string `json:"name"`
	Short string `json:"short"`
	Usage string `json:"usage,omitempty"`
}

// HelpOutput is the compact help envelope for robot mode.
type HelpOutput struct {
	Commands []HelpCommand `json:"commands"`
	Flags    []string      `json:"flags"`
}
