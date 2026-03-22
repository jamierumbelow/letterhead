-- Step 1: Create new messages table with account_id
CREATE TABLE messages_new (
    account_id TEXT NOT NULL DEFAULT '',
    gmail_id TEXT NOT NULL,
    thread_id TEXT NOT NULL,
    history_id INTEGER NOT NULL,
    internal_date INTEGER NOT NULL,
    received_at INTEGER NOT NULL,
    subject TEXT NOT NULL DEFAULT '',
    snippet TEXT NOT NULL DEFAULT '',
    from_addr TEXT NOT NULL DEFAULT '',
    from_name TEXT NOT NULL DEFAULT '',
    plain_body TEXT NOT NULL DEFAULT '',
    html_body TEXT NOT NULL DEFAULT '',
    attachment_metadata_json TEXT NOT NULL DEFAULT '[]',
    raw_size_bytes INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (account_id, gmail_id)
);

-- Step 2: Backfill from existing data
INSERT INTO messages_new
SELECT COALESCE((SELECT account_id FROM sync_state LIMIT 1), ''),
       gmail_id, thread_id, history_id, internal_date, received_at,
       subject, snippet, from_addr, from_name,
       plain_body, html_body, attachment_metadata_json,
       raw_size_bytes, created_at, updated_at
FROM messages;

-- Step 3: Drop old table and rename
DROP TABLE messages;
ALTER TABLE messages_new RENAME TO messages;

-- Step 4: Add index for per-account date queries
CREATE INDEX idx_messages_account_date ON messages(account_id, internal_date);

-- Step 5: Recreate message_labels with account_id
CREATE TABLE message_labels_new (
    account_id TEXT NOT NULL DEFAULT '',
    gmail_id TEXT NOT NULL,
    label TEXT NOT NULL,
    PRIMARY KEY (account_id, gmail_id, label),
    FOREIGN KEY (account_id, gmail_id) REFERENCES messages(account_id, gmail_id) ON DELETE CASCADE
);

INSERT INTO message_labels_new
SELECT COALESCE((SELECT account_id FROM sync_state LIMIT 1), ''),
       gmail_id, label
FROM message_labels;

DROP TABLE message_labels;
ALTER TABLE message_labels_new RENAME TO message_labels;

-- Step 6: Recreate message_recipients with account_id
CREATE TABLE message_recipients_new (
    account_id TEXT NOT NULL DEFAULT '',
    gmail_id TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('to', 'cc', 'bcc')),
    addr TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (account_id, gmail_id, role, addr),
    FOREIGN KEY (account_id, gmail_id) REFERENCES messages(account_id, gmail_id) ON DELETE CASCADE
);

INSERT INTO message_recipients_new
SELECT COALESCE((SELECT account_id FROM sync_state LIMIT 1), ''),
       gmail_id, role, addr, name
FROM message_recipients;

DROP TABLE message_recipients;
ALTER TABLE message_recipients_new RENAME TO message_recipients;

-- Step 7: Drop and recreate FTS triggers
DROP TRIGGER IF EXISTS messages_ai;
DROP TRIGGER IF EXISTS messages_ad;
DROP TRIGGER IF EXISTS messages_au;

CREATE TRIGGER messages_ai AFTER INSERT ON messages BEGIN
    INSERT INTO messages_fts(rowid, subject, plain_body, snippet, from_addr, from_name)
    VALUES (new.rowid, new.subject, new.plain_body, new.snippet, new.from_addr, new.from_name);
END;

CREATE TRIGGER messages_ad AFTER DELETE ON messages BEGIN
    INSERT INTO messages_fts(messages_fts, rowid, subject, plain_body, snippet, from_addr, from_name)
    VALUES ('delete', old.rowid, old.subject, old.plain_body, old.snippet, old.from_addr, old.from_name);
END;

CREATE TRIGGER messages_au AFTER UPDATE ON messages BEGIN
    INSERT INTO messages_fts(messages_fts, rowid, subject, plain_body, snippet, from_addr, from_name)
    VALUES ('delete', old.rowid, old.subject, old.plain_body, old.snippet, old.from_addr, old.from_name);
    INSERT INTO messages_fts(rowid, subject, plain_body, snippet, from_addr, from_name)
    VALUES (new.rowid, new.subject, new.plain_body, new.snippet, new.from_addr, new.from_name);
END;

-- Step 8: Rebuild FTS index
INSERT INTO messages_fts(messages_fts) VALUES('rebuild');
