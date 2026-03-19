package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jamierumbelow/letterhead/pkg/types"
)

type Mode string

const (
	ModeHuman Mode = "human"
	ModeJSON  Mode = "json"
	ModeJSONL Mode = "jsonl"
)

var (
	ErrConflictingModes = errors.New("--json and --jsonl cannot be used together")
	ErrInvalidMode      = errors.New("invalid output mode")
)

type Formatter interface {
	WriteStatus(io.Writer, types.StatusOutput) error
	WriteFind(io.Writer, types.FindOutput) error
	WriteRead(io.Writer, types.ReadOutput) error
}

func ModeFromFlags(asJSON, asJSONL bool) (Mode, error) {
	if asJSON && asJSONL {
		return "", ErrConflictingModes
	}

	switch {
	case asJSON:
		return ModeJSON, nil
	case asJSONL:
		return ModeJSONL, nil
	default:
		return ModeHuman, nil
	}
}

func NewFormatter(mode Mode) (Formatter, error) {
	switch mode {
	case ModeHuman:
		return humanFormatter{}, nil
	case ModeJSON:
		return jsonFormatter{}, nil
	case ModeJSONL:
		return jsonlFormatter{}, nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrInvalidMode, mode)
	}
}

type humanFormatter struct{}

func (humanFormatter) WriteStatus(w io.Writer, output types.StatusOutput) error {
	lastSyncAt := "never"
	if output.LastSyncAt != nil {
		lastSyncAt = output.LastSyncAt.Format(time.RFC3339)
	}

	_, err := fmt.Fprintf(
		w,
		"Account: %s\nArchive Path: %s\nSync Mode: %s\nMessage Count: %d\nThread Count: %d\nBootstrap Complete: %t\nBootstrap Progress: %.1f%%\nLast Sync At: %s\nScheduler State: %s\nDB Health: %s\n",
		output.Account,
		output.ArchivePath,
		output.SyncMode,
		output.MessageCount,
		output.ThreadCount,
		output.BootstrapComplete,
		output.BootstrapProgress,
		lastSyncAt,
		output.SchedulerState,
		output.DBHealth,
	)

	return err
}

func (humanFormatter) WriteFind(w io.Writer, output types.FindOutput) error {
	if len(output.Results) == 0 {
		_, err := fmt.Fprintln(w, "No results.")
		return err
	}

	// Header
	fmt.Fprintf(w, "%d results (%dms)\n\n", output.TotalCount, output.QueryMS)

	for _, result := range output.Results {
		subject := result.Subject
		if len(subject) > 60 {
			subject = subject[:57] + "..."
		}

		date := relativeDate(result.LatestAt)

		line := fmt.Sprintf(
			"%-8s  %-60s  (%d)  %s",
			date,
			subject,
			result.MessageCount,
			strings.Join(result.Participants, ", "),
		)

		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}

	// Pagination footer
	if output.Offset > 0 || output.TotalCount > output.Limit {
		start := output.Offset + 1
		end := output.Offset + len(output.Results)
		fmt.Fprintf(w, "\nShowing %d-%d of %d\n", start, end, output.TotalCount)
	}

	return nil
}

func relativeDate(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "now"
	case diff < time.Hour:
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	case diff < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(diff.Hours()/24))
	case diff < 30*24*time.Hour:
		return fmt.Sprintf("%dw ago", int(diff.Hours()/(24*7)))
	default:
		return t.Format("Jan 02")
	}
}

func (humanFormatter) WriteRead(w io.Writer, output types.ReadOutput) error {
	if _, err := fmt.Fprintf(
		w,
		"Subject: %s\nFrom: %s <%s>\nDate: %s\nParticipants: %s\n",
		output.Subject,
		output.From.Name,
		output.From.Email,
		output.Date.Format(time.RFC3339),
		strings.Join(output.Participants, ", "),
	); err != nil {
		return err
	}

	if output.Body != "" {
		if _, err := fmt.Fprintf(w, "\n%s\n", output.Body); err != nil {
			return err
		}
	}

	if len(output.Messages) == 0 {
		return nil
	}

	if _, err := fmt.Fprintln(w, "\nThread Messages:"); err != nil {
		return err
	}

	for _, message := range output.Messages {
		if _, err := fmt.Fprintf(
			w,
			"- %s  %s  %s\n",
			message.Date.Format("2006-01-02 15:04"),
			message.Subject,
			message.From.Email,
		); err != nil {
			return err
		}
	}

	return nil
}

type jsonFormatter struct{}

func (jsonFormatter) WriteStatus(w io.Writer, output types.StatusOutput) error {
	return writeJSON(w, output)
}

func (jsonFormatter) WriteFind(w io.Writer, output types.FindOutput) error {
	return writeJSON(w, output)
}

func (jsonFormatter) WriteRead(w io.Writer, output types.ReadOutput) error {
	return writeJSON(w, output)
}

type jsonlFormatter struct{}

func (jsonlFormatter) WriteStatus(w io.Writer, output types.StatusOutput) error {
	return writeJSON(w, output)
}

func (jsonlFormatter) WriteFind(w io.Writer, output types.FindOutput) error {
	encoder := json.NewEncoder(w)
	for _, result := range output.Results {
		if err := encoder.Encode(result); err != nil {
			return err
		}
	}

	return nil
}

func (jsonlFormatter) WriteRead(w io.Writer, output types.ReadOutput) error {
	return writeJSON(w, output)
}

func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)

	return encoder.Encode(value)
}
