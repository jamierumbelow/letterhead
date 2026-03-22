package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jamierumbelow/letterhead/internal/diagnostics"
	"github.com/jamierumbelow/letterhead/internal/query"
	"github.com/jamierumbelow/letterhead/internal/store"
	"github.com/jamierumbelow/letterhead/pkg/types"
	"github.com/spf13/cobra"
)

func newFindCommand() *cobra.Command {
	var flags struct {
		from          []string
		to            []string
		subject       string
		labels        []string
		after         string
		before        string
		hasAttachment bool
		hasAttachSet  bool
		limit         int
		offset        int
	}

	cmd := &cobra.Command{
		Use:   "find [search terms...]",
		Short: "Search the local archive",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := ensureInitialized()
			if err != nil {
				return err
			}

			// Get the --account flag value for scoping search.
			// If empty, AccountID is left blank (cross-account search).
			accountFlag, _ := cmd.Flags().GetString("account")

			var hasAttach *bool
			if cmd.Flags().Changed("has-attachment") {
				hasAttach = &flags.hasAttachment
			}

			qf := query.QueryFlags{
				From:          flags.from,
				To:            flags.to,
				Subject:       flags.subject,
				Labels:        flags.labels,
				After:         flags.after,
				Before:        flags.before,
				HasAttachment: hasAttach,
				AccountID:     accountFlag,
				Limit:         flags.limit,
				Offset:        flags.offset,
			}

			q, err := query.Parse(args, qf)
			if err != nil {
				return err
			}

			db, err := store.Open(store.DatabasePath(cfg.ArchiveRoot))
			if err != nil {
				return err
			}
			defer db.Close()

			s := store.New(db)
			ctx := context.Background()

			start := time.Now()
			threads, err := s.SearchThreads(ctx, q)
			if err != nil {
				return err
			}
			elapsed := time.Since(start)

			// Build FindOutput
			results := make([]types.FindResult, len(threads))
			for i, t := range threads {
				participants := t.Participants
				if participants == nil {
					participants = []string{}
				}
				results[i] = types.FindResult{
					ResultID:     fmt.Sprintf("res_%d", i+1),
					ThreadID:     t.ThreadID,
					Subject:      t.Subject,
					Participants: participants,
					LatestAt:     t.LatestAt,
					MessageCount: t.MessageCount,
					Snippet:      t.Snippet,
					ReadHandle:   t.ThreadID,
				}
			}

			output := types.FindOutput{
				Results:    results,
				TotalCount: len(results),
				Limit:      flags.limit,
				Offset:     flags.offset,
				QueryMS:    elapsed.Milliseconds(),
			}

			_, formatter, err := formatterFromCommand(cmd)
			if err != nil {
				return err
			}

			// Audit log
			audit := diagnostics.NewAuditLog(cfg.ArchiveRoot)
			audit.Log(diagnostics.AuditEntry{
				Command:     "find",
				Query:       strings.Join(args, " "),
				ResultCount: len(results),
			})

			return formatter.WriteFind(cmd.OutOrStdout(), output)
		},
	}

	f := cmd.Flags()
	f.StringSliceVar(&flags.from, "from", nil, "filter by sender")
	f.StringSliceVar(&flags.to, "to", nil, "filter by recipient")
	f.StringVar(&flags.subject, "subject", "", "filter by subject")
	f.StringSliceVar(&flags.labels, "label", nil, "filter by label")
	f.StringVar(&flags.after, "after", "", "messages after date (YYYY-MM-DD)")
	f.StringVar(&flags.before, "before", "", "messages before date (YYYY-MM-DD)")
	f.BoolVar(&flags.hasAttachment, "has-attachment", false, "filter by attachment presence")
	f.IntVar(&flags.limit, "limit", 20, "max results")
	f.IntVar(&flags.offset, "offset", 0, "result offset")

	return cmd
}
