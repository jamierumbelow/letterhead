#!/usr/bin/env bash
set -euo pipefail

# E2E test for letterhead with pre-populated database.
#
# Since we cannot do real Gmail/IMAP sync in tests, we write a config file,
# let the CLI create a migrated database, then insert test messages with
# sqlite3 and exercise find/read/status.

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

PASS=0
FAIL=0
TMPDIR=$(mktemp -d)
export LETTERHEAD_ARCHIVE_ROOT="$TMPDIR/archive"
export XDG_CONFIG_HOME="$TMPDIR/config"
export XDG_DATA_HOME="$TMPDIR/data"

# Resolve the project root (where go.mod lives)
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
LETTERHEAD="go run -C $PROJECT_ROOT ./cmd/letterhead"

cleanup() {
    rm -rf "$TMPDIR"
}
trap cleanup EXIT

log_pass() {
    echo -e "${GREEN}[PASS]${NC} $1"
    PASS=$((PASS + 1))
}

log_fail() {
    echo -e "${RED}[FAIL]${NC} $1"
    echo "  Output: $2"
    FAIL=$((FAIL + 1))
}

assert_contains() {
    local output="$1"
    local expected="$2"
    local desc="$3"
    if echo "$output" | grep -q "$expected"; then
        log_pass "$desc"
    else
        log_fail "$desc" "$output"
    fi
}

assert_not_contains() {
    local output="$1"
    local unexpected="$2"
    local desc="$3"
    if echo "$output" | grep -q "$unexpected"; then
        log_fail "$desc" "$output"
    else
        log_pass "$desc"
    fi
}

assert_exit_0() {
    local desc="$1"
    shift
    local output
    if output=$("$@" 2>&1); then
        log_pass "$desc"
        echo "$output"
    else
        log_fail "$desc" "$output"
        echo "$output"
    fi
}

# ---------------------------------------------------------------------------
# 1. Create config manually (skip interactive wizard)
# ---------------------------------------------------------------------------
echo "=== Setting up config and archive ==="

CONFIG_DIR="$XDG_CONFIG_HOME/letterhead"
mkdir -p "$CONFIG_DIR"
mkdir -p "$LETTERHEAD_ARCHIVE_ROOT"

cat > "$CONFIG_DIR/config.toml" <<EOF
archive_root = "$LETTERHEAD_ARCHIVE_ROOT"
account_email = "alice@test.com"
auth_method = "apppassword"
sync_mode = "recent"
recent_window_weeks = 12
scheduler_cadence = "1h"
EOF

if [ -f "$CONFIG_DIR/config.toml" ]; then
    log_pass "config.toml created"
else
    log_fail "config.toml created" "file not found"
fi

# ---------------------------------------------------------------------------
# 2. Create the database with migrations via the CLI
# ---------------------------------------------------------------------------
echo ""
echo "=== Creating database via CLI ==="

# Running status will trigger ensureInitialized -> store.Open -> ApplyMigrations
$LETTERHEAD status 2>/dev/null || true

DB_PATH="$LETTERHEAD_ARCHIVE_ROOT/letterhead.db"
if [ -f "$DB_PATH" ]; then
    log_pass "database created at expected path"
else
    log_fail "database created at expected path" "DB not found at $DB_PATH"
    echo "FATAL: cannot continue without database"
    exit 1
fi

# ---------------------------------------------------------------------------
# 3. Pre-populate database with test messages
# ---------------------------------------------------------------------------
echo ""
echo "=== Populating test data ==="

NOW_UNIX=$(date +%s)
NOW_MS=$((NOW_UNIX * 1000))
HOUR_AGO_MS=$(((NOW_UNIX - 3600) * 1000))
TWO_HOURS_AGO_MS=$(((NOW_UNIX - 7200) * 1000))

sqlite3 "$DB_PATH" <<SQL
PRAGMA trusted_schema = ON;
-- Alice's messages (3 messages in thread-a1)
INSERT INTO messages (gmail_id, thread_id, history_id, internal_date, received_at,
    subject, snippet, from_addr, from_name, plain_body, html_body,
    attachment_metadata_json, raw_size_bytes, created_at, updated_at)
VALUES
    ('alice-1', 'thread-a1', 100, $TWO_HOURS_AGO_MS, $NOW_UNIX,
     'Project update from Alice', 'Here is the latest update...', 'alice@test.com', 'Alice Test',
     'Hello, here is the project update from Alice.', '', '[]', 0, $NOW_UNIX, $NOW_UNIX),
    ('alice-2', 'thread-a1', 101, $HOUR_AGO_MS, $NOW_UNIX,
     'Re: Project update from Alice', 'Thanks for the update...', 'bob@test.com', 'Bob Test',
     'Thanks for the update, Alice.', '', '[]', 0, $NOW_UNIX, $NOW_UNIX),
    ('alice-3', 'thread-a1', 102, $NOW_MS, $NOW_UNIX,
     'Re: Project update from Alice', 'No problem, glad to help...', 'alice@test.com', 'Alice Test',
     'No problem! Let me know if you need anything else.', '', '[]', 0, $NOW_UNIX, $NOW_UNIX);

-- Bob's messages (2 messages in thread-b1)
INSERT INTO messages (gmail_id, thread_id, history_id, internal_date, received_at,
    subject, snippet, from_addr, from_name, plain_body, html_body,
    attachment_metadata_json, raw_size_bytes, created_at, updated_at)
VALUES
    ('bob-1', 'thread-b1', 200, $HOUR_AGO_MS, $NOW_UNIX,
     'Meeting notes from Bob', 'Here are the meeting notes...', 'bob@test.com', 'Bob Test',
     'Meeting notes: discussed roadmap and priorities.', '', '[]', 0, $NOW_UNIX, $NOW_UNIX),
    ('bob-2', 'thread-b1', 201, $NOW_MS, $NOW_UNIX,
     'Re: Meeting notes from Bob', 'Action items attached...', 'bob@test.com', 'Bob Test',
     'Action items from the meeting are attached.', '', '[]', 0, $NOW_UNIX, $NOW_UNIX);

-- Labels for all messages
INSERT INTO message_labels (gmail_id, label) VALUES
    ('alice-1', 'INBOX'),
    ('alice-2', 'INBOX'),
    ('alice-3', 'INBOX'),
    ('bob-1', 'INBOX'),
    ('bob-2', 'INBOX'),
    ('bob-1', 'IMPORTANT');

-- Recipients
INSERT INTO message_recipients (gmail_id, role, addr, name) VALUES
    ('alice-1', 'to', 'bob@test.com', 'Bob Test'),
    ('alice-2', 'to', 'alice@test.com', 'Alice Test'),
    ('alice-3', 'to', 'bob@test.com', 'Bob Test'),
    ('bob-1', 'to', 'team@test.com', 'Team'),
    ('bob-2', 'to', 'team@test.com', 'Team');

-- Sync state for alice
INSERT INTO sync_state (account_id, history_id, bootstrap_complete, messages_synced, last_sync_at,
    uid_validity, last_uid, auth_method)
VALUES ('alice@test.com', 102, 1, 5, $NOW_UNIX, 0, 0, 'apppassword');
SQL

INSERTED=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM messages;")
if [ "$INSERTED" -eq 5 ]; then
    log_pass "inserted 5 test messages"
else
    log_fail "inserted 5 test messages" "got $INSERTED"
fi

LABEL_COUNT=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM message_labels;")
if [ "$LABEL_COUNT" -eq 6 ]; then
    log_pass "inserted 6 label rows"
else
    log_fail "inserted 6 label rows" "got $LABEL_COUNT"
fi

# ---------------------------------------------------------------------------
# 4. Test find with --label INBOX
# ---------------------------------------------------------------------------
echo ""
echo "=== Testing find ==="

output=$($LETTERHEAD find --label INBOX --json 2>/dev/null) || true
assert_contains "$output" "Alice Test" "find --label INBOX: includes alice's messages"
assert_contains "$output" "Bob Test" "find --label INBOX: includes bob's messages"
assert_contains "$output" "thread-a1" "find --label INBOX: includes thread-a1"
assert_contains "$output" "thread-b1" "find --label INBOX: includes thread-b1"

# ---------------------------------------------------------------------------
# 5. Test find with freetext search
# ---------------------------------------------------------------------------
echo ""
echo "=== Testing find with freetext ==="

output=$($LETTERHEAD find "meeting" --json 2>/dev/null) || true
assert_contains "$output" "Bob Test" "find 'meeting': finds bob's meeting notes"
assert_contains "$output" "thread-b1" "find 'meeting': returns thread-b1"
assert_not_contains "$output" "thread-a1" "find 'meeting': does not return thread-a1"

# ---------------------------------------------------------------------------
# 6. Test find with --from filter
# ---------------------------------------------------------------------------
echo ""
echo "=== Testing find --from ==="

output=$($LETTERHEAD find --from alice@test.com --json 2>/dev/null) || true
assert_contains "$output" "Alice Test" "find --from alice: includes alice's messages"
assert_contains "$output" "thread-a1" "find --from alice: returns thread-a1"

# ---------------------------------------------------------------------------
# 7. Test read by message ID
# ---------------------------------------------------------------------------
echo ""
echo "=== Testing read ==="

output=$($LETTERHEAD read alice-1 --json 2>/dev/null) || true
assert_contains "$output" "alice-1" "read alice-1: returns correct message ID"
assert_contains "$output" "Project update from Alice" "read alice-1: correct subject"

output=$($LETTERHEAD read bob-1 --json 2>/dev/null) || true
assert_contains "$output" "bob-1" "read bob-1: returns correct message ID"
assert_contains "$output" "Meeting notes" "read bob-1: correct subject"

# ---------------------------------------------------------------------------
# 8. Test read by thread (thread mode)
# ---------------------------------------------------------------------------
echo ""
echo "=== Testing read --thread ==="

output=$($LETTERHEAD read thread-a1 --thread --json 2>/dev/null) || true
assert_contains "$output" "thread-a1" "read --thread thread-a1: returns thread"
assert_contains "$output" "alice-1" "read --thread thread-a1: includes first message"
assert_contains "$output" "alice-3" "read --thread thread-a1: includes last message"

# ---------------------------------------------------------------------------
# 9. Test read with text view
# ---------------------------------------------------------------------------
echo ""
echo "=== Testing read --view text ==="

output=$($LETTERHEAD read alice-1 --view text --json 2>/dev/null) || true
assert_contains "$output" "project update from Alice" "read --view text: includes body text"

# ---------------------------------------------------------------------------
# 10. Test status
# ---------------------------------------------------------------------------
echo ""
echo "=== Testing status ==="

output=$($LETTERHEAD status --json 2>/dev/null) || true
assert_contains "$output" "alice@test.com" "status: shows configured account"
assert_contains "$output" "5" "status: shows message count"

# ---------------------------------------------------------------------------
# 11. Test find with IMPORTANT label (only bob's messages)
# ---------------------------------------------------------------------------
echo ""
echo "=== Testing find --label IMPORTANT ==="

output=$($LETTERHEAD find --label IMPORTANT --json 2>/dev/null) || true
assert_contains "$output" "Bob Test" "find --label IMPORTANT: includes bob"
assert_not_contains "$output" "thread-a1" "find --label IMPORTANT: excludes alice's thread"

# ---------------------------------------------------------------------------
# 12. Test find with --subject filter
# ---------------------------------------------------------------------------
echo ""
echo "=== Testing find --subject ==="

output=$($LETTERHEAD find --subject "Meeting" --json 2>/dev/null) || true
assert_contains "$output" "Meeting notes" "find --subject Meeting: finds meeting thread"
assert_not_contains "$output" "thread-a1" "find --subject Meeting: excludes project update thread"

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo ""
echo "================================"
echo "Results: $PASS passed, $FAIL failed"
echo "================================"
[ "$FAIL" -eq 0 ] || exit 1
