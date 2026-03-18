CREATE TABLE messages (
    gmail_id TEXT PRIMARY KEY,
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
    updated_at INTEGER NOT NULL
);

CREATE TABLE message_labels (
    gmail_id TEXT NOT NULL,
    label TEXT NOT NULL,
    PRIMARY KEY (gmail_id, label),
    FOREIGN KEY (gmail_id) REFERENCES messages(gmail_id) ON DELETE CASCADE
);

CREATE TABLE message_recipients (
    gmail_id TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('to', 'cc', 'bcc')),
    addr TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (gmail_id, role, addr),
    FOREIGN KEY (gmail_id) REFERENCES messages(gmail_id) ON DELETE CASCADE
);

CREATE TABLE sync_state (
    account_id TEXT PRIMARY KEY,
    history_id INTEGER NOT NULL,
    bootstrap_complete INTEGER NOT NULL DEFAULT 0,
    messages_synced INTEGER NOT NULL DEFAULT 0,
    last_sync_at INTEGER
);

CREATE TABLE sync_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id TEXT NOT NULL,
    started_at INTEGER NOT NULL,
    finished_at INTEGER,
    mode TEXT NOT NULL,
    messages_synced INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL,
    error_msg TEXT NOT NULL DEFAULT ''
);
