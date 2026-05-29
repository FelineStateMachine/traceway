# traceway-cli

A Go CLI for the [Traceway](https://github.com/tracewayapp/traceway) observability platform — exceptions, logs, endpoints, metrics. Designed to be **first-class for both LLMs (invoking via shell tools) and humans** (with `gh`-style ergonomics).

## Install

### Released binary (Linux / macOS)

```bash
curl -fsSL https://cli.tracewayapp.com/install.sh | bash
# pin a version: ... | TRACEWAY_CLI_VERSION=1.2.3 bash
# custom dir:    ... | TRACEWAY_BIN_DIR="$HOME/.local/bin" bash
```

Windows (PowerShell):

```powershell
iwr -useb https://cli.tracewayapp.com/install.ps1 | iex
```

Or download an archive directly from the [releases page](https://github.com/tracewayapp/traceway/releases) (tagged `cli/vX.Y.Z`), verify it against `checksums.txt`, and put `traceway` on your `PATH`.

### From source

This repo ships a Nix dev shell with Go 1.26, `just`, `gotestsum`, `golangci-lint`, `govulncheck`, and `gh`:

```bash
nix develop
just build         # produces ./bin/traceway
```

Or vanilla Go:

```bash
go build -o bin/traceway ./cmd/traceway
```

## Quick start

```bash
# 1. log in (creates ~/.config/traceway/config.json + ~/.local/state/traceway/state.json)
traceway login --url https://cloud.tracewayapp.com

# 2. pick a project (one-time; future calls use it implicitly)
traceway projects list
traceway projects use <project-id>

# 3. ask questions
traceway exceptions list --since 24h
traceway logs query --since 1h --search "OutOfMemory"
traceway endpoints list --since 1h
traceway metrics query --name http.server.duration --aggregation avg --since 1h
```

## Commands

| Command | Purpose |
|---|---|
| `traceway login` | Authenticate and store the JWT |
| `traceway logout` | Forget the stored JWT for a profile |
| `traceway profiles {list,use}` | Manage multiple Traceway accounts/instances |
| `traceway projects {list,use}` | List or select the active project |
| `traceway exceptions list` | Recent grouped exceptions |
| `traceway exceptions show <hash>` | A single exception group + occurrences |
| `traceway exceptions archive <hash>...` | Archive one or more groups (mutating; needs `--yes` non-interactively) |
| `traceway exceptions unarchive <hash>...` | Unarchive (mutating; needs `--yes` non-interactively) |
| `traceway logs query` | Query logs with severity / service / search filters |
| `traceway endpoints list` | Per-endpoint p50/p95/p99 stats |
| `traceway metrics query` | Time-series metric queries |

Run `traceway <command> --help` for full per-command flags.

## Profiles

Multiple Traceway instances or accounts coexist via profiles. Configuration (URL, username) lives in `$XDG_CONFIG_HOME/traceway/config.json` so it can be checked in or managed declaratively (e.g. NixOS); credentials and the active project live in `$XDG_STATE_HOME/traceway/state.json`.

```bash
traceway login --url https://traceway.example.com --profile work
traceway profiles list
traceway profiles use work
traceway --profile personal exceptions list   # one-off override
```

## Output formats

The `--output` flag picks the format. The default is `table` on a TTY and `json` otherwise — i.e. piping always gets machine-readable output.

| Format | Use |
|---|---|
| `table` | Human-friendly columns (default on TTY) |
| `json` | Compact JSON, one record per line. Default when stdout isn't a TTY |
| `yaml` | YAML rendering of the same data |

`--fields a,b,c` projects list responses to just those keys (the `pagination` wrapper passes through unchanged):

```bash
traceway exceptions list --output json --fields exceptionHash,count,lastSeen
```

## Errors

Every error writes a stable JSON envelope to stderr (in `json` / `yaml` modes — prose in `table` mode) and exits with one of:

| Exit | Meaning |
|---|---|
| 0 | Success |
| 1 | Generic / API error |
| 2 | Usage error (bad flags, missing confirmation, invalid time range) |
| 3 | Connection failure |
| 4 | Auth failure (`not_authenticated`, `token_expired`, `forbidden`) |
| 5 | Not found |
| 6 | Rate limited |
| 7 | Server (5xx) |

Envelope shape:

```json
{"error":"token_expired","message":"session expired or invalid","hint":"traceway login","exit_code":4}
```

The `error` field is a stable snake_case identifier. LLMs/scripts can branch on it.

## Mutations + confirmation

`exceptions archive` and `exceptions unarchive` require explicit consent because they change server state:

- Pass `--yes` to skip the prompt.
- Or set `TRACEWAY_ASSUME_YES=1` in the environment.
- Or run interactively and answer the `Continue? [y/N]` prompt.

Calling a mutating command from a non-TTY context (script, LLM tool call) without one of the opt-ins fails immediately with `usage_error` (exit 2) — no hung prompts.

## Smoke testing

Unit/CLI tests run by default and never touch a network:

```bash
just test
```

End-to-end tests against a real Traceway instance are gated behind a build tag and read connection info from env vars:

```bash
export TRACEWAY_SMOKE_URL=https://traceway.example.com
export TRACEWAY_SMOKE_USERNAME=you@example.com
export TRACEWAY_SMOKE_PASSWORD=...
export TRACEWAY_SMOKE_PROJECT_ID=...
just smoke-test
```

If any of those vars is missing, the smoke tests skip cleanly rather than fail.

To run the same suite against an ephemeral backend (SQLite, seeded automatically — no live instance or env vars needed), use:

```bash
just smoke-ci    # or: ./scripts/smoke-ci.sh
```

This builds and starts the embedded backend, seeds a user + project, runs the smoke suite against it, and tears everything down. It's the same path CI runs (`.github/workflows/cli.yml`), so backend API changes that break the CLI's hand-rolled request/response types get caught.

## Contributing

```bash
just check   # lint + test + vulncheck
```

The library at `pkg/client` deliberately has zero CLI dependencies — it's importable directly by other Go programs (e.g. a future MCP server).
