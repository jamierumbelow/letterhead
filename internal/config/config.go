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
)

type Config struct {
	ArchiveRoot       string     `toml:"archive_root"`
	AccountEmail      string     `toml:"account_email"`
	AuthMethod        AuthMethod `toml:"auth_method"`
	SyncMode          SyncMode   `toml:"sync_mode"`
	RecentWindowWeeks int        `toml:"recent_window_weeks"`
	SchedulerCadence  string     `toml:"scheduler_cadence"`
}

func Default() (Config, error) {
	archiveRoot, err := DefaultArchiveRoot()
	if err != nil {
		return Config{}, err
	}

	return Config{
		ArchiveRoot:       archiveRoot,
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

	cfg.applyDefaults()

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
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

	encoder := toml.NewEncoder(file)
	if err := encoder.Encode(cfg); err != nil {
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
	if !c.AuthMethod.valid() {
		return fmt.Errorf("%w: %q", ErrInvalidAuthMethod, c.AuthMethod)
	}

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

func (c *Config) applyDefaults() {
	if c.AuthMethod == "" {
		c.AuthMethod = AuthMethodOAuth
	}

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
