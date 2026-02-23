# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
mise run build    # build ./ha-client binary
mise run test     # run all tests (go test ./... -v)
mise run clean    # remove ./ha-client
mise run install  # build and copy to /usr/local/bin/ha-client
```

Run a single test:
```bash
go test ./internal/config/... -run TestName -v
```

Go version: 1.25 (managed by mise).

## Architecture

The project is a kubectl-style CLI for Home Assistant. The binary is `ha-client`; the module is `github.com/rnorth/ha-client`.

**Transport layers** (`internal/client/`):
- `RESTClient` — HTTP client for `/api/*` endpoints (states, actions, server info)
- `WSClient` — WebSocket client for `/api/websocket` (area/device/entity registry, event streaming). Handles the HA auth handshake on connect and uses an atomic message ID counter.
- `types.go` — all shared domain types (`State`, `Area`, `Device`, `EntityEntry`, `WSMessage`, etc.)

**Credential resolution** (`internal/config/`):
Priority: CLI flags > `HASS_SERVER`/`HASS_TOKEN` env vars > OS keychain (`go-keyring`) > `~/.config/ha-client/config.yaml`. Keychain saves are atomic — partial failures fall back to file.

**Output rendering** (`internal/output/`):
`DetectFormat` auto-selects table (TTY) or JSON (piped). `Render` dispatches to JSON/YAML/table renderers. Table format is kubectl-style: left-aligned, space-padded columns, headers derived from JSON struct tags.

**Commands** (`cmd/`):
Each file registers subcommands on `rootCmd` (cobra). Commands call `resolveConfig()` + `newWSClient()` or create a `RESTClient` directly. The `resolveFormat()` helper detects output format from the `-o` flag and stdout TTY state.

`describe` subcommands always output YAML at a TTY and JSON when piped.
