package types

import "time"

// Address is a normalized mail address with an optional display name.
type Address struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// AttachmentMeta describes an attachment without downloading its body.
type AttachmentMeta struct {
	Filename  string `json:"filename"`
	MIMEType  string `json:"mime_type"`
	SizeBytes int64  `json:"size_bytes"`
	PartID    string `json:"part_id"`
}

// Message is the canonical normalized representation of a Gmail message.
type Message struct {
	GmailID      string           `json:"gmail_id"`
	ThreadID     string           `json:"thread_id"`
	HistoryID    uint64           `json:"history_id"`
	InternalDate int64            `json:"internal_date"`
	ReceivedAt   time.Time        `json:"received_at"`
	Subject      string           `json:"subject"`
	Snippet      string           `json:"snippet"`
	From         Address          `json:"from"`
	To           []Address        `json:"to,omitempty"`
	CC           []Address        `json:"cc,omitempty"`
	BCC          []Address        `json:"bcc,omitempty"`
	Labels       []string         `json:"labels,omitempty"`
	PlainBody    string           `json:"plain_body"`
	HTMLBody     string           `json:"html_body,omitempty"`
	Attachments  []AttachmentMeta `json:"attachments,omitempty"`
}

// ThreadSummary is the summary-first result shape returned by search.
type ThreadSummary struct {
	ThreadID     string    `json:"thread_id"`
	Subject      string    `json:"subject"`
	Participants []string  `json:"participants,omitempty"`
	LatestAt     time.Time `json:"latest_at"`
	MessageCount int       `json:"message_count"`
	Snippet      string    `json:"snippet"`
	LabelNames   []string  `json:"label_names,omitempty"`
	MessageIDs   []string  `json:"message_ids,omitempty"`
}
