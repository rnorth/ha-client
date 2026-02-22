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
| `event` | `watch` | Real-time event stream (WebSocket) |
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

`ha-client login` — interactive prompt; stores to OS keychain when available, falls back to config file in headless/container environments where no keychain is present
`ha-client logout` — removes credentials from keychain or config file

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
│   │   ├── rest.go    # HA REST API client (states, actions, info)
│   │   └── ws.go      # HA WebSocket API client (events, registry, automations)
│   ├── config/
│   │   └── config.go  # Credential resolution logic
│   └── output/
│       └── output.go  # Table/JSON/YAML rendering
└── go.mod
```

## Key Dependencies

- [github.com/spf13/cobra](https://github.com/spf13/cobra) — CLI framework
- [github.com/zalando/go-keyring](https://github.com/zalando/go-keyring) — OS keychain abstraction (with config-file fallback when keychain unavailable)
- [github.com/gorilla/websocket](https://github.com/gorilla/websocket) — WebSocket client for HA WS API
- [gopkg.in/yaml.v3](https://gopkg.in/yaml.v3) — YAML output and config file parsing
- [github.com/olekukonko/tablewriter](https://github.com/olekukonko/tablewriter) — table rendering

## HA API Endpoints Used

### REST API (`/api/`)
- `GET /api/` — info / health check
- `GET /api/config` — server config (location, timezone, version)
- `GET /api/states` — list entity states
- `GET /api/states/<entity_id>` — get one entity state
- `POST /api/states/<entity_id>` — set entity state
- `GET /api/services` — list available actions
- `POST /api/services/<domain>/<service>` — call an action

### WebSocket API (`/api/websocket`)

All registry and real-time operations go through the HA WebSocket API using authenticated message commands:

- `subscribe_events` — stream events (for `event watch`)
- `config/area_registry/list` — list areas
- `config/area_registry/create` — create area
- `config/area_registry/delete` — delete area
- `config/device_registry/list` — list/get devices
- `config/entity_registry/list` — list/get entities
- `config/entity_registry/get` — describe entity
- `automation/config` — list/get automations
- `call_service` with `automation.trigger` / `automation.turn_on` / `automation.turn_off` — trigger/enable/disable automations
