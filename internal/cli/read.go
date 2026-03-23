package cli

import (
	"context"
	"database/sql"
	"strings"

	"github.com/jamierumbelow/letterhead/internal/diagnostics"
	"github.com/jamierumbelow/letterhead/internal/output"
	"github.com/jamierumbelow/letterhead/internal/store"
	"github.com/jamierumbelow/letterhead/pkg/types"
	"github.com/spf13/cobra"
)

func newReadCommand() *cobra.Command {
	var (
		view     string
		asThread bool
	)

	cmd := &cobra.Command{
		Use:   "read <message-id or thread-id>",
		Short: "Read a message or thread",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			handle := args[0]

			cfg, err := ensureInitialized()
			if err != nil {
				return err
			}

			accountFlag, _ := cmd.Flags().GetString("account")

			db, err := store.Open(store.DatabasePath(cfg.ArchiveRoot))
			if err != nil {
				return err
			}
			defer db.Close()

			s := store.New(db)
			ctx := context.Background()

			readView := types.ReadView(view)

			_, formatter, err := formatterFromCommand(cmd)
			if err != nil {
				return err
			}

			// Audit log
			audit := diagnostics.NewAuditLog(cfg.ArchiveRoot)
			audit.Log(diagnostics.AuditEntry{
				Command:     "read",
				ID:          handle,
				ResultCount: 1,
			})

			if asThread {
				return readThread(ctx, cmd, s, formatter, accountFlag, handle, readView)
			}

			return readSingle(ctx, cmd, s, formatter, accountFlag, handle, readView)
		},
	}

	cmd.Flags().StringVar(&view, "view", "summary", "view level: summary, text, or full")
	cmd.Flags().BoolVar(&asThread, "thread", false, "read the whole thread")

	return cmd
}

func readSingle(ctx context.Context, cmd *cobra.Command, s *store.Store, formatter output.Formatter, accountID string, handle string, view types.ReadView) error {
	// Try as message ID first
	msg, err := s.GetMessageForAccount(ctx, accountID, handle)
	if err == sql.ErrNoRows {
		// Try as thread ID — get the latest message
		msgs, threadErr := s.GetMessagesInThreadForAccount(ctx, accountID, handle)
		if threadErr != nil || len(msgs) == 0 {
			return NewExitErrorWithHint(ExitNotFound, "letterhead find <query>", "message or thread %q not found", handle)
		}
		latest := msgs[len(msgs)-1]
		msg = &latest
	} else if err != nil {
		return err
	}

	output := buildReadOutput(msg, view)
	return formatter.WriteRead(cmd.OutOrStdout(), output)
}

func readThread(ctx context.Context, cmd *cobra.Command, s *store.Store, formatter output.Formatter, accountID string, handle string, view types.ReadView) error {
	// Resolve thread ID
	threadID := handle

	// Check if handle is a message ID; if so, get its thread
	msg, err := s.GetMessageForAccount(ctx, accountID, handle)
	if err == nil {
		threadID = msg.ThreadID
	}

	msgs, err := s.GetMessagesInThreadForAccount(ctx, accountID, threadID)
	if err != nil {
		return err
	}
	if len(msgs) == 0 {
		return NewExitErrorWithHint(ExitNotFound, "letterhead find <query>", "thread %q not found", handle)
	}

	latest := msgs[len(msgs)-1]

	output := types.ReadOutput{
		View:         view,
		MessageID:    latest.GmailID,
		ThreadID:     threadID,
		Subject:      latest.Subject,
		From:         latest.From,
		Date:         latest.ReceivedAt,
		Participants: threadParticipantNames(msgs),
	}

	switch view {
	case types.ReadViewText:
		var bodies []string
		for _, m := range msgs {
			bodies = append(bodies, m.PlainBody)
		}
		output.Body = strings.Join(bodies, "\n\n---\n\n")
	case types.ReadViewFull:
		output.Body = latest.PlainBody
	}

	// Add message summaries
	for _, m := range msgs {
		output.Messages = append(output.Messages, types.MessageSummary{
			MessageID:       m.GmailID,
			ThreadID:        m.ThreadID,
			Subject:         m.Subject,
			From:            m.From,
			Date:            m.ReceivedAt,
			Participants:    messageParticipantNames(&m),
			Snippet:         m.Snippet,
			LabelNames:      m.Labels,
			AttachmentCount: len(m.Attachments),
		})
	}

	return formatter.WriteRead(cmd.OutOrStdout(), output)
}

func buildReadOutput(msg *types.Message, view types.ReadView) types.ReadOutput {
	output := types.ReadOutput{
		View:         view,
		MessageID:    msg.GmailID,
		ThreadID:     msg.ThreadID,
		Subject:      msg.Subject,
		From:         msg.From,
		Date:         msg.ReceivedAt,
		Participants: messageParticipantNames(msg),
	}

	switch view {
	case types.ReadViewText:
		output.Body = msg.PlainBody
	case types.ReadViewFull:
		output.Body = msg.PlainBody
	}

	return output
}

func messageParticipantNames(msg *types.Message) []string {
	seen := make(map[string]bool)
	var names []string

	add := func(a types.Address) {
		display := a.Name
		if display == "" {
			display = a.Email
		}
		if !seen[display] {
			seen[display] = true
			names = append(names, display)
		}
	}

	add(msg.From)
	for _, a := range msg.To {
		add(a)
	}
	for _, a := range msg.CC {
		add(a)
	}

	return names
}

func threadParticipantNames(msgs []types.Message) []string {
	seen := make(map[string]bool)
	var names []string

	for _, msg := range msgs {
		for _, name := range messageParticipantNames(&msg) {
			if !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}

	return names
}
