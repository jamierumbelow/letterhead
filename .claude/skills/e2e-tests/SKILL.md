---
name: e2e-tests
description: Run and extend the Go e2e test suite for letterhead CLI commands. Use when making changes to CLI behaviour, adding commands, or debugging broken flows.
---

# e2e-tests – End-to-End CLI Test Suite

Letterhead has a fast Go e2e test suite that exercises every CLI command through
cobra directly (no subprocesses, no `go run`). The full suite runs in under a
second.

## Running

```bash
# Run all e2e tests
go test ./internal/cli/ -run TestE2E -v

# Run a specific test
go test ./internal/cli/ -run TestE2E_FindByLabel -v

# Run the full test suite (e2e + unit)
go test ./...
```

## When to run

- **After any change to CLI commands** — find, read, status, accounts, init, auth
- **After changes to config loading/saving** — especially migration logic
- **After changes to the store layer** — queries, schema, upsert
- **After changes to output formatting** — JSON/JSONL/human mode
- **Before committing** — run at minimum `go test ./internal/cli/ -run TestE2E`

## When to extend

Add new e2e tests when:

- Adding a new CLI command or subcommand
- Adding new flags to existing commands
- Fixing a bug — write the test first to reproduce, then fix
- Changing account resolution or multi-account behaviour
- Modifying the config format or migration logic

## How it works

### Test environment

Every test uses `setupTestEnv(t)` which creates a fully isolated environment:

- Temporary directories for `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, and archive root
- Uses `t.Setenv` so env vars are restored after the test
- Uses `t.TempDir` so everything is cleaned up automatically

Because `t.Setenv` is used, tests **cannot** use `t.Parallel()`. This is fine —
the suite is fast enough without parallelism.

### Setting up state

```go
env := setupTestEnv(t)

// Write config with accounts
env.writeConfig(config.Config{
    Accounts: []config.AccountConfig{
        {Email: "alice@test.com", AuthMethod: config.AuthMethodAppPassword},
    },
    DefaultAccount: "alice@test.com",
})

// Seed messages via the store layer
env.seedMessages("alice@test.com", []types.Message{...})

// Seed sync state
env.seedSyncState(&store.SyncState{
    AccountID: "alice@test.com",
    ...
})
```

Or use `setupPopulatedEnv(t)` for the common case: one account (alice@test.com)
with 5 messages across 2 threads, sync state set, ready to query.

### Running commands

```go
// Run a command, get stdout/stderr/error
stdout, stderr, err := env.run("find", "--label", "INBOX", "--json")

// Run and fail the test on error
stdout := env.mustRun("status", "--json")

// Run with --json and decode into a typed struct
var output types.FindOutput
err := env.runJSON(&output, "find", "--from", "alice@test.com")
```

Commands execute through `NewRootCommand()` with `SetOut`/`SetErr` capturing
output — the same cobra wiring as production, but no process overhead.

### Test fixtures

Two fixture functions provide reusable test data:

- `aliceMessages()` — 3 messages in thread-a1 (project update thread between Alice and Bob)
- `bobMessages()` — 2 messages in thread-b1 (meeting notes from Bob, labelled IMPORTANT)

Timestamps use package-level `now`, `hourAgo`, `twoHrsAgo` variables.

## What's covered

| Area | Tests |
|------|-------|
| Init | creates config + DB, idempotent re-init |
| Status | before init, after init (empty), with data |
| Accounts | list empty/single/multi, set default, unknown email error |
| Find | all, freetext, --from, --label, --subject, no results, --limit, message counts |
| Read | by message ID, --view text, --thread, thread via message ID, not found |
| Multi-account | status with breakdown, cross-account find |
| Errors | conflicting output modes, missing arguments |
| Legacy | flat config migration to accounts[] |
| Output | human mode doesn't produce JSON |

## File location

All e2e tests live in `internal/cli/e2e_test.go`. They are in the `cli` package
so they can access internal types like `accountInfo` and use `NewRootCommand()`
directly.

## Patterns to follow

1. **Name tests `TestE2E_*`** so they can be run as a group with `-run TestE2E`
2. **Use `env.runJSON` for assertions** — decode into typed structs, not string matching
3. **Test JSON output** — it's the stable contract; human output tests just verify it's not JSON
4. **One assertion focus per test** — keep tests small and specific
5. **Seed data through the store layer** — use `env.seedMessages`, not raw SQL
