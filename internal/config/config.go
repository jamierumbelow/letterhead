package config

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

const (
	defaultRecentWindowWeeks = 12
	defaultSchedulerCadence  = "1h"
	configDirName            = "letterhead"
	configFileName           = "config.toml"
	tokenFilePrefix          = "token_"
	tokenFileSuffix          = ".json"
)

type SyncMode string

const (
	SyncModeInbox  SyncMode = "inbox"
	SyncModeRecent SyncMode = "recent"
	SyncModeFull   SyncMode = "full"
)

type AuthMethod string

const (
	AuthMethodOAuth       AuthMethod = "oauth"
	AuthMethodAppPassword AuthMethod = "apppassword"
)

var (
	ErrAccountEmailRequired = errors.New("account email is required")
	ErrInvalidSyncMode      = errors.New("sync mode must be inbox, recent, or full")
	ErrInvalidRecentWindow  = errors.New("recent window weeks must be greater than zero")
	ErrInvalidCadence       = errors.New("scheduler cadence must be a valid duration")
	ErrInvalidAuthMethod    = errors.New("auth method must be oauth or apppassword")
	ErrAccountNotFound      = errors.New("account not found")
	ErrAmbiguousAccount     = errors.New("multiple accounts configured; specify --account or set default_account")
	ErrDuplicateAccount     = errors.New("duplicate account email")
)

// AccountConfig holds per-account settings.
type AccountConfig struct {
	Email      string     `toml:"email"`
	AuthMethod AuthMethod `toml:"auth_method"`
	SyncMode   SyncMode   `toml:"sync_mode,omitempty"`
}

type Config struct {
	ArchiveRoot       string          `toml:"archive_root"`
	DefaultAccount    string          `toml:"default_account,omitempty"`
	Accounts          []AccountConfig `toml:"accounts"`
	SyncMode          SyncMode        `toml:"sync_mode"`
	RecentWindowWeeks int             `toml:"recent_window_weeks"`
	SchedulerCadence  string          `toml:"scheduler_cadence"`

	// Deprecated: kept for backward-compatible loading only.
	// Save() does not write these fields.
	AccountEmail string     `toml:"account_email,omitempty"`
	AuthMethod   AuthMethod `toml:"auth_method,omitempty"`
}

func Default() (Config, error) {
	archiveRoot, err := DefaultArchiveRoot()
	if err != nil {
		return Config{}, err
	}

	return Config{
		ArchiveRoot:       archiveRoot,
		Accounts:          []AccountConfig{},
		SyncMode:          SyncModeRecent,
		RecentWindowWeeks: defaultRecentWindowWeeks,
		SchedulerCadence:  defaultSchedulerCadence,
	}, nil
}

func Load() (Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return Config{}, err
	}

	cfg.migrateDeprecated()
	cfg.applyDefaults()

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// configWritable is the serialization struct for Save(). It omits
// the deprecated flat AccountEmail/AuthMethod fields so they are
// never written to new config files.
type configWritable struct {
	ArchiveRoot       string          `toml:"archive_root"`
	DefaultAccount    string          `toml:"default_account,omitempty"`
	Accounts          []AccountConfig `toml:"accounts"`
	SyncMode          SyncMode        `toml:"sync_mode"`
	RecentWindowWeeks int             `toml:"recent_window_weeks"`
	SchedulerCadence  string          `toml:"scheduler_cadence"`
}

func Save(cfg Config) error {
	cfg.applyDefaults()

	if err := cfg.Validate(); err != nil {
		return err
	}

	path, err := ConfigPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	file, err := os.CreateTemp(filepath.Dir(path), "config-*.toml")
	if err != nil {
		return err
	}

	tempPath := file.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempPath)
		}
	}()

	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return err
	}

	writable := configWritable{
		ArchiveRoot:       cfg.ArchiveRoot,
		DefaultAccount:    cfg.DefaultAccount,
		Accounts:          cfg.Accounts,
		SyncMode:          cfg.SyncMode,
		RecentWindowWeeks: cfg.RecentWindowWeeks,
		SchedulerCadence:  cfg.SchedulerCadence,
	}

	encoder := toml.NewEncoder(file)
	if err := encoder.Encode(writable); err != nil {
		_ = file.Close()
		return err
	}

	if err := file.Close(); err != nil {
		return err
	}

	if err := os.Rename(tempPath, path); err != nil {
		return err
	}

	cleanup = false

	return nil
}

func (c Config) Validate() error {
	if !c.SyncMode.valid() {
		return fmt.Errorf("%w: %q", ErrInvalidSyncMode, c.SyncMode)
	}

	if c.RecentWindowWeeks <= 0 {
		return fmt.Errorf("%w: %d", ErrInvalidRecentWindow, c.RecentWindowWeeks)
	}

	if _, err := time.ParseDuration(c.SchedulerCadence); err != nil {
		return fmt.Errorf("%w: %q", ErrInvalidCadence, c.SchedulerCadence)
	}

	if strings.TrimSpace(c.ArchiveRoot) == "" {
		return errors.New("archive root is required")
	}

	// Validate each account.
	seen := make(map[string]bool, len(c.Accounts))
	for i, acct := range c.Accounts {
		if strings.TrimSpace(acct.Email) == "" {
			return fmt.Errorf("%w: account[%d]", ErrAccountEmailRequired, i)
		}
		lower := strings.ToLower(acct.Email)
		if seen[lower] {
			return fmt.Errorf("%w: %q", ErrDuplicateAccount, acct.Email)
		}
		seen[lower] = true
		if !acct.AuthMethod.valid() {
			return fmt.Errorf("%w: %q (account %q)", ErrInvalidAuthMethod, acct.AuthMethod, acct.Email)
		}
		if acct.SyncMode != "" && !acct.SyncMode.valid() {
			return fmt.Errorf("%w: %q (account %q)", ErrInvalidSyncMode, acct.SyncMode, acct.Email)
		}
	}

	return nil
}

func (c Config) SchedulerInterval() (time.Duration, error) {
	if err := c.Validate(); err != nil {
		return 0, err
	}

	return time.ParseDuration(c.SchedulerCadence)
}

func ConfigPath() (string, error) {
	configHome, err := xdgConfigHome()
	if err != nil {
		return "", err
	}

	return filepath.Join(configHome, configDirName, configFileName), nil
}

func DefaultArchiveRoot() (string, error) {
	dataHome, err := xdgDataHome()
	if err != nil {
		return "", err
	}

	return filepath.Join(dataHome, configDirName, "archive"), nil
}

func TokenPath(accountEmail string) (string, error) {
	accountEmail = strings.TrimSpace(strings.ToLower(accountEmail))
	if accountEmail == "" {
		return "", ErrAccountEmailRequired
	}

	configHome, err := xdgConfigHome()
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256([]byte(accountEmail))
	fileName := tokenFilePrefix + hex.EncodeToString(sum[:]) + tokenFileSuffix

	return filepath.Join(configHome, configDirName, fileName), nil
}

// ResolveAccount determines which account to use given an optional flag value.
// Priority: explicit flag > DefaultAccount > sole account.
func (c Config) ResolveAccount(flagValue string) (*AccountConfig, error) {
	if flagValue != "" {
		acct := c.AccountByEmail(flagValue)
		if acct == nil {
			emails := make([]string, len(c.Accounts))
			for i, a := range c.Accounts {
				emails[i] = a.Email
			}
			return nil, fmt.Errorf("%w: %q (available: %s)", ErrAccountNotFound, flagValue, strings.Join(emails, ", "))
		}
		return acct, nil
	}

	if c.DefaultAccount != "" {
		acct := c.AccountByEmail(c.DefaultAccount)
		if acct == nil {
			return nil, fmt.Errorf("%w: default %q", ErrAccountNotFound, c.DefaultAccount)
		}
		return acct, nil
	}

	switch len(c.Accounts) {
	case 0:
		return nil, fmt.Errorf("%w: no accounts configured", ErrAccountNotFound)
	case 1:
		return &c.Accounts[0], nil
	default:
		emails := make([]string, len(c.Accounts))
		for i, a := range c.Accounts {
			emails[i] = a.Email
		}
		return nil, fmt.Errorf("%w: %s", ErrAmbiguousAccount, strings.Join(emails, ", "))
	}
}

// AccountByEmail returns a pointer to the matching AccountConfig, or nil.
func (c Config) AccountByEmail(email string) *AccountConfig {
	email = strings.ToLower(strings.TrimSpace(email))
	for i := range c.Accounts {
		if strings.ToLower(strings.TrimSpace(c.Accounts[i].Email)) == email {
			return &c.Accounts[i]
		}
	}
	return nil
}

func (c *Config) applyDefaults() {
	if c.SyncMode == "" {
		c.SyncMode = SyncModeRecent
	}

	if c.RecentWindowWeeks == 0 {
		c.RecentWindowWeeks = defaultRecentWindowWeeks
	}

	if c.SchedulerCadence == "" {
		c.SchedulerCadence = defaultSchedulerCadence
	}

	if strings.TrimSpace(c.ArchiveRoot) == "" {
		if archiveRoot, err := DefaultArchiveRoot(); err == nil {
			c.ArchiveRoot = archiveRoot
		}
	}
}

// migrateDeprecated migrates the old flat account_email/auth_method fields
// into the Accounts slice and back-populates them for backward compat.
// Called only from Load() so that Save() does not re-add removed accounts.
func (c *Config) migrateDeprecated() {
	// Migrate deprecated flat fields into Accounts slice.
	if c.AccountEmail != "" && len(c.Accounts) == 0 {
		authMethod := c.AuthMethod
		if authMethod == "" {
			authMethod = AuthMethodOAuth
		}
		c.Accounts = append(c.Accounts, AccountConfig{
			Email:      c.AccountEmail,
			AuthMethod: authMethod,
		})
		c.DefaultAccount = c.AccountEmail
		c.AccountEmail = ""
		c.AuthMethod = ""
	}

	// Back-populate deprecated fields for backward compat with existing code.
	if c.AccountEmail == "" && len(c.Accounts) > 0 {
		c.AccountEmail = c.Accounts[0].Email
		c.AuthMethod = c.Accounts[0].AuthMethod
	}

	// Default auth method on the deprecated field when no accounts exist.
	if c.AuthMethod == "" {
		c.AuthMethod = AuthMethodOAuth
	}
}

func (m AuthMethod) valid() bool {
	switch m {
	case AuthMethodOAuth, AuthMethodAppPassword:
		return true
	default:
		return false
	}
}

func AppPasswordPath(accountEmail string) (string, error) {
	accountEmail = strings.TrimSpace(strings.ToLower(accountEmail))
	if accountEmail == "" {
		return "", ErrAccountEmailRequired
	}

	configHome, err := xdgConfigHome()
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256([]byte(accountEmail))
	fileName := "apppassword_" + hex.EncodeToString(sum[:]) + ".txt"

	return filepath.Join(configHome, configDirName, fileName), nil
}

func (m SyncMode) valid() bool {
	switch m {
	case SyncModeInbox, SyncModeRecent, SyncModeFull:
		return true
	default:
		return false
	}
}

func xdgConfigHome() (string, error) {
	if value := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); value != "" {
		return value, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".config"), nil
}

func xdgDataHome() (string, error) {
	if value := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); value != "" {
		return value, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".local", "share"), nil
}
