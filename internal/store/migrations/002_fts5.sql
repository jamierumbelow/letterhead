CREATE VIRTUAL TABLE messages_fts USING fts5(
    subject,
    plain_body,
    snippet,
    from_addr,
    from_name,
    content = 'messages',
    content_rowid = 'rowid',
    tokenize = 'unicode61'
);

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
