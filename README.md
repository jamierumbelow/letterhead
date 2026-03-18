# letterhead

Local-first, read-only Gmail mirror for humans and agents. Syncs your inbox to a local SQLite database with full-text search.

## Quickstart

```bash
go install github.com/jamierumbelow/letterhead/cmd/letterhead@latest

# authenticate with gmail (one-time)
gcloud auth application-default login \
  --scopes=https://www.googleapis.com/auth/gmail.readonly

# set account_email in config, then sync
echo 'account_email = "you@gmail.com"' >> ~/.config/letterhead/config.toml
letterhead sync
letterhead find quarterly report
letterhead read <thread-id> --thread
```

No explicit `init` needed -- letterhead auto-initialises on first use.

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

### 1. Set your account email

Letterhead auto-initialises on first use. Just set your Gmail address:

```bash
# creates the config dir if needed
mkdir -p ~/.config/letterhead
echo 'account_email = "you@gmail.com"' >> ~/.config/letterhead/config.toml
```

Or if you want to customise the archive location, run `letterhead init --archive-root ~/mail-archive` first.

### 2. Authenticate

The easiest way (if you have `gcloud` installed):

```bash
gcloud auth application-default login \
  --scopes=https://www.googleapis.com/auth/gmail.readonly
```

That's it -- letterhead picks up the credentials automatically.

Alternatively, `letterhead auth` will run an interactive OAuth flow if you provide your own client credentials (see [Auth](#authentication) below).

### 3. Sync your inbox

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

### JSON output

Every command supports `--json` and `--jsonl` for structured output, useful for piping to `jq` or feeding to AI agents:

```bash
letterhead find quarterly report --json | jq '.results[].subject'
letterhead find --from boss@company.com --jsonl | head -5
letterhead status --json
```

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

Letterhead tries these auth methods in order:

1. **gcloud application-default credentials** (recommended) -- zero config if you have `gcloud`. Just run `gcloud auth application-default login --scopes=https://www.googleapis.com/auth/gmail.readonly` once and letterhead picks it up automatically.

2. **Stored OAuth token** -- from a previous `letterhead auth` session.

3. **Interactive OAuth flow** -- opens your browser to authenticate. Requires client credentials from one of:
   - A `credentials.json` file at `~/.config/letterhead/credentials.json` (create a Desktop app in [Google Cloud Console](https://console.cloud.google.com))
   - Environment variables `LETTERHEAD_CLIENT_ID` and `LETTERHEAD_CLIENT_SECRET`

For most users, option 1 is the simplest path.

## How it works

- **SQLite + FTS5**: Messages are stored in a single SQLite database with WAL mode. A full-text search index covers subject, body, snippet, and sender fields.
- **Gmail API**: Uses the Gmail API with read-only scope. OAuth tokens are stored with 0600 permissions.
- **Resumable sync**: Bootstrap sync tracks progress in the database. If interrupted, it picks up where it left off.
- **Summary-first**: `find` returns compact thread summaries. Use `read` to get the full content.

## License

See [LICENSE](LICENSE).
