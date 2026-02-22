# ha-client

A kubectl-style command-line client for [Home Assistant](https://www.home-assistant.io/). Designed for humans and AI agents alike.

```
ha-client state list
ENTITY_ID                                STATE  LAST_UPDATED
light.desk                               on     2026-02-22 09:14:03 +0000 UTC
switch.fan                               off    2026-02-22 08:55:12 +0000 UTC
sensor.living_room_temperature           21.4   2026-02-22 09:13:58 +0000 UTC
```

## Install

**From source** (requires Go 1.25):

```bash
go install github.com/rnorth/ha-client@latest
```

**Build locally** (requires [mise](https://mise.jdx.dev/)):

```bash
mise run build        # produces ./ha-client
mise run install      # copies to /usr/local/bin/ha-client
```

## Configuration

### Login

The easiest way to configure credentials is with the interactive login command:

```bash
ha-client login
# Home Assistant server URL (e.g. http://homeassistant.local:8123): http://192.168.1.10:8123
# Long-lived access token: ••••••••••••••
# Credentials saved successfully.
```

Credentials are stored in the OS keychain (macOS Keychain, GNOME Keyring, Windows Credential Manager). On headless systems where no keychain is available, they fall back to `~/.config/ha-client/config.yaml`.

```bash
ha-client logout      # removes stored credentials
```

### Credential resolution order

Every command resolves credentials in this priority order:

| Priority | Source |
|----------|--------|
| 1 (highest) | `--server` / `--token` flags |
| 2 | `HASS_SERVER` / `HASS_TOKEN` environment variables |
| 3 | OS keychain |
| 4 (lowest) | `~/.config/ha-client/config.yaml` |

This makes `ha-client` easy to use in scripts and CI:

```bash
HASS_SERVER=http://ha.local:8123 HASS_TOKEN=... ha-client state list
# or per-command
ha-client --server http://ha.local:8123 --token ... state list
```

## Commands

### `info` — server information

```bash
ha-client info
```

Shows the Home Assistant version, location name, timezone, and unit system.

---

### `state` — entity states

```bash
ha-client state list
ha-client state list -o json          # machine-readable output
```

```bash
ha-client state get light.desk
```

```bash
ha-client state describe light.desk   # full state including all attributes (YAML)
```

```bash
ha-client state set light.desk on
ha-client state set light.desk on --attributes '{"brightness": 200}'
```

---

### `action` — call services

```bash
ha-client action list                 # lists all domain.action pairs
```

```bash
ha-client action call light.turn_on --data '{"entity_id": "light.desk"}'
ha-client action call light.turn_off --data '{"entity_id": "light.desk"}'
ha-client action call homeassistant.reload_config_entry
```

Actions are what Home Assistant calls "services". The format is `<domain>.<action>`.

---

### `event` — real-time event stream

```bash
ha-client event watch                             # all events
ha-client event watch --type state_changed        # filtered by type
ha-client event watch --type call_service | jq .  # pipe to jq
```

Streams events as newline-delimited JSON until Ctrl+C. Uses the WebSocket API.

---

### `area` — area registry

```bash
ha-client area list
ha-client area get living_room        # by area_id or name
ha-client area create "Garage"
ha-client area delete garage
```

---

### `device` — device registry

```bash
ha-client device list
ha-client device get abc123           # by device_id or name
ha-client device describe abc123      # full details (YAML)
```

The `list` output includes `ID`, `NAME`, `MANUFACTURER`, `MODEL`, and `AREA_ID`.

---

### `entity` — entity registry

```bash
ha-client entity list
ha-client entity list -o json | jq '.[] | select(.platform == "hue")'
ha-client entity get light.desk
ha-client entity describe light.desk  # full registry entry (YAML)
```

---

### `automation` — automations

```bash
ha-client automation list
ha-client automation get morning_routine          # prefix "automation." is optional
ha-client automation describe morning_routine     # full details (YAML)

ha-client automation trigger morning_routine
ha-client automation enable  morning_routine
ha-client automation disable morning_routine
```

```bash
ha-client automation export morning_routine          # Export automation config as YAML
ha-client automation export morning_routine -o json  # Export as JSON

ha-client automation apply -f automation.yaml            # Apply (create or update) automation
ha-client automation apply -f automation.yaml --dry-run  # Preview changes without applying
```

`export` works with UI-created automations that have a `unique_id` in storage. The exported YAML can be edited and re-applied with `apply`.

---

## Output formats

All commands support three output formats controlled by `-o` / `--output`:

| Flag | Format | Best for |
|------|--------|----------|
| _(default, TTY)_ | table | human reading at a terminal |
| _(default, pipe)_ | JSON | piping to `jq`, scripts, AI agents |
| `-o json` | JSON | explicit machine-readable output |
| `-o yaml` | YAML | config files, readability |
| `-o table` | table | force table even when piped |

**TTY auto-detection:** when stdout is a terminal, output is a formatted table. When piped or redirected, output is JSON automatically. This makes `ha-client` composable without needing explicit flags:

```bash
ha-client state list                         # → table (terminal)
ha-client state list | cat                   # → JSON (piped)
ha-client state list | jq '.[].entity_id'   # → entity IDs, one per line
```

`describe` subcommands always use YAML when at a terminal (better for nested attributes), and JSON when piped.

---

## Development

```bash
mise run build    # build ./ha-client
mise run test     # run all tests
mise run clean    # remove ./ha-client
mise run install  # build and copy to /usr/local/bin
```

Requires Go 1.25 and [mise](https://mise.jdx.dev/).

### Architecture

| Layer | Transport | Used for |
|-------|-----------|----------|
| REST (`/api/*`) | HTTP | states, actions, server info |
| WebSocket (`/api/websocket`) | WS | areas, devices, entity registry, event streaming |

Credential resolution, output rendering, and API transport are each isolated packages under `internal/` with full test coverage.
