package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jamierumbelow/letterhead/internal/config"
)

func TestStatusCommandUsesDefaultPathsWhenNotInitialized(t *testing.T) {
	configHome := t.TempDir()
	dataHome := t.TempDir()

	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_DATA_HOME", dataHome)

	cmd := NewRootCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"status", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	defaultCfg, err := config.Default()
	if err != nil {
		t.Fatalf("config.Default() error = %v", err)
	}

	payload := stdout.String()
	expectedFields := []string{
		`"account":"not authenticated"`,
		`"archive_path":"` + defaultCfg.ArchiveRoot + `"`,
		`"sync_mode":"recent"`,
		`"db_health":"not initialized"`,
	}

	for _, field := range expectedFields {
		if !strings.Contains(payload, field) {
			t.Fatalf("status payload %q does not contain %q", payload, field)
		}
	}
}

func TestStatusCommandReturnsInitializedSkeleton(t *testing.T) {
	configHome := t.TempDir()
	dataHome := t.TempDir()
	archiveRoot := filepath.Join(t.TempDir(), "archive")

	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_DATA_HOME", dataHome)

	cmd := NewRootCommand()
	cmd.SetArgs([]string{"init", "--archive-root", archiveRoot})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init Execute() error = %v", err)
	}

	cmd = NewRootCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"status", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("status Execute() error = %v", err)
	}

	payload := stdout.String()
	expectedFields := []string{
		`"archive_path":"` + archiveRoot + `"`,
		`"message_count":0`,
		`"thread_count":0`,
		`"db_health":"ok"`,
	}

	for _, field := range expectedFields {
		if !strings.Contains(payload, field) {
			t.Fatalf("status payload %q does not contain %q", payload, field)
		}
	}
}
