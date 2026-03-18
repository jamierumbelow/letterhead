package config

import (
	"errors"
	"os"
	"path/filepath"
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
		ArchiveRoot:       filepath.Join(dataHome, "custom-archive"),
		AccountEmail:      "user@example.com",
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

	if loaded != cfg {
		t.Fatalf("Load() = %#v, want %#v", loaded, cfg)
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
