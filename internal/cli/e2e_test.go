package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jamierumbelow/letterhead/internal/config"
	"github.com/jamierumbelow/letterhead/internal/store"
	"github.com/jamierumbelow/letterhead/pkg/types"
)

// testEnv holds an isolated letterhead environment for e2e tests.
type testEnv struct {
	t          *testing.T
	configHome string
	dataHome   string
	archiveRoot string
	dbPath     string
}

// setupTestEnv creates a fully initialized test environment with config,
// database, and test data pre-populated. It sets XDG env vars so that
// config.Load() and friends find the test paths.
func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()

	configHome := t.TempDir()
	dataHome := t.TempDir()
	archiveRoot := filepath.Join(t.TempDir(), "archive")

	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_DATA_HOME", dataHome)

	if err := os.MkdirAll(archiveRoot, 0o700); err != nil {
		t.Fatal(err)
	}

	env := &testEnv{
		t:           t,
		configHome:  configHome,
		dataHome:    dataHome,
		archiveRoot: archiveRoot,
		dbPath:      store.DatabasePath(archiveRoot),
	}

	return env
}

// writeConfig writes a config.toml and ensures the database schema exists.
func (e *testEnv) writeConfig(cfg config.Config) {
	e.t.Helper()
	cfg.ArchiveRoot = e.archiveRoot

	if cfg.SyncMode == "" {
		cfg.SyncMode = config.SyncModeRecent
	}
	if cfg.RecentWindowWeeks == 0 {
		cfg.RecentWindowWeeks = 12
	}
	if cfg.SchedulerCadence == "" {
		cfg.SchedulerCadence = "1h"
	}

	if err := config.Save(cfg); err != nil {
		e.t.Fatal(err)
	}

	// Ensure DB + schema exist.
	db, err := store.Open(e.dbPath)
	if err != nil {
		e.t.Fatal(err)
	}
	db.Close()
}

// seedMessages inserts test messages into the database via the store layer.
func (e *testEnv) seedMessages(accountID string, msgs []types.Message) {
	e.t.Helper()

	db, err := store.Open(e.dbPath)
	if err != nil {
		e.t.Fatal(err)
	}
	defer db.Close()

	s := store.NewWithAccount(db, accountID)
	ctx := context.Background()

	for i := range msgs {
		if err := s.UpsertMessage(ctx, &msgs[i]); err != nil {
			e.t.Fatalf("seed message %q: %v", msgs[i].GmailID, err)
		}
	}
}

// seedSyncState inserts sync state for an account.
func (e *testEnv) seedSyncState(st *store.SyncState) {
	e.t.Helper()

	db, err := store.Open(e.dbPath)
	if err != nil {
		e.t.Fatal(err)
	}
	defer db.Close()

	s := store.New(db)
	if err := s.SetSyncState(context.Background(), st); err != nil {
		e.t.Fatalf("seed sync state: %v", err)
	}
}

// run executes a CLI command and returns stdout, stderr, and any error.
func (e *testEnv) run(args ...string) (stdout, stderr string, err error) {
	e.t.Helper()

	cmd := NewRootCommand()
	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cmd.SetArgs(args)

	err = cmd.Execute()
	return outBuf.String(), errBuf.String(), err
}

// mustRun is like run but fails the test on error.
func (e *testEnv) mustRun(args ...string) string {
	e.t.Helper()
	stdout, stderr, err := e.run(args...)
	if err != nil {
		e.t.Fatalf("command %v failed: %v\nstdout: %s\nstderr: %s", args, err, stdout, stderr)
	}
	return stdout
}

// runJSON executes a command with --json and decodes the output.
func (e *testEnv) runJSON(v any, args ...string) error {
	e.t.Helper()
	args = append(args, "--json")
	stdout := e.mustRun(args...)
	return json.Unmarshal([]byte(stdout), v)
}

// --- Test fixtures ---

var (
	now       = time.Now().UTC().Truncate(time.Second)
	hourAgo   = now.Add(-time.Hour)
	twoHrsAgo = now.Add(-2 * time.Hour)
)

func aliceMessages() []types.Message {
	return []types.Message{
		{
			GmailID:      "alice-1",
			ThreadID:     "thread-a1",
			HistoryID:    100,
			InternalDate: twoHrsAgo.UnixMilli(),
			ReceivedAt:   twoHrsAgo,
			Subject:      "Project update from Alice",
			Snippet:      "Here is the latest update...",
			From:         types.Address{Email: "alice@test.com", Name: "Alice Test"},
			To:           []types.Address{{Email: "bob@test.com", Name: "Bob Test"}},
			PlainBody:    "Hello, here is the project update from Alice.",
			Labels:       []string{"INBOX"},
		},
		{
			GmailID:      "alice-2",
			ThreadID:     "thread-a1",
			HistoryID:    101,
			InternalDate: hourAgo.UnixMilli(),
			ReceivedAt:   hourAgo,
			Subject:      "Re: Project update from Alice",
			Snippet:      "Thanks for the update...",
			From:         types.Address{Email: "bob@test.com", Name: "Bob Test"},
			To:           []types.Address{{Email: "alice@test.com", Name: "Alice Test"}},
			PlainBody:    "Thanks for the update, Alice.",
			Labels:       []string{"INBOX"},
		},
		{
			GmailID:      "alice-3",
			ThreadID:     "thread-a1",
			HistoryID:    102,
			InternalDate: now.UnixMilli(),
			ReceivedAt:   now,
			Subject:      "Re: Project update from Alice",
			Snippet:      "No problem, glad to help...",
			From:         types.Address{Email: "alice@test.com", Name: "Alice Test"},
			To:           []types.Address{{Email: "bob@test.com", Name: "Bob Test"}},
			PlainBody:    "No problem! Let me know if you need anything else.",
			Labels:       []string{"INBOX"},
		},
	}
}

func bobMessages() []types.Message {
	return []types.Message{
		{
			GmailID:      "bob-1",
			ThreadID:     "thread-b1",
			HistoryID:    200,
			InternalDate: hourAgo.UnixMilli(),
			ReceivedAt:   hourAgo,
			Subject:      "Meeting notes from Bob",
			Snippet:      "Here are the meeting notes...",
			From:         types.Address{Email: "bob@test.com", Name: "Bob Test"},
			To:           []types.Address{{Email: "team@test.com", Name: "Team"}},
			PlainBody:    "Meeting notes: discussed roadmap and priorities.",
			Labels:       []string{"INBOX", "IMPORTANT"},
		},
		{
			GmailID:      "bob-2",
			ThreadID:     "thread-b1",
			HistoryID:    201,
			InternalDate: now.UnixMilli(),
			ReceivedAt:   now,
			Subject:      "Re: Meeting notes from Bob",
			Snippet:      "Action items attached...",
			From:         types.Address{Email: "bob@test.com", Name: "Bob Test"},
			To:           []types.Address{{Email: "team@test.com", Name: "Team"}},
			PlainBody:    "Action items from the meeting are attached.",
			Labels:       []string{"INBOX"},
		},
	}
}

// setupPopulatedEnv creates a test env with alice@test.com configured and
// seeded with alice + bob messages. This is the most common starting point.
func setupPopulatedEnv(t *testing.T) *testEnv {
	t.Helper()

	env := setupTestEnv(t)
	env.writeConfig(config.Config{
		Accounts: []config.AccountConfig{
			{Email: "alice@test.com", AuthMethod: config.AuthMethodAppPassword},
		},
		DefaultAccount: "alice@test.com",
	})

	env.seedMessages("alice@test.com", aliceMessages())
	env.seedMessages("alice@test.com", bobMessages())

	syncTime := now
	env.seedSyncState(&store.SyncState{
		AccountID:         "alice@test.com",
		HistoryID:         102,
		BootstrapComplete: true,
		MessagesSynced:    5,
		LastSyncAt:        &syncTime,
		AuthMethod:        "apppassword",
	})

	return env
}

// ===================================================================
// Init tests
// ===================================================================

func TestE2E_InitCreatesConfigAndDatabase(t *testing.T) {


	env := setupTestEnv(t)

	stdout := env.mustRun("init", "--archive-root", env.archiveRoot)

	if !strings.Contains(stdout, "Letterhead initialized") {
		t.Fatalf("expected initialization message, got: %s", stdout)
	}

	// Config should exist
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() = %v", err)
	}
	if cfg.ArchiveRoot != env.archiveRoot {
		t.Fatalf("ArchiveRoot = %q, want %q", cfg.ArchiveRoot, env.archiveRoot)
	}

	// DB should exist
	if _, err := os.Stat(env.dbPath); err != nil {
		t.Fatalf("database not created: %v", err)
	}
}

func TestE2E_InitIsIdempotent(t *testing.T) {


	env := setupTestEnv(t)

	env.mustRun("init", "--archive-root", env.archiveRoot)
	stdout := env.mustRun("init", "--archive-root", env.archiveRoot)

	if !strings.Contains(stdout, "already initialized") {
		t.Fatalf("expected 'already initialized', got: %s", stdout)
	}
}

// ===================================================================
// Status tests
// ===================================================================

func TestE2E_StatusBeforeInit(t *testing.T) {


	env := setupTestEnv(t)

	var status types.StatusOutput
	if err := env.runJSON(&status, "status"); err != nil {
		t.Fatal(err)
	}

	if status.DBHealth != "not initialized" {
		t.Fatalf("DBHealth = %q, want 'not initialized'", status.DBHealth)
	}
	if status.Account != "not configured" {
		t.Fatalf("Account = %q, want 'not configured'", status.Account)
	}
}

func TestE2E_StatusAfterInit(t *testing.T) {


	env := setupTestEnv(t)
	env.writeConfig(config.Config{})

	var status types.StatusOutput
	if err := env.runJSON(&status, "status"); err != nil {
		t.Fatal(err)
	}

	if status.DBHealth != "ok" {
		t.Fatalf("DBHealth = %q, want 'ok'", status.DBHealth)
	}
	if status.MessageCount != 0 {
		t.Fatalf("MessageCount = %d, want 0", status.MessageCount)
	}
}

func TestE2E_StatusWithData(t *testing.T) {


	env := setupPopulatedEnv(t)

	var status types.StatusOutput
	if err := env.runJSON(&status, "status"); err != nil {
		t.Fatal(err)
	}

	if status.DBHealth != "ok" {
		t.Fatalf("DBHealth = %q, want 'ok'", status.DBHealth)
	}
	if status.MessageCount != 5 {
		t.Fatalf("MessageCount = %d, want 5", status.MessageCount)
	}
	if status.ThreadCount != 2 {
		t.Fatalf("ThreadCount = %d, want 2", status.ThreadCount)
	}
	if !status.BootstrapComplete {
		t.Fatal("BootstrapComplete = false, want true")
	}
	if !strings.Contains(status.Account, "alice@test.com") {
		t.Fatalf("Account = %q, want to contain 'alice@test.com'", status.Account)
	}
}

// ===================================================================
// Accounts tests
// ===================================================================

func TestE2E_AccountsListEmpty(t *testing.T) {


	env := setupTestEnv(t)
	env.writeConfig(config.Config{})

	stdout := env.mustRun("accounts", "list", "--json")

	// Should be an empty array
	var accounts []accountInfo
	if err := json.Unmarshal([]byte(stdout), &accounts); err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 0 {
		t.Fatalf("len(accounts) = %d, want 0", len(accounts))
	}
}

func TestE2E_AccountsListSingle(t *testing.T) {


	env := setupPopulatedEnv(t)

	stdout := env.mustRun("accounts", "list", "--json")

	var accounts []accountInfo
	if err := json.Unmarshal([]byte(stdout), &accounts); err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 {
		t.Fatalf("len(accounts) = %d, want 1", len(accounts))
	}
	if accounts[0].Email != "alice@test.com" {
		t.Fatalf("Email = %q, want 'alice@test.com'", accounts[0].Email)
	}
	if accounts[0].AuthMethod != "apppassword" {
		t.Fatalf("AuthMethod = %q, want 'apppassword'", accounts[0].AuthMethod)
	}
	if !accounts[0].IsDefault {
		t.Fatal("IsDefault = false, want true")
	}
	if accounts[0].MessagesSynced != 5 {
		t.Fatalf("MessagesSynced = %d, want 5", accounts[0].MessagesSynced)
	}
}

func TestE2E_AccountsListMultiple(t *testing.T) {


	env := setupTestEnv(t)
	env.writeConfig(config.Config{
		Accounts: []config.AccountConfig{
			{Email: "alice@test.com", AuthMethod: config.AuthMethodAppPassword},
			{Email: "bob@test.com", AuthMethod: config.AuthMethodOAuth},
		},
		DefaultAccount: "alice@test.com",
	})

	stdout := env.mustRun("accounts", "list", "--json")

	var accounts []accountInfo
	if err := json.Unmarshal([]byte(stdout), &accounts); err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 2 {
		t.Fatalf("len(accounts) = %d, want 2", len(accounts))
	}
	if accounts[0].Email != "alice@test.com" {
		t.Fatalf("accounts[0].Email = %q", accounts[0].Email)
	}
	if accounts[1].Email != "bob@test.com" {
		t.Fatalf("accounts[1].Email = %q", accounts[1].Email)
	}
	if !accounts[0].IsDefault {
		t.Fatal("alice should be default")
	}
	if accounts[1].IsDefault {
		t.Fatal("bob should not be default")
	}
}

func TestE2E_AccountsDefault(t *testing.T) {


	env := setupTestEnv(t)
	env.writeConfig(config.Config{
		Accounts: []config.AccountConfig{
			{Email: "alice@test.com", AuthMethod: config.AuthMethodAppPassword},
			{Email: "bob@test.com", AuthMethod: config.AuthMethodOAuth},
		},
		DefaultAccount: "alice@test.com",
	})

	env.mustRun("accounts", "default", "bob@test.com")

	// Verify via accounts list
	stdout := env.mustRun("accounts", "list", "--json")
	var accounts []accountInfo
	if err := json.Unmarshal([]byte(stdout), &accounts); err != nil {
		t.Fatal(err)
	}

	for _, a := range accounts {
		if a.Email == "bob@test.com" && !a.IsDefault {
			t.Fatal("bob should now be default")
		}
		if a.Email == "alice@test.com" && a.IsDefault {
			t.Fatal("alice should no longer be default")
		}
	}
}

func TestE2E_AccountsDefaultUnknownEmail(t *testing.T) {


	env := setupTestEnv(t)
	env.writeConfig(config.Config{
		Accounts: []config.AccountConfig{
			{Email: "alice@test.com", AuthMethod: config.AuthMethodAppPassword},
		},
	})

	_, _, err := env.run("accounts", "default", "unknown@test.com")
	if err == nil {
		t.Fatal("expected error for unknown email")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error = %v, want 'not found'", err)
	}
}

// ===================================================================
// Find tests
// ===================================================================

func TestE2E_FindAll(t *testing.T) {


	env := setupPopulatedEnv(t)

	var output types.FindOutput
	if err := env.runJSON(&output, "find"); err != nil {
		t.Fatal(err)
	}

	if output.TotalCount != 2 {
		t.Fatalf("TotalCount = %d, want 2 (two threads)", output.TotalCount)
	}

	threadIDs := make(map[string]bool)
	for _, r := range output.Results {
		threadIDs[r.ThreadID] = true
	}
	if !threadIDs["thread-a1"] {
		t.Fatal("missing thread-a1")
	}
	if !threadIDs["thread-b1"] {
		t.Fatal("missing thread-b1")
	}
}

func TestE2E_FindFreetextSearch(t *testing.T) {


	env := setupPopulatedEnv(t)

	var output types.FindOutput
	if err := env.runJSON(&output, "find", "meeting"); err != nil {
		t.Fatal(err)
	}

	if output.TotalCount != 1 {
		t.Fatalf("TotalCount = %d, want 1", output.TotalCount)
	}
	if output.Results[0].ThreadID != "thread-b1" {
		t.Fatalf("ThreadID = %q, want 'thread-b1'", output.Results[0].ThreadID)
	}
}

func TestE2E_FindByFrom(t *testing.T) {


	env := setupPopulatedEnv(t)

	var output types.FindOutput
	if err := env.runJSON(&output, "find", "--from", "alice@test.com"); err != nil {
		t.Fatal(err)
	}

	if output.TotalCount < 1 {
		t.Fatalf("TotalCount = %d, want >= 1", output.TotalCount)
	}

	for _, r := range output.Results {
		if r.ThreadID != "thread-a1" {
			t.Fatalf("unexpected thread %q for --from alice", r.ThreadID)
		}
	}
}

func TestE2E_FindByLabel(t *testing.T) {


	env := setupPopulatedEnv(t)

	var output types.FindOutput
	if err := env.runJSON(&output, "find", "--label", "IMPORTANT"); err != nil {
		t.Fatal(err)
	}

	if output.TotalCount != 1 {
		t.Fatalf("TotalCount = %d, want 1", output.TotalCount)
	}
	if output.Results[0].ThreadID != "thread-b1" {
		t.Fatalf("ThreadID = %q, want 'thread-b1'", output.Results[0].ThreadID)
	}
}

func TestE2E_FindBySubject(t *testing.T) {


	env := setupPopulatedEnv(t)

	var output types.FindOutput
	if err := env.runJSON(&output, "find", "--subject", "Meeting"); err != nil {
		t.Fatal(err)
	}

	if output.TotalCount != 1 {
		t.Fatalf("TotalCount = %d, want 1", output.TotalCount)
	}
	if !strings.Contains(output.Results[0].Subject, "Meeting") {
		t.Fatalf("Subject = %q, expected to contain 'Meeting'", output.Results[0].Subject)
	}
}

func TestE2E_FindNoResults(t *testing.T) {


	env := setupPopulatedEnv(t)

	var output types.FindOutput
	if err := env.runJSON(&output, "find", "xyznonexistent"); err != nil {
		t.Fatal(err)
	}

	if output.TotalCount != 0 {
		t.Fatalf("TotalCount = %d, want 0", output.TotalCount)
	}
}

func TestE2E_FindWithLimit(t *testing.T) {


	env := setupPopulatedEnv(t)

	var output types.FindOutput
	if err := env.runJSON(&output, "find", "--limit", "1"); err != nil {
		t.Fatal(err)
	}

	if len(output.Results) != 1 {
		t.Fatalf("len(Results) = %d, want 1", len(output.Results))
	}
}

func TestE2E_FindThreadHasCorrectMessageCount(t *testing.T) {


	env := setupPopulatedEnv(t)

	var output types.FindOutput
	if err := env.runJSON(&output, "find"); err != nil {
		t.Fatal(err)
	}

	for _, r := range output.Results {
		switch r.ThreadID {
		case "thread-a1":
			if r.MessageCount != 3 {
				t.Fatalf("thread-a1 MessageCount = %d, want 3", r.MessageCount)
			}
		case "thread-b1":
			if r.MessageCount != 2 {
				t.Fatalf("thread-b1 MessageCount = %d, want 2", r.MessageCount)
			}
		}
	}
}

// ===================================================================
// Read tests
// ===================================================================

func TestE2E_ReadMessageByID(t *testing.T) {


	env := setupPopulatedEnv(t)

	var output types.ReadOutput
	if err := env.runJSON(&output, "read", "alice-1"); err != nil {
		t.Fatal(err)
	}

	if output.MessageID != "alice-1" {
		t.Fatalf("MessageID = %q, want 'alice-1'", output.MessageID)
	}
	if output.Subject != "Project update from Alice" {
		t.Fatalf("Subject = %q", output.Subject)
	}
	if output.From.Email != "alice@test.com" {
		t.Fatalf("From.Email = %q", output.From.Email)
	}
	if output.ThreadID != "thread-a1" {
		t.Fatalf("ThreadID = %q, want 'thread-a1'", output.ThreadID)
	}
}

func TestE2E_ReadMessageWithTextView(t *testing.T) {


	env := setupPopulatedEnv(t)

	var output types.ReadOutput
	if err := env.runJSON(&output, "read", "alice-1", "--view", "text"); err != nil {
		t.Fatal(err)
	}

	if output.Body == "" {
		t.Fatal("Body is empty, expected text content")
	}
	if !strings.Contains(output.Body, "project update from Alice") {
		t.Fatalf("Body = %q, expected to contain message text", output.Body)
	}
}

func TestE2E_ReadThread(t *testing.T) {


	env := setupPopulatedEnv(t)

	var output types.ReadOutput
	if err := env.runJSON(&output, "read", "thread-a1", "--thread"); err != nil {
		t.Fatal(err)
	}

	if output.ThreadID != "thread-a1" {
		t.Fatalf("ThreadID = %q, want 'thread-a1'", output.ThreadID)
	}
	if len(output.Messages) != 3 {
		t.Fatalf("len(Messages) = %d, want 3", len(output.Messages))
	}

	// Messages should be in chronological order
	msgIDs := make([]string, len(output.Messages))
	for i, m := range output.Messages {
		msgIDs[i] = m.MessageID
	}
	if msgIDs[0] != "alice-1" || msgIDs[1] != "alice-2" || msgIDs[2] != "alice-3" {
		t.Fatalf("message order = %v, want [alice-1, alice-2, alice-3]", msgIDs)
	}
}

func TestE2E_ReadThreadWithTextView(t *testing.T) {


	env := setupPopulatedEnv(t)

	var output types.ReadOutput
	if err := env.runJSON(&output, "read", "thread-a1", "--thread", "--view", "text"); err != nil {
		t.Fatal(err)
	}

	if output.Body == "" {
		t.Fatal("Body is empty for thread text view")
	}
	// Should contain content from multiple messages joined
	if !strings.Contains(output.Body, "project update from Alice") {
		t.Fatalf("Body missing alice-1 content: %q", output.Body)
	}
	if !strings.Contains(output.Body, "Thanks for the update") {
		t.Fatalf("Body missing alice-2 content: %q", output.Body)
	}
}

func TestE2E_ReadThreadViaMessageID(t *testing.T) {


	env := setupPopulatedEnv(t)

	// Pass a message ID with --thread — should resolve to the thread
	var output types.ReadOutput
	if err := env.runJSON(&output, "read", "alice-2", "--thread"); err != nil {
		t.Fatal(err)
	}

	if output.ThreadID != "thread-a1" {
		t.Fatalf("ThreadID = %q, want 'thread-a1'", output.ThreadID)
	}
	if len(output.Messages) != 3 {
		t.Fatalf("len(Messages) = %d, want 3", len(output.Messages))
	}
}

func TestE2E_ReadNotFound(t *testing.T) {


	env := setupPopulatedEnv(t)

	_, _, err := env.run("read", "nonexistent-id", "--json")
	if err == nil {
		t.Fatal("expected error for nonexistent message")
	}
}

// ===================================================================
// Multi-account tests
// ===================================================================

func TestE2E_MultiAccountStatus(t *testing.T) {


	env := setupTestEnv(t)
	env.writeConfig(config.Config{
		Accounts: []config.AccountConfig{
			{Email: "alice@test.com", AuthMethod: config.AuthMethodAppPassword},
			{Email: "bob@test.com", AuthMethod: config.AuthMethodOAuth},
		},
		DefaultAccount: "alice@test.com",
	})

	env.seedMessages("alice@test.com", aliceMessages())
	env.seedMessages("bob@test.com", bobMessages())

	syncTime := now
	env.seedSyncState(&store.SyncState{
		AccountID:         "alice@test.com",
		HistoryID:         102,
		BootstrapComplete: true,
		MessagesSynced:    3,
		LastSyncAt:        &syncTime,
		AuthMethod:        "apppassword",
	})
	env.seedSyncState(&store.SyncState{
		AccountID:         "bob@test.com",
		HistoryID:         201,
		BootstrapComplete: true,
		MessagesSynced:    2,
		LastSyncAt:        &syncTime,
		AuthMethod:        "oauth",
	})

	var status types.StatusOutput
	if err := env.runJSON(&status, "status"); err != nil {
		t.Fatal(err)
	}

	if len(status.Accounts) != 2 {
		t.Fatalf("len(Accounts) = %d, want 2", len(status.Accounts))
	}
	if status.MessageCount != 5 {
		t.Fatalf("MessageCount = %d, want 5", status.MessageCount)
	}
}

func TestE2E_MultiAccountFindCrossAccount(t *testing.T) {


	env := setupTestEnv(t)
	env.writeConfig(config.Config{
		Accounts: []config.AccountConfig{
			{Email: "alice@test.com", AuthMethod: config.AuthMethodAppPassword},
			{Email: "bob@test.com", AuthMethod: config.AuthMethodOAuth},
		},
		DefaultAccount: "alice@test.com",
	})

	env.seedMessages("alice@test.com", aliceMessages())
	env.seedMessages("bob@test.com", bobMessages())

	// Without --account, should find threads from both accounts
	var output types.FindOutput
	if err := env.runJSON(&output, "find"); err != nil {
		t.Fatal(err)
	}

	if output.TotalCount != 2 {
		t.Fatalf("TotalCount = %d, want 2", output.TotalCount)
	}
}

// ===================================================================
// Error handling tests
// ===================================================================

func TestE2E_ConflictingOutputModes(t *testing.T) {


	env := setupTestEnv(t)
	env.writeConfig(config.Config{})

	_, _, err := env.run("status", "--json", "--jsonl")
	if err == nil {
		t.Fatal("expected error for conflicting output modes")
	}
}

func TestE2E_ReadRequiresArgument(t *testing.T) {


	env := setupPopulatedEnv(t)

	_, _, err := env.run("read", "--json")
	if err == nil {
		t.Fatal("expected error when read has no argument")
	}
}

// ===================================================================
// Legacy config migration tests
// ===================================================================

func TestE2E_LegacyConfigMigration(t *testing.T) {


	env := setupTestEnv(t)

	// Write a legacy-style config with flat account_email/auth_method
	configDir := filepath.Join(env.configHome, "letterhead")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}

	legacyConfig := `archive_root = "` + env.archiveRoot + `"
account_email = "legacy@test.com"
auth_method = "apppassword"
sync_mode = "recent"
recent_window_weeks = 12
scheduler_cadence = "1h"
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(legacyConfig), 0o600); err != nil {
		t.Fatal(err)
	}

	// Ensure DB exists
	db, err := store.Open(env.dbPath)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	// status should work — the migration should convert the flat fields to accounts[]
	var status types.StatusOutput
	if err := env.runJSON(&status, "status"); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(status.Account, "legacy@test.com") {
		t.Fatalf("Account = %q, want to contain 'legacy@test.com'", status.Account)
	}

	// accounts list should show the migrated account
	stdout := env.mustRun("accounts", "list", "--json")
	var accounts []accountInfo
	if err := json.Unmarshal([]byte(stdout), &accounts); err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 {
		t.Fatalf("len(accounts) = %d, want 1", len(accounts))
	}
	if accounts[0].Email != "legacy@test.com" {
		t.Fatalf("Email = %q, want 'legacy@test.com'", accounts[0].Email)
	}
}

// ===================================================================
// Output mode tests
// ===================================================================

func TestE2E_StatusHumanOutput(t *testing.T) {


	env := setupPopulatedEnv(t)

	stdout := env.mustRun("status")

	// Human output should contain readable text, not JSON
	if strings.HasPrefix(strings.TrimSpace(stdout), "{") {
		t.Fatal("expected human output, got JSON")
	}
	if !strings.Contains(stdout, "alice@test.com") {
		t.Fatalf("human status should mention account, got: %s", stdout)
	}
}

func TestE2E_FindHumanOutput(t *testing.T) {


	env := setupPopulatedEnv(t)

	stdout := env.mustRun("find")

	if strings.HasPrefix(strings.TrimSpace(stdout), "{") {
		t.Fatal("expected human output, got JSON")
	}
}
