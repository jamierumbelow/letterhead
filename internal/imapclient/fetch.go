package imapclient

import (
	"context"
	"fmt"
	"io"

	"github.com/emersion/go-imap/v2"
	goimapclient "github.com/emersion/go-imap/v2/imapclient"
	"github.com/jamierumbelow/letterhead/pkg/types"
)

// ListUIDs selects the given folder and returns the UIDs in it.
// If sinceUID > 0, only UIDs greater than sinceUID are returned.
// Returns the UIDs and the UIDVALIDITY of the mailbox.
func ListUIDs(_ context.Context, conn *goimapclient.Client, folder string, sinceUID uint32) (uids []imap.UID, uidValidity uint32, err error) {
	// SELECT the folder
	selectData, err := conn.Select(folder, nil).Wait()
	if err != nil {
		return nil, 0, fmt.Errorf("select %q: %w", folder, err)
	}
	uidValidity = selectData.UIDValidity

	// Build search criteria
	criteria := &imap.SearchCriteria{}
	if sinceUID > 0 {
		uidSet := imap.UIDSet{}
		uidSet.AddRange(imap.UID(sinceUID+1), 0) // 0 means * (max)
		criteria.UID = []imap.UIDSet{uidSet}
	}

	searchData, err := conn.Search(criteria, nil).Wait()
	if err != nil {
		return nil, 0, fmt.Errorf("search: %w", err)
	}

	uids = searchData.AllUIDs()
	return uids, uidValidity, nil
}

// FetchMessages fetches the raw RFC822 messages for the given UIDs and parses them.
func FetchMessages(_ context.Context, conn *goimapclient.Client, folder string, uids []imap.UID) ([]*types.Message, error) {
	if len(uids) == 0 {
		return nil, nil
	}

	// Ensure folder is selected
	if _, err := conn.Select(folder, nil).Wait(); err != nil {
		return nil, fmt.Errorf("select %q: %w", folder, err)
	}

	uidSet := imap.UIDSetNum(uids...)

	fetchOptions := &imap.FetchOptions{
		UID: true,
		BodySection: []*imap.FetchItemBodySection{
			{}, // empty = entire message (BODY[])
		},
	}

	fetchCmd := conn.Fetch(uidSet, fetchOptions)

	var messages []*types.Message
	for {
		msgData := fetchCmd.Next()
		if msgData == nil {
			break
		}

		// Collect all item data for this message
		var raw []byte
		for {
			item := msgData.Next()
			if item == nil {
				break
			}
			if bodySection, ok := item.(goimapclient.FetchItemDataBodySection); ok {
				raw, _ = readAll(bodySection)
			}
		}

		if raw == nil {
			continue
		}

		msg, err := ParseRFC822Message(raw)
		if err != nil {
			continue // skip unparseable messages
		}
		messages = append(messages, msg)
	}

	if err := fetchCmd.Close(); err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}

	return messages, nil
}

func readAll(section goimapclient.FetchItemDataBodySection) ([]byte, error) {
	if section.Literal == nil {
		return nil, fmt.Errorf("nil literal")
	}
	return io.ReadAll(section.Literal)
}
