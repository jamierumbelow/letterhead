# letterhead

Local-first, read-only Gmail mirror for humans and agents. Syncs your inbox to a local SQLite database with full-text search.

## Quickstart

```bash
go install github.com/jamierumbelow/letterhead/cmd/letterhead@latest

letterhead sync
# > Welcome to letterhead! Let's get you set up.
# > Gmail address: you@gmail.com
# > Archive location [~/.local/share/letterhead/archive]:
# (opens browser for OAuth consent, then syncs)
# > Synced 1432 messages (2m14s)

letterhead find quarterly report
letterhead read <thread-id> --thread
```

## Install

```bash
go install github.com/jamierumbelow/letterhead/cmd/letterhead@latest
```

Or build from source:

```bash
git clone https://github.com/jamierumbelow/letterhead.git
cd letterhead
go build -o letterhead ./cmd/letterhead
```

## Setup

On first run, letterhead walks you through setup interactively -- asking for your Gmail address and archive location. You can also run `letterhead init` to do this explicitly, or pass `--archive-root` to customise the location.

### 1. Authenticate

```bash
letterhead auth
```

Opens your browser for Google OAuth consent (read-only access). You'll need to provide OAuth client credentials -- see [Authentication](#authentication) below.

### 2. Sync your inbox

```bash
letterhead sync
```

Downloads your inbox. Progress is shown live. The sync is resumable -- if interrupted, just run `sync` again.

## Usage

### Search

```bash
# Full-text search
letterhead find quarterly report

# Filter by sender
letterhead find --from alice@example.com

# Filter by date range
letterhead find --after 2025-01-01 --before 2025-06-01

# Filter by label
letterhead find --label IMPORTANT

# Combine filters
letterhead find budget --from finance@company.com --has-attachment

# Pagination
letterhead find --limit 50 --offset 20
```

### Read

```bash
# Read a message (summary view, the default)
letterhead read <message-id>

# Read the plain text body
letterhead read <message-id> --view text

# Read the full stored representation
letterhead read <message-id> --view full

# Read an entire thread
letterhead read <thread-id> --thread
```

The `read_handle` from `find` output can be passed directly to `read`.

### Status

```bash
letterhead status
```

Shows account, archive path, message/thread counts, sync progress, and database health.

### JSON output (robot mode)

Every command supports `--json` and `--jsonl` for structured output. When stdout is piped (not a TTY), JSON is the default -- no flag needed.

```bash
# Explicit
letterhead find quarterly report --json | jq '.results[].subject'
letterhead status --json

# Auto-detected (piped)
letterhead find quarterly report | jq '.results[].subject'
letterhead status | cat

# JSONL for streaming
letterhead find --from boss@company.com --jsonl | head -5
```

**Compact help**: Running `letterhead` with no args in JSON mode emits a ~100 token command index:

```bash
letterhead --json
# {"commands":[{"name":"find","short":"Search the local archive",...}],"flags":["--json","--jsonl","--account <email>"]}
```

**Structured errors** go to stderr with exit code, error code, and recovery hint:

```json
{"ok":false,"error":{"code":"not_found","exit_code":7,"message":"thread \"abc\" not found","hint":"letterhead find <query>"}}
```

**Exit codes**: 0=success, 1=usage, 2=lock conflict, 3=auth, 4=store, 5=network, 6=not initialized, 7=not found.

## Config

Config lives at `~/.config/letterhead/config.toml`:

```toml
archive_root = "~/.local/share/letterhead/archive"
account_email = "you@gmail.com"
sync_mode = "recent"           # inbox | recent | full
recent_window_weeks = 12
scheduler_cadence = "1h"
```

## Authentication

Letterhead needs OAuth client credentials to authenticate with Gmail. You can provide these in two ways:

1. **credentials.json** (recommended) -- create a Desktop app in [Google Cloud Console](https://console.cloud.google.com), enable the Gmail API, download the credentials JSON, and save it as `~/.config/letterhead/credentials.json`.

2. **Environment variables** -- set `LETTERHEAD_CLIENT_ID` and `LETTERHEAD_CLIENT_SECRET`.

Then run `letterhead auth` (or just `letterhead sync` -- it will prompt you automatically). Your browser opens for OAuth consent, and letterhead stores the token locally with 0600 permissions.

This is a one-time setup. After that, letterhead refreshes tokens automatically.

## How it works

- **SQLite + FTS5**: Messages are stored in a single SQLite database with WAL mode. A full-text search index covers subject, body, snippet, and sender fields.
- **Gmail API**: Uses the Gmail API with read-only scope. OAuth tokens are stored with 0600 permissions.
- **Resumable sync**: Bootstrap sync tracks progress in the database. If interrupted, it picks up where it left off.
- **Summary-first**: `find` returns compact thread summaries. Use `read` to get the full content.

## License

See [LICENSE](LICENSE).
