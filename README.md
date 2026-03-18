# letterhead

Local-first, read-only Gmail mirror for humans and agents. Syncs your inbox to a local SQLite database with full-text search.

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

### 1. Create Google Cloud credentials

You need a Google Cloud project with the Gmail API enabled:

1. Go to [Google Cloud Console](https://console.cloud.google.com)
2. Create a project (or use an existing one)
3. Enable the **Gmail API**
4. Go to **Credentials** > **Create Credentials** > **OAuth client ID**
5. Choose **Desktop app**, give it a name, and download the JSON
6. Save it as `~/.config/letterhead/credentials.json`

Alternatively, set environment variables:

```bash
export LETTERHEAD_CLIENT_ID="your-client-id"
export LETTERHEAD_CLIENT_SECRET="your-client-secret"
```

### 2. Initialise the archive

```bash
letterhead init
```

This creates the config file at `~/.config/letterhead/config.toml` and the SQLite database under `~/.local/share/letterhead/archive/`.

You can choose a custom archive location:

```bash
letterhead init --archive-root ~/mail-archive
```

### 3. Set your account email

Edit `~/.config/letterhead/config.toml` and add your Gmail address:

```toml
account_email = "you@gmail.com"
```

### 4. Sync your inbox

```bash
letterhead sync
```

On first run this opens your browser for OAuth consent, then downloads your inbox. Progress is shown live. The sync is resumable -- if interrupted, just run `sync` again.

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

## How it works

- **SQLite + FTS5**: Messages are stored in a single SQLite database with WAL mode. A full-text search index covers subject, body, snippet, and sender fields.
- **Gmail API**: Uses the Gmail API with read-only scope. OAuth tokens are stored with 0600 permissions.
- **Resumable sync**: Bootstrap sync tracks progress in the database. If interrupted, it picks up where it left off.
- **Summary-first**: `find` returns compact thread summaries. Use `read` to get the full content.

## License

See [LICENSE](LICENSE).
