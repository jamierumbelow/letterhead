package scheduler

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

const (
	launchdLabel = "com.letterhead.sync"
)

func plistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist")
}

var plistTmpl = template.Must(template.New("plist").Parse(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>{{ .Label }}</string>
    <key>ProgramArguments</key>
    <array>
        <string>{{ .Bin }}</string>
        <string>sync</string>
        <string>--all</string>
    </array>
    <key>StartInterval</key>
    <integer>{{ .Interval }}</integer>
    <key>StandardOutPath</key>
    <string>{{ .LogDir }}/sync.stdout.log</string>
    <key>StandardErrorPath</key>
    <string>{{ .LogDir }}/sync.stderr.log</string>
    <key>RunAtLoad</key>
    <false/>
</dict>
</plist>
`))

type launchdScheduler struct{}

func (l *launchdScheduler) Install(cfg Config) error {
	// Ensure log directory exists
	if err := os.MkdirAll(cfg.LogDir, 0700); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	// Ensure LaunchAgents directory exists
	dir := filepath.Dir(plistPath())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create LaunchAgents dir: %w", err)
	}

	f, err := os.Create(plistPath())
	if err != nil {
		return fmt.Errorf("create plist: %w", err)
	}
	defer f.Close()

	data := struct {
		Label    string
		Bin      string
		Interval int
		LogDir   string
	}{
		Label:    launchdLabel,
		Bin:      cfg.LetterheadBin,
		Interval: cfg.IntervalSecs,
		LogDir:   cfg.LogDir,
	}

	if err := plistTmpl.Execute(f, data); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}

	if err := exec.Command("launchctl", "load", plistPath()).Run(); err != nil {
		return fmt.Errorf("launchctl load: %w", err)
	}

	return nil
}

func (l *launchdScheduler) Uninstall() error {
	path := plistPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	_ = exec.Command("launchctl", "unload", path).Run()

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove plist: %w", err)
	}
	return nil
}

func (l *launchdScheduler) IsInstalled() bool {
	_, err := os.Stat(plistPath())
	return err == nil
}

func (l *launchdScheduler) Status() string {
	if !l.IsInstalled() {
		return "not installed"
	}
	return "installed (launchd)"
}
