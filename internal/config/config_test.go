package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultUsesXDGPaths(t *testing.T) {
	configHome := t.TempDir()
	dataHome := t.TempDir()

	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_DATA_HOME", dataHome)

	cfg, err := Default()
	if err != nil {
		t.Fatalf("Default() error = %v", err)
	}

	if cfg.ArchiveRoot != filepath.Join(dataHome, "letterhead", "archive") {
		t.Fatalf("ArchiveRoot = %q", cfg.ArchiveRoot)
	}

	if cfg.SyncMode != SyncModeRecent {
		t.Fatalf("SyncMode = %q", cfg.SyncMode)
	}

	if cfg.RecentWindowWeeks != defaultRecentWindowWeeks {
		t.Fatalf("RecentWindowWeeks = %d", cfg.RecentWindowWeeks)
	}

	if cfg.SchedulerCadence != defaultSchedulerCadence {
		t.Fatalf("SchedulerCadence = %q", cfg.SchedulerCadence)
	}

	if len(cfg.Accounts) != 0 {
		t.Fatalf("Accounts = %v, want empty", cfg.Accounts)
	}

	configPath, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath() error = %v", err)
	}

	if configPath != filepath.Join(configHome, "letterhead", "config.toml") {
		t.Fatalf("ConfigPath() = %q", configPath)
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	configHome := t.TempDir()
	dataHome := t.TempDir()

	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_DATA_HOME", dataHome)

	cfg := Config{
		ArchiveRoot: filepath.Join(dataHome, "custom-archive"),
		Accounts: []AccountConfig{
			{Email: "user@example.com", AuthMethod: AuthMethodOAuth},
		},
		DefaultAccount:    "user@example.com",
		SyncMode:          SyncModeInbox,
		RecentWindowWeeks: 4,
		SchedulerCadence:  "30m",
	}

	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	configPath, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath() error = %v", err)
	}

	fileInfo, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("os.Stat(%q) error = %v", configPath, err)
	}

	if fileInfo.Mode().Perm() != 0o600 {
		t.Fatalf("config mode = %o, want 600", fileInfo.Mode().Perm())
	}

	dirInfo, err := os.Stat(filepath.Dir(configPath))
	if err != nil {
		t.Fatalf("os.Stat(%q) error = %v", filepath.Dir(configPath), err)
	}

	if dirInfo.Mode().Perm() != 0o700 {
		t.Fatalf("config dir mode = %o, want 700", dirInfo.Mode().Perm())
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(loaded.Accounts) != 1 {
		t.Fatalf("Accounts len = %d, want 1", len(loaded.Accounts))
	}
	if loaded.Accounts[0].Email != "user@example.com" {
		t.Fatalf("Accounts[0].Email = %q", loaded.Accounts[0].Email)
	}
	if loaded.SyncMode != SyncModeInbox {
		t.Fatalf("SyncMode = %q", loaded.SyncMode)
	}
	// Backward compat: AccountEmail should be populated from Accounts[0].
	if loaded.AccountEmail != "user@example.com" {
		t.Fatalf("AccountEmail = %q, want back-populated", loaded.AccountEmail)
	}
}

func TestSaveDoesNotWriteDeprecatedFields(t *testing.T) {
	configHome := t.TempDir()
	dataHome := t.TempDir()

	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_DATA_HOME", dataHome)

	cfg := Config{
		ArchiveRoot: filepath.Join(dataHome, "archive"),
		Accounts: []AccountConfig{
			{Email: "a@b.com", AuthMethod: AuthMethodOAuth},
		},
		SyncMode:          SyncModeRecent,
		RecentWindowWeeks: 12,
		SchedulerCadence:  "1h",
	}

	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	configPath, _ := ConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}

	contents := string(data)
	if strings.Contains(contents, "account_email") {
		t.Fatalf("saved config contains deprecated account_email field:\n%s", contents)
	}
}

func TestLoadMigratesLegacyFlatFields(t *testing.T) {
	configHome := t.TempDir()
	dataHome := t.TempDir()

	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_DATA_HOME", dataHome)

	// Write a legacy-format config file directly.
	dir := filepath.Join(configHome, "letterhead")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := `archive_root = "` + filepath.Join(dataHome, "archive") + `"
account_email = "old@example.com"
auth_method = "apppassword"
sync_mode = "inbox"
recent_window_weeks = 4
scheduler_cadence = "30m"
`
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.Accounts) != 1 {
		t.Fatalf("Accounts len = %d, want 1", len(cfg.Accounts))
	}
	if cfg.Accounts[0].Email != "old@example.com" {
		t.Fatalf("Accounts[0].Email = %q", cfg.Accounts[0].Email)
	}
	if cfg.Accounts[0].AuthMethod != AuthMethodAppPassword {
		t.Fatalf("Accounts[0].AuthMethod = %q", cfg.Accounts[0].AuthMethod)
	}
	if cfg.DefaultAccount != "old@example.com" {
		t.Fatalf("DefaultAccount = %q", cfg.DefaultAccount)
	}
	// Backward compat fields should be populated.
	if cfg.AccountEmail != "old@example.com" {
		t.Fatalf("AccountEmail = %q, want back-populated", cfg.AccountEmail)
	}
	if cfg.AuthMethod != AuthMethodAppPassword {
		t.Fatalf("AuthMethod = %q, want back-populated", cfg.AuthMethod)
	}
}

func TestLoadReturnsNotExistForMissingConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	_, err := Load()
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Load() error = %v, want os.ErrNotExist", err)
	}
}

func TestValidateRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	archiveRoot := t.TempDir()

	tests := []struct {
		name string
		cfg  Config
		want error
	}{
		{
			name: "invalid account auth method",
			cfg: Config{
				ArchiveRoot: archiveRoot,
				Accounts: []AccountConfig{
					{Email: "a@b.com", AuthMethod: "broken"},
				},
				SyncMode:          SyncModeRecent,
				RecentWindowWeeks: defaultRecentWindowWeeks,
				SchedulerCadence:  defaultSchedulerCadence,
			},
			want: ErrInvalidAuthMethod,
		},
		{
			name: "invalid sync mode",
			cfg: Config{
				ArchiveRoot:       archiveRoot,
				SyncMode:          "broken",
				RecentWindowWeeks: defaultRecentWindowWeeks,
				SchedulerCadence:  defaultSchedulerCadence,
			},
			want: ErrInvalidSyncMode,
		},
		{
			name: "invalid recent window",
			cfg: Config{
				ArchiveRoot:       archiveRoot,
				SyncMode:          SyncModeRecent,
				RecentWindowWeeks: -1,
				SchedulerCadence:  defaultSchedulerCadence,
			},
			want: ErrInvalidRecentWindow,
		},
		{
			name: "invalid cadence",
			cfg: Config{
				ArchiveRoot:       archiveRoot,
				SyncMode:          SyncModeRecent,
				RecentWindowWeeks: defaultRecentWindowWeeks,
				SchedulerCadence:  "not-a-duration",
			},
			want: ErrInvalidCadence,
		},
		{
			name: "duplicate account emails",
			cfg: Config{
				ArchiveRoot: archiveRoot,
				Accounts: []AccountConfig{
					{Email: "a@b.com", AuthMethod: AuthMethodOAuth},
					{Email: "a@b.com", AuthMethod: AuthMethodOAuth},
				},
				SyncMode:          SyncModeRecent,
				RecentWindowWeeks: defaultRecentWindowWeeks,
				SchedulerCadence:  defaultSchedulerCadence,
			},
			want: ErrDuplicateAccount,
		},
		{
			name: "empty account email",
			cfg: Config{
				ArchiveRoot: archiveRoot,
				Accounts: []AccountConfig{
					{Email: "", AuthMethod: AuthMethodOAuth},
				},
				SyncMode:          SyncModeRecent,
				RecentWindowWeeks: defaultRecentWindowWeeks,
				SchedulerCadence:  defaultSchedulerCadence,
			},
			want: ErrAccountEmailRequired,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.cfg.Validate()
			if !errors.Is(err, tt.want) {
				t.Fatalf("Validate() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestValidateAcceptsEmptyAccounts(t *testing.T) {
	cfg := Config{
		ArchiveRoot:       t.TempDir(),
		SyncMode:          SyncModeRecent,
		RecentWindowWeeks: 12,
		SchedulerCadence:  "1h",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestResolveAccount(t *testing.T) {
	cfg := Config{
		Accounts: []AccountConfig{
			{Email: "alice@example.com", AuthMethod: AuthMethodOAuth},
			{Email: "bob@example.com", AuthMethod: AuthMethodAppPassword},
		},
		DefaultAccount: "alice@example.com",
	}

	// Explicit flag value.
	acct, err := cfg.ResolveAccount("bob@example.com")
	if err != nil {
		t.Fatalf("ResolveAccount(bob) error = %v", err)
	}
	if acct.Email != "bob@example.com" {
		t.Fatalf("got %q", acct.Email)
	}

	// Default account.
	acct, err = cfg.ResolveAccount("")
	if err != nil {
		t.Fatalf("ResolveAccount('') error = %v", err)
	}
	if acct.Email != "alice@example.com" {
		t.Fatalf("got %q, want default alice", acct.Email)
	}

	// Not found.
	_, err = cfg.ResolveAccount("nobody@example.com")
	if !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("error = %v, want ErrAccountNotFound", err)
	}

	// Ambiguous: multiple accounts, no default.
	cfg.DefaultAccount = ""
	_, err = cfg.ResolveAccount("")
	if !errors.Is(err, ErrAmbiguousAccount) {
		t.Fatalf("error = %v, want ErrAmbiguousAccount", err)
	}

	// Single account, no default, no flag: auto-resolve.
	cfg.Accounts = cfg.Accounts[:1]
	acct, err = cfg.ResolveAccount("")
	if err != nil {
		t.Fatalf("single account error = %v", err)
	}
	if acct.Email != "alice@example.com" {
		t.Fatalf("got %q", acct.Email)
	}

	// No accounts at all.
	cfg.Accounts = nil
	_, err = cfg.ResolveAccount("")
	if !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("error = %v, want ErrAccountNotFound", err)
	}
}

func TestAccountByEmail(t *testing.T) {
	cfg := Config{
		Accounts: []AccountConfig{
			{Email: "Alice@Example.com", AuthMethod: AuthMethodOAuth},
		},
	}

	// Case-insensitive lookup.
	acct := cfg.AccountByEmail("alice@example.com")
	if acct == nil {
		t.Fatal("expected non-nil")
	}
	if acct.Email != "Alice@Example.com" {
		t.Fatalf("got %q", acct.Email)
	}

	// Not found.
	if cfg.AccountByEmail("nobody@example.com") != nil {
		t.Fatal("expected nil for missing account")
	}
}

func TestTokenPathUsesHashedAccountName(t *testing.T) {
	configHome := t.TempDir()

	t.Setenv("XDG_CONFIG_HOME", configHome)

	tokenPath, err := TokenPath("User@Example.com")
	if err != nil {
		t.Fatalf("TokenPath() error = %v", err)
	}

	expected := filepath.Join(configHome, "letterhead", "token_b4c9a289323b21a01c3e940f150eb9b8c542587f1abfd8f0e1cc1ffc5e475514.json")
	if tokenPath != expected {
		t.Fatalf("TokenPath() = %q, want %q", tokenPath, expected)
	}
}

func TestTokenPathRejectsBlankAccount(t *testing.T) {
	t.Parallel()

	_, err := TokenPath("   ")
	if !errors.Is(err, ErrAccountEmailRequired) {
		t.Fatalf("TokenPath() error = %v, want %v", err, ErrAccountEmailRequired)
	}
}
