package scheduler

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

const unitName = "letterhead-sync"

func systemdUserDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "systemd", "user")
}

func servicePath() string {
	return filepath.Join(systemdUserDir(), unitName+".service")
}

func timerPath() string {
	return filepath.Join(systemdUserDir(), unitName+".timer")
}

var serviceTmpl = template.Must(template.New("service").Parse(`[Unit]
Description=Letterhead Gmail sync

[Service]
Type=oneshot
ExecStart={{ .Bin }} sync

[Install]
WantedBy=default.target
`))

var timerTmpl = template.Must(template.New("timer").Parse(`[Unit]
Description=Letterhead periodic sync timer

[Timer]
OnBootSec=5min
OnUnitActiveSec={{ .Interval }}sec

[Install]
WantedBy=timers.target
`))

type systemdScheduler struct{}

func (s *systemdScheduler) Install(cfg Config) error {
	dir := systemdUserDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create systemd user dir: %w", err)
	}

	// Write service unit
	sf, err := os.Create(servicePath())
	if err != nil {
		return fmt.Errorf("create service unit: %w", err)
	}
	defer sf.Close()

	if err := serviceTmpl.Execute(sf, struct{ Bin string }{Bin: cfg.LetterheadBin}); err != nil {
		return fmt.Errorf("write service unit: %w", err)
	}

	// Write timer unit
	tf, err := os.Create(timerPath())
	if err != nil {
		return fmt.Errorf("create timer unit: %w", err)
	}
	defer tf.Close()

	if err := timerTmpl.Execute(tf, struct{ Interval int }{Interval: cfg.IntervalSecs}); err != nil {
		return fmt.Errorf("write timer unit: %w", err)
	}

	// Reload and enable
	if err := exec.Command("systemctl", "--user", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("daemon-reload: %w", err)
	}

	if err := exec.Command("systemctl", "--user", "enable", "--now", unitName+".timer").Run(); err != nil {
		return fmt.Errorf("enable timer: %w", err)
	}

	return nil
}

func (s *systemdScheduler) Uninstall() error {
	_ = exec.Command("systemctl", "--user", "disable", "--now", unitName+".timer").Run()

	for _, path := range []string{servicePath(), timerPath()} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", filepath.Base(path), err)
		}
	}

	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()

	return nil
}

func (s *systemdScheduler) IsInstalled() bool {
	_, err := os.Stat(timerPath())
	return err == nil
}

func (s *systemdScheduler) Status() string {
	if !s.IsInstalled() {
		return "not installed"
	}
	return "installed (systemd)"
}
