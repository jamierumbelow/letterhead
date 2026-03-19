package diagnostics

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// AuditEntry records a single find or read operation.
type AuditEntry struct {
	Timestamp   time.Time `json:"timestamp"`
	Command     string    `json:"command"`
	Query       string    `json:"query,omitempty"`
	ID          string    `json:"id,omitempty"`
	ResultCount int       `json:"result_count"`
}

// AuditLog writes append-only JSONL entries to a file.
type AuditLog struct {
	path string
}

// NewAuditLog creates an audit log at archiveRoot/audit.log.
func NewAuditLog(archiveRoot string) *AuditLog {
	return &AuditLog{path: filepath.Join(archiveRoot, "audit.log")}
}

// Log appends an audit entry. Errors are silently ignored since
// audit logging must never block the main operation.
func (a *AuditLog) Log(entry AuditEntry) {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}

	f, err := os.OpenFile(a.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()

	_ = json.NewEncoder(f).Encode(entry)
}
