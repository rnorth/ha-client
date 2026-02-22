# ha-client Design

**Date:** 2026-02-22
**Status:** Approved

## Overview

A Go CLI tool (`ha-client`) for interacting with Home Assistant instances, targeting both human users and AI agents. Provides a single self-contained binary with no runtime dependencies.

Inspired by [home-assistant-cli](https://github.com/home-assistant-ecosystem/home-assistant-cli) (Python), but built for single-binary distribution and designed with a kubectl-style UX.

## Goals

- Single self-contained binary, no Python/pip dependency
- kubectl-style resource/verb model for consistency and composability
- Usable by both humans (table output) and agents (JSON output)
- Secure credential storage via OS keychain

## Command Model

**Structure:** `ha-client <resource> <verb> [name] [flags]`

### Commands

| Resource | Verbs | Description |
|---|---|---|
| `state` | `list`, `get`, `describe`, `set` | Entity states |
| `action` | `list`, `call` | HA actions (formerly "services") |
| `event` | `watch` | Real-time SSE event stream |
| `area` | `list`, `get`, `create`, `delete` | Area registry |
| `device` | `list`, `get`, `describe` | Device registry |
| `automation` | `list`, `get`, `describe`, `trigger`, `enable`, `disable` | Automations |
| `entity` | `list`, `get`, `describe` | Entity registry |
| *(global)* | `login`, `logout`, `info` | Credentials + server info |

### Global Flags

- `--output table|json|yaml` — output format (default: auto-detect TTY)
- `--server <url>` — override HA server URL
- `--token <token>` — override auth token

## Configuration & Credentials

Resolution order (highest priority first):

1. `--server` / `--token` CLI flags
2. `HASS_SERVER` / `HASS_TOKEN` environment variables
3. OS keychain (macOS Keychain, Linux libsecret, Windows Credential Manager)
4. Config file: `~/.config/ha-client/config.yaml`

`ha-client login` — interactive prompt storing credentials to OS keychain
`ha-client logout` — removes credentials from OS keychain

## Output Behaviour

- **TTY detected:** table format by default
- **No TTY (piped):** JSON by default
- Always overridable with `--output`
- Data → stdout, errors → stderr
- Exit codes: `0` success, `1` error, `2` not found

## Architecture

```
ha-cli/
├── main.go
├── cmd/
│   ├── root.go        # Root command, global flags
│   ├── login.go       # ha-client login / logout
│   ├── info.go        # ha-client info
│   ├── state.go       # ha-client state {list,get,describe,set}
│   ├── action.go      # ha-client action {list,call}
│   ├── event.go       # ha-client event watch
│   ├── area.go        # ha-client area {list,get,create,delete}
│   ├── device.go      # ha-client device {list,get,describe}
│   ├── automation.go  # ha-client automation {list,get,describe,trigger,enable,disable}
│   └── entity.go      # ha-client entity {list,get,describe}
├── internal/
│   ├── client/
│   │   └── client.go  # HA REST API client (thin HTTP wrapper)
│   ├── config/
│   │   └── config.go  # Credential resolution logic
│   └── output/
│       └── output.go  # Table/JSON/YAML rendering
└── go.mod
```

## Key Dependencies

- [github.com/spf13/cobra](https://github.com/spf13/cobra) — CLI framework
- [github.com/zalando/go-keyring](https://github.com/zalando/go-keyring) — OS keychain abstraction
- [gopkg.in/yaml.v3](https://gopkg.in/yaml.v3) — YAML output and config file parsing
- [github.com/olekukonko/tablewriter](https://github.com/olekukonko/tablewriter) — table rendering

## HA API Endpoints Used

- `GET /api/` — info
- `GET /api/states` — list states
- `GET /api/states/<entity_id>` — get state
- `POST /api/states/<entity_id>` — set state
- `GET /api/services` — list actions
- `POST /api/services/<domain>/<service>` — call action
- `GET /api/events` — list event types
- `GET /api/events/<event_type>` (SSE) — watch events
- WebSocket API or REST for automation control
- `GET /api/config` — server config / info
