package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jamierumbelow/letterhead/internal/config"
	"github.com/jamierumbelow/letterhead/internal/store"
)

func TestInitCommandCreatesConfigAndDatabase(t *testing.T) {
	configHome := t.TempDir()
	dataHome := t.TempDir()
	archiveRoot := filepath.Join(t.TempDir(), "archive")

	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_DATA_HOME", dataHome)

	cmd := NewRootCommand()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"init", "--archive-root", archiveRoot})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	if cfg.ArchiveRoot != archiveRoot {
		t.Fatalf("ArchiveRoot = %q, want %q", cfg.ArchiveRoot, archiveRoot)
	}

	if _, err := os.Stat(store.DatabasePath(archiveRoot)); err != nil {
		t.Fatalf("database not created: %v", err)
	}

	if !strings.Contains(stdout.String(), "Letterhead initialized.") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestInitCommandIsIdempotent(t *testing.T) {
	configHome := t.TempDir()
	dataHome := t.TempDir()
	archiveRoot := filepath.Join(t.TempDir(), "archive")

	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_DATA_HOME", dataHome)

	cmd := NewRootCommand()
	cmd.SetArgs([]string{"init", "--archive-root", archiveRoot})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("first Execute() error = %v", err)
	}

	cmd = NewRootCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"init", "--archive-root", archiveRoot})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("second Execute() error = %v", err)
	}

	if !strings.Contains(stdout.String(), "already initialized") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestInitCommandPromptsForArchiveRoot(t *testing.T) {
	configHome := t.TempDir()
	dataHome := t.TempDir()

	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_DATA_HOME", dataHome)

	cmd := NewRootCommand()
	cmd.SetIn(strings.NewReader("\n"))
	cmd.SetArgs([]string{"init", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	defaultCfg, err := config.Default()
	if err != nil {
		t.Fatalf("config.Default() error = %v", err)
	}

	if cfg.ArchiveRoot != defaultCfg.ArchiveRoot {
		t.Fatalf("ArchiveRoot = %q, want %q", cfg.ArchiveRoot, defaultCfg.ArchiveRoot)
	}
}
