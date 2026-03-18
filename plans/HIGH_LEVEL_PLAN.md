# Letterhead

`letterhead` is a local, read-only Gmail mirror for humans and agents, with fast local search and deliberate escalation from summaries to full message content.

It is **not** an MCP product, a hosted service, a semantic pipeline, or an LLM answer engine.

## Product Definition

### Goal

Build a Go CLI that:

1. connects to Gmail with read-only scope
2. mirrors a selected slice of mail locally
3. searches that mirror locally with no Gmail round-trip at query time
4. returns compact summaries by default
5. makes full-message access explicit via `read`

### Core Product Principles

- Gmail-only for v1
- read-only by scope and by product surface
- local-first: sync touches the network, `find` and `read` do not
- summary-first retrieval is the default contract
- explicit operations: init, sync, status, repair, rebuild
- fast local search matters more than broad feature count
- keep the first release CLI-first and JSON-friendly rather than adding MCP or a server

Do **not** adopt in v1:

- Semantic Context Shadow as a required subsystem
- MCP as a primary interface
- PII masking as part of the core sync path
- real-time push / IMAP IDLE / PubSub
- attachment OCR
- TUI dashboard

## Final V1 Scope

### In Scope

- one local archive per Gmail account
- installed-app OAuth with Gmail read-only scope
- sync modes: `inbox`, `recent`, `full`
- local SQLite storage of normalized messages and metadata
- SQLite FTS5 search
- summary-first `find`
- explicit `read`
- `status`
- incremental sync
- explicit scheduler install
- `doctor`
- index rebuild / repair paths
- human-readable output plus `--json` and `--jsonl`

### Out of Scope

- outbound email actions
- MCP server
- `ask` / built-in LLM synthesis
- semantic entity/commitment extraction
- attachment text extraction or OCR
- multi-provider support
- multi-account UX everywhere on day one
- daemon / always-on background service

## Key Decisions To Lock Now

### 1. Storage: SQLite Is The Source Of Truth

Use **one SQLite database per account** as the canonical store and search index.

Why this is the best synthesis:

- it is materially simpler than JSON archive + SQLite catalog + separate index
- it avoids dual-write drift between archive and index
- FTS5 keeps search inside the same file and transaction boundary
- operational tooling like integrity checks and rebuilds become much simpler

For v1, do **not** maintain per-message JSON files alongside the database. That is a meaningful complexity increase without enough immediate value.

If inspectability becomes important later, add an export/debug path rather than carrying two storage systems from day one.

### 2. Search Backend: SQLite FTS5, Not `qmd`, Not Bleve

Use SQLite FTS5 now.

- no extra index service
- no separate index directory
- no early backend abstraction ceremony
- adequate relevance, phrase search, prefix search, and performance for the expected mailbox sizes

Keep the search/query code isolated so the backend could be swapped later, but do **not** design a general pluggable index interface until there is a second real backend worth supporting.

### 3. Product Interface: CLI First, Not MCP First

The CLI is enough for v1 if it has:

- stable human-readable defaults
- `--json`
- `--jsonl`
- predictable exit codes

That already makes the tool agent-usable through ordinary subprocess calls. MCP can be added later as a thin wrapper around stable CLI/library primitives if it proves necessary.

### 4. Retrieval Contract: Summary First

This is the most important product decision.

- `find` returns thread-oriented summaries by default
- `read` is the explicit step for deeper content
- broad queries should not dump full message bodies by default

This gives Letterhead a clear product shape and avoids turning it into a raw mailbox dump.

### 5. Sync Modes: `inbox`, `recent`, `full`

Use these user-facing modes:

- `inbox`: messages currently in Gmail Inbox
- `recent`: messages after a configurable cutoff; default 12 weeks
- `full`: entire mailbox history

Default should be `recent` with a 12-week window.

This is clearer than `light`/`heavy`, and more explicit than `current`.

### 6. Auth: Keep It Practical

Use installed-app OAuth with `gmail.readonly`.

Pragmatic auth stance:

- support browser flow with a paste-back fallback for headless cases
- store the refresh token outside the main config file
- use a `0600` token file first; add OS keychain integration later if needed
- support either a maintained shared OAuth client or user-supplied credentials, but do not block the project on app-verification work

This keeps the product moving while preserving a clean upgrade path.

## User Experience

### Primary Commands

```text
letterhead init
letterhead status
letterhead sync [--mode inbox|recent|full]
letterhead find [query] [filters...]
letterhead read <id> [--view summary|text|full] [--thread]
letterhead doctor
letterhead sync install
letterhead sync uninstall
```

### Query Shape

Support both:

- free-text query terms
- structured flags like `--from`, `--to`, `--subject`, `--label`, `--after`, `--before`, `--has-attachment`

The structured flags should be first-class. Agents should not be forced to construct query strings when flags are clearer.

### `find` Output

`find` should default to thread summaries that include:

- stable result id
- thread id
- subject
- key participants
- latest activity time
- message count
- snippet
- matched fields
- a clear read handle

Human-readable output should be compact and scannable.

`--json` should return stable objects.

`--jsonl` should stream one result per line for agent pipelines.

### `read` Output

`read` should support three explicit views:

- `summary`: richer single-item summary
- `text`: normalized body text
- `full`: full archived representation for debugging

If `find` returns thread summaries, `read` should make it easy to read either:

- the representative message
- the whole thread

## Architecture

### 1. CLI / Output Layer

- Cobra is a reasonable choice for command structure and completions
- keep the visible top-level UX small
- human-readable by default, with stable JSON modes

### 2. Config Layer

Use a simple local config file to store:

- archive root
- selected account
- sync mode
- recent window
- scheduler cadence

Do not store tokens in the main config file.

### 3. Gmail Layer

Responsibilities:

- OAuth client creation
- message listing
- message fetch
- History API delta fetch
- MIME parsing and normalization

Normalization rules:

- prefer `text/plain` when present
- derive readable plain text from HTML when needed
- strip only obviously noisy markup or transport artifacts
- store attachment metadata, but not attachment bodies in v1

### 4. Store Layer

SQLite tables should at minimum cover:

- `messages`
- `message_labels`
- `sync_state`
- `sync_runs` or similar journaling table for durable checkpointing

The `messages` table should store:

- Gmail message id
- thread id
- history id
- timestamps
- subject
- snippet
- sender / recipients
- labels
- normalized plain-text body
- optional HTML body
- attachment metadata

FTS5 should index the searchable text directly from this store.

### 5. Query Layer

Search semantics belong to Letterhead, not Gmail.

Implement a small internal query model that supports:

- freetext
- sender/recipient filters
- subject filter
- label filter
- date range
- attachment presence

Search should be thread-aware by default. Thread summaries can be computed at query time first; materialize cached thread-summary tables only if profiling proves it is necessary.

### 6. Sync Layer

Use two sync modes internally:

- bootstrap sync
- incremental sync

#### Bootstrap

Keep bootstrap **progressive and resumable**.

Recommended approach:

1. capture current `historyId`
2. list message ids for the chosen mode
3. skip ids already stored locally
4. fetch messages in bounded batches
5. write each batch transactionally
6. expose partial progress through `status`
7. mark bootstrap complete only after local writes succeed

Important synthesis decision:

Do **not** start with a complicated metadata-first / hydrate-later pipeline unless profiling shows it is needed. The simpler resumable batch bootstrap is the right v1 move. If very large archives later justify a two-stage bootstrap, add it as an optimization, not as the starting architecture.

#### Incremental

After bootstrap:

- read the last successful `historyId`
- fetch deltas from Gmail History API
- apply adds/updates/deletes transactionally
- only advance checkpoint after local commit succeeds

If history has expired, run a bounded repair flow for the selected mode rather than trying to patch around an invalid checkpoint indefinitely.

### 7. Locking And Reliability

- enforce a single-writer lock for sync, repair, and rebuild
- retries should handle 429/5xx with backoff and jitter
- interrupted syncs should resume cleanly
- `find` and `read` must continue to work during non-conflicting maintenance operations when possible

### 8. Scheduler

The tool should not require a daemon in v1.

Use an explicit install command for periodic sync:

- macOS: `launchd`
- Linux: `systemd --user`
- fallback: cron if necessary

Do not silently install background jobs during auth or init.

## Operational Commands

### `status`

This should be the main "is Letterhead healthy?" command.

Show:

- connected account
- archive path
- sync mode
- message/thread counts
- bootstrap progress if incomplete
- last successful sync time
- scheduler state
- database/index health summary

### `doctor`

Check at least:

- config parseability
- token presence / auth validity
- Gmail profile access
- SQLite integrity
- checkpoint health
- scheduler installation
- available disk space

### Rebuild / Repair

Support:

- FTS rebuild from the local SQLite store
- sync repair when checkpoint/history state is invalid

These recovery paths are part of the product, not afterthoughts.

## Repository Shape

```text
cmd/letterhead/
internal/
  auth/
  cli/
  config/
  gmail/
  output/
  query/
  scheduler/
  store/
  syncer/
  diagnostics/
pkg/types/
docs/
```

Keep provider-specific code in `internal/gmail`. Do not build a provider abstraction in v1.

## Delivery Plan

### Phase 0: Contracts And Skeleton

- initialize the Go module and command skeleton
- define config format and path conventions
- define normalized message schema
- define SQLite schema including FTS5
- define the output contracts for `status`, `find`, and `read`
- lock the summary-first retrieval contract

Exit criteria:

- `letterhead init`
- `letterhead status`
- concrete JSON contracts for the main commands

### Phase 1: Auth + Bootstrap Sync

- implement Gmail read-only auth
- persist tokens safely outside config
- implement `inbox` bootstrap first
- make bootstrap resumable
- show bootstrap progress in `status`

Exit criteria:

- a user can authenticate
- sync a real inbox locally
- restart interrupted bootstrap without duplicating data

### Phase 2: Local Search + Read

- add FTS5-backed `find`
- group results by thread
- implement structured filters and JSON output
- implement `read` with explicit view levels

Exit criteria:

- useful local search with no network calls
- summary-first retrieval works cleanly

### Phase 3: Incremental Sync + Scheduling + Doctor

- implement History API incremental sync
- add checkpoint repair flow
- add explicit scheduler install/uninstall
- add `doctor`

Exit criteria:

- archive stays current without manual full re-sync
- operational issues have visible recovery paths

### Phase 4: Hardening

- add `recent` and `full` modes
- improve MIME normalization and HTML handling
- add benchmark coverage
- add fixture-based integration tests
- consider lightweight local audit logging for `read` and `find`

Exit criteria:

- large-mailbox behavior is predictable
- regressions are measurable
- day-to-day UX remains simple

## Test And Verification Strategy

The plan should include automated checks for:

- MIME decoding edge cases
- bootstrap resume behavior
- checkpoint advancement only after successful writes
- history expiration repair flow
- thread-grouped search correctness
- JSON output stability
- SQLite migration and integrity checks

Maintain fixtures for:

- multipart plain + HTML messages
- quoted-printable and base64url payloads
- large threads
- deleted/changed messages in History API deltas

## Future Layer, Not V1

After the archive/search product is solid, there are two legitimate extension paths:

### 1. Agent Convenience Layer

- MCP wrapper around the stable CLI/library primitives
- optional `ask` command that uses local search results and an opt-in model

### 2. Derived Intelligence Layer

- thread summaries beyond simple snippets
- commitment extraction
- entity/project views

If these are added, they should be **derived from the local archive**, never entangled with the correctness-critical sync path.

## Final Recommendation

If I had to lock the plan today, I would lock these decisions immediately:

- Gmail-only, read-only, CLI-first
- SQLite as both canonical store and FTS5 search index
- no MCP in v1
- no built-in LLM answer flow in v1
- no semantic shadow in v1
- `init`, `status`, `sync`, `find`, `read`, `doctor` as the primary surface
- summary-first retrieval as the product-defining interaction
- progressive resumable bootstrap now, more complex hydration only if proven necessary
- explicit scheduler install
- strong repair and rebuild paths from the beginning

That is the strongest combination of the three plans because it keeps the best product ideas, chooses the simplest architecture that can actually deliver them, and avoids spending the project's early complexity budget on the wrong things.
