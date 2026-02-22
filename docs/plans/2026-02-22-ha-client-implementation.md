# ha-client Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build `ha-client`, a single-binary Go CLI for Home Assistant with kubectl-style UX (`ha-client <resource> <verb> [name]`), usable by humans and AI agents alike.

**Architecture:** Cobra CLI with two API transports — a REST client for state/action/info and a WebSocket client for registry (area, device, entity) and real-time (event watch) operations. Credentials resolve through: CLI flags → env vars → OS keychain → config file. Auto-detects TTY to choose table vs JSON output.

**Tech Stack:** Go 1.25, Cobra v1, gorilla/websocket, go-keyring, tablewriter v2, yaml.v3, testify

---

## Task 1: Initialize Go module and project scaffold

**Files:**
- Create: `go.mod`
- Create: `main.go`
- Create: `cmd/root.go`
- Create: `.mise.toml`

**Step 1: Initialize the Go module**

```bash
cd /Users/rnorth/github.com/rnorth/ha-cli
go mod init github.com/rnorth/ha-cli
```

Expected: `go.mod` created with `module github.com/rnorth/ha-cli` and `go 1.25`

**Step 2: Install dependencies**

```bash
go get github.com/spf13/cobra@latest
go get github.com/zalando/go-keyring@latest
go get github.com/gorilla/websocket@latest
go get gopkg.in/yaml.v3@latest
go get github.com/olekukonko/tablewriter@latest
go get github.com/stretchr/testify@latest
```

**Step 3: Create `.mise.toml`**

```toml
[tools]
go = "1.25"
```

**Step 4: Create `cmd/root.go`**

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	outputFormat string
	serverFlag   string
	tokenFlag    string
)

var rootCmd = &cobra.Command{
	Use:   "ha-client",
	Short: "A kubectl-style CLI for Home Assistant",
	Long:  "Interact with Home Assistant instances from the command line. Designed for humans and AI agents.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "", "output format: table, json, yaml (default: auto-detect TTY)")
	rootCmd.PersistentFlags().StringVar(&serverFlag, "server", "", "HA server URL (overrides config/env)")
	rootCmd.PersistentFlags().StringVar(&tokenFlag, "token", "", "HA access token (overrides config/env)")
}
```

**Step 5: Create `main.go`**

```go
package main

import "github.com/rnorth/ha-cli/cmd"

func main() {
	cmd.Execute()
}
```

**Step 6: Build and verify**

```bash
go build -o ha-client .
./ha-client --help
```

Expected output: usage text with `ha-client` and `--output`, `--server`, `--token` flags listed.

**Step 7: Commit**

```bash
git add go.mod go.sum main.go cmd/root.go .mise.toml
git commit -m "feat: scaffold Go module and root Cobra command"
```

---

## Task 2: Config and credential resolution

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

**Step 1: Write failing tests**

Create `internal/config/config_test.go`:

```go
package config_test

import (
	"os"
	"testing"

	"github.com/rnorth/ha-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlagsTakePriority(t *testing.T) {
	t.Setenv("HASS_SERVER", "http://from-env:8123")
	t.Setenv("HASS_TOKEN", "env-token")

	cfg, err := config.Resolve("http://from-flags:8123", "flag-token")
	require.NoError(t, err)
	assert.Equal(t, "http://from-flags:8123", cfg.Server)
	assert.Equal(t, "flag-token", cfg.Token)
}

func TestEnvVarsFallback(t *testing.T) {
	t.Setenv("HASS_SERVER", "http://from-env:8123")
	t.Setenv("HASS_TOKEN", "env-token")

	cfg, err := config.Resolve("", "")
	require.NoError(t, err)
	assert.Equal(t, "http://from-env:8123", cfg.Server)
	assert.Equal(t, "env-token", cfg.Token)
}

func TestPartialFlagOverride(t *testing.T) {
	t.Setenv("HASS_SERVER", "http://from-env:8123")
	t.Setenv("HASS_TOKEN", "env-token")

	cfg, err := config.Resolve("http://override:8123", "")
	require.NoError(t, err)
	assert.Equal(t, "http://override:8123", cfg.Server)
	assert.Equal(t, "env-token", cfg.Token)
}

func TestConfigFileFallback(t *testing.T) {
	os.Unsetenv("HASS_SERVER")
	os.Unsetenv("HASS_TOKEN")

	dir := t.TempDir()
	cfgFile := dir + "/config.yaml"
	err := os.WriteFile(cfgFile, []byte("server: http://from-file:8123\ntoken: file-token\n"), 0600)
	require.NoError(t, err)

	cfg, err := config.ResolveWithFile("", "", cfgFile)
	require.NoError(t, err)
	assert.Equal(t, "http://from-file:8123", cfg.Server)
	assert.Equal(t, "file-token", cfg.Token)
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/config/...
```

Expected: compile error — `config` package does not exist yet.

**Step 3: Implement `internal/config/config.go`**

```go
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/zalando/go-keyring"
	"gopkg.in/yaml.v3"
)

const (
	keychainService = "ha-client"
	keychainServer  = "server"
	keychainToken   = "token"
)

type Config struct {
	Server string `yaml:"server"`
	Token  string `yaml:"token"`
}

// Resolve returns config using the full resolution chain:
// CLI flags > env vars > OS keychain > config file
func Resolve(serverFlag, tokenFlag string) (*Config, error) {
	return ResolveWithFile(serverFlag, tokenFlag, DefaultConfigPath())
}

func ResolveWithFile(serverFlag, tokenFlag, configFile string) (*Config, error) {
	cfg := &Config{}

	// Layer 4: config file (lowest priority)
	if fileCfg, err := loadFile(configFile); err == nil {
		cfg.Server = fileCfg.Server
		cfg.Token = fileCfg.Token
	}

	// Layer 3: OS keychain
	if server, err := keyring.Get(keychainService, keychainServer); err == nil && server != "" {
		cfg.Server = server
	}
	if token, err := keyring.Get(keychainService, keychainToken); err == nil && token != "" {
		cfg.Token = token
	}

	// Layer 2: environment variables
	if v := os.Getenv("HASS_SERVER"); v != "" {
		cfg.Server = v
	}
	if v := os.Getenv("HASS_TOKEN"); v != "" {
		cfg.Token = v
	}

	// Layer 1: CLI flags (highest priority)
	if serverFlag != "" {
		cfg.Server = serverFlag
	}
	if tokenFlag != "" {
		cfg.Token = tokenFlag
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	if c.Server == "" {
		return fmt.Errorf("no server configured: use 'ha-client login', set HASS_SERVER, or use --server")
	}
	if c.Token == "" {
		return fmt.Errorf("no token configured: use 'ha-client login', set HASS_TOKEN, or use --token")
	}
	return nil
}

// SaveToKeychain saves credentials to OS keychain, falling back to config file.
func SaveToKeychain(server, token string) error {
	if err := keyring.Set(keychainService, keychainServer, server); err != nil {
		// Keychain unavailable (headless/container) — fall back to file
		return SaveToFile(server, token, DefaultConfigPath())
	}
	return keyring.Set(keychainService, keychainToken, token)
}

// DeleteFromKeychain removes stored credentials.
func DeleteFromKeychain() error {
	errServer := keyring.Delete(keychainService, keychainServer)
	errToken := keyring.Delete(keychainService, keychainToken)
	// Also clear config file
	_ = os.Remove(DefaultConfigPath())
	if errServer != nil {
		return errServer
	}
	return errToken
}

func SaveToFile(server, token, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	cfg := Config{Server: server, Token: token}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func loadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func DefaultConfigPath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("APPDATA"), "ha-client", "config.yaml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "ha-client", "config.yaml")
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/config/... -v
```

Expected: all 4 tests PASS.

**Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat: add credential resolution (flags > env > keychain > file)"
```

---

## Task 3: Output renderer

**Files:**
- Create: `internal/output/output.go`
- Create: `internal/output/output_test.go`

**Step 1: Write failing tests**

Create `internal/output/output_test.go`:

```go
package output_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/rnorth/ha-cli/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type item struct {
	Name  string `json:"name" yaml:"name"`
	State string `json:"state" yaml:"state"`
}

func TestJSONOutput(t *testing.T) {
	var buf bytes.Buffer
	data := []item{{"light.desk", "on"}, {"switch.fan", "off"}}

	err := output.Render(&buf, output.FormatJSON, data, nil)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), `"name": "light.desk"`)
	assert.Contains(t, buf.String(), `"state": "on"`)
}

func TestYAMLOutput(t *testing.T) {
	var buf bytes.Buffer
	data := []item{{"light.desk", "on"}}

	err := output.Render(&buf, output.FormatYAML, data, nil)
	require.NoError(t, err)

	var out []item
	require.NoError(t, yaml.Unmarshal(buf.Bytes(), &out))
	assert.Equal(t, "light.desk", out[0].Name)
}

func TestAutoFormatNoTTY(t *testing.T) {
	// When stdout is not a TTY (like in tests/pipes), auto → JSON
	fmt := output.DetectFormat("", os.Stdout)
	// In test runner, stdout is not a TTY
	assert.Equal(t, output.FormatJSON, fmt)
}

func TestExplicitOverride(t *testing.T) {
	fmt := output.DetectFormat("yaml", os.Stdout)
	assert.Equal(t, output.FormatYAML, fmt)
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/output/...
```

Expected: compile error.

**Step 3: Implement `internal/output/output.go`**

```go
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"

	"github.com/olekukonko/tablewriter"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatYAML  Format = "yaml"
	FormatAuto  Format = "auto"
)

// DetectFormat resolves the output format. If override is set, use it.
// Otherwise, use table when stdout is a TTY, JSON when piped.
func DetectFormat(override string, stdout *os.File) Format {
	switch strings.ToLower(override) {
	case "table":
		return FormatTable
	case "json":
		return FormatJSON
	case "yaml":
		return FormatYAML
	}
	// Auto-detect
	if term.IsTerminal(int(stdout.Fd())) {
		return FormatTable
	}
	return FormatJSON
}

// Render writes data to w in the requested format.
// data must be a slice of structs or a single struct/map.
// columns is used only for table format; if nil, all exported fields are used.
func Render(w io.Writer, format Format, data interface{}, columns []string) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	case FormatYAML:
		return yaml.NewEncoder(w).Encode(data)
	case FormatTable:
		return renderTable(w, data, columns)
	default:
		return fmt.Errorf("unknown format: %s", format)
	}
}

func renderTable(w io.Writer, data interface{}, columns []string) error {
	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	table := tablewriter.NewWriter(w)
	table.SetBorder(false)
	table.SetHeaderLine(false)
	table.SetColumnSeparator("  ")
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)

	// Handle slice of structs
	if v.Kind() == reflect.Slice {
		if v.Len() == 0 {
			fmt.Fprintln(w, "(none)")
			return nil
		}
		elem := v.Index(0)
		if elem.Kind() == reflect.Ptr {
			elem = elem.Elem()
		}
		headers, fields := resolveColumns(elem.Type(), columns)
		table.SetHeader(headers)
		for i := 0; i < v.Len(); i++ {
			row := v.Index(i)
			if row.Kind() == reflect.Ptr {
				row = row.Elem()
			}
			table.Append(extractRow(row, fields))
		}
		table.Render()
		return nil
	}

	// Single struct
	if v.Kind() == reflect.Struct {
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			table.Append([]string{f.Name, fmt.Sprintf("%v", v.Field(i).Interface())})
		}
		table.Render()
		return nil
	}

	// Fallback
	fmt.Fprintln(w, data)
	return nil
}

func resolveColumns(t reflect.Type, override []string) (headers []string, fields []string) {
	if len(override) > 0 {
		return override, override
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		tag := f.Tag.Get("json")
		name := f.Name
		if tag != "" && tag != "-" {
			name = strings.Split(tag, ",")[0]
		}
		headers = append(headers, strings.ToUpper(name))
		fields = append(fields, f.Name)
	}
	return
}

func extractRow(v reflect.Value, fields []string) []string {
	row := make([]string, len(fields))
	t := v.Type()
	for i, fieldName := range fields {
		for j := 0; j < t.NumField(); j++ {
			if t.Field(j).Name == fieldName || strings.Split(t.Field(j).Tag.Get("json"), ",")[0] == fieldName {
				row[i] = fmt.Sprintf("%v", v.Field(j).Interface())
				break
			}
		}
	}
	return row
}
```

Note: `golang.org/x/term` is needed for TTY detection — add it:

```bash
go get golang.org/x/term
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/output/... -v
```

Expected: all 4 tests PASS.

**Step 5: Commit**

```bash
git add internal/output/ go.mod go.sum
git commit -m "feat: add output renderer (table/JSON/YAML with TTY auto-detect)"
```

---

## Task 4: REST API client

**Files:**
- Create: `internal/client/types.go`
- Create: `internal/client/rest.go`
- Create: `internal/client/rest_test.go`

**Step 1: Create `internal/client/types.go`**

```go
package client

import "time"

type HAInfo struct {
	Version         string `json:"version"`
	LocationName    string `json:"location_name"`
	Latitude        float64 `json:"latitude"`
	Longitude       float64 `json:"longitude"`
	Timezone        string `json:"time_zone"`
	UnitSystem      UnitSystem `json:"unit_system"`
}

type UnitSystem struct {
	Length      string `json:"length"`
	Temperature string `json:"temperature"`
}

type State struct {
	EntityID    string                 `json:"entity_id"`
	State       string                 `json:"state"`
	Attributes  map[string]interface{} `json:"attributes"`
	LastChanged time.Time              `json:"last_changed"`
	LastUpdated time.Time              `json:"last_updated"`
}

type ActionDomain struct {
	Domain   string                     `json:"domain"`
	Services map[string]ActionDetail    `json:"services"`
}

type ActionDetail struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Fields      map[string]interface{} `json:"fields"`
}
```

**Step 2: Write failing tests for `rest.go`**

Create `internal/client/rest_test.go`:

```go
package client_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rnorth/ha-cli/internal/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *client.RESTClient) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := client.NewRESTClient(srv.URL, "test-token")
	require.NoError(t, err)
	return srv, c
}

func TestGetInfo(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/config", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		json.NewEncoder(w).Encode(client.HAInfo{Version: "2024.1.0", LocationName: "Home"})
	})

	info, err := c.GetInfo()
	require.NoError(t, err)
	assert.Equal(t, "2024.1.0", info.Version)
	assert.Equal(t, "Home", info.LocationName)
}

func TestListStates(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/states", r.URL.Path)
		json.NewEncoder(w).Encode([]client.State{
			{EntityID: "light.desk", State: "on"},
			{EntityID: "switch.fan", State: "off"},
		})
	})

	states, err := c.ListStates()
	require.NoError(t, err)
	assert.Len(t, states, 2)
	assert.Equal(t, "light.desk", states[0].EntityID)
}

func TestGetState(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/states/light.desk", r.URL.Path)
		json.NewEncoder(w).Encode(client.State{EntityID: "light.desk", State: "on"})
	})

	state, err := c.GetState("light.desk")
	require.NoError(t, err)
	assert.Equal(t, "on", state.State)
}

func TestSetState(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/states/light.desk", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(client.State{EntityID: "light.desk", State: "off"})
	})

	state, err := c.SetState("light.desk", "off", nil)
	require.NoError(t, err)
	assert.Equal(t, "off", state.State)
}

func TestListActions(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/services", r.URL.Path)
		json.NewEncoder(w).Encode([]client.ActionDomain{{Domain: "light"}})
	})

	actions, err := c.ListActions()
	require.NoError(t, err)
	assert.Len(t, actions, 1)
	assert.Equal(t, "light", actions[0].Domain)
}

func TestCallAction(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/services/light/turn_on", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]client.State{})
	})

	err := c.CallAction("light", "turn_on", map[string]interface{}{"entity_id": "light.desk"})
	require.NoError(t, err)
}
```

**Step 3: Run tests to verify they fail**

```bash
go test ./internal/client/... 2>&1 | head -5
```

Expected: compile error — `RESTClient` not defined.

**Step 4: Implement `internal/client/rest.go`**

```go
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type RESTClient struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewRESTClient(serverURL, token string) (*RESTClient, error) {
	url := strings.TrimRight(serverURL, "/")
	return &RESTClient{
		baseURL: url,
		token:   token,
		http:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (c *RESTClient) get(path string, out interface{}) error {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("unauthorized: check your token")
	}
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("not found")
	}
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *RESTClient) post(path string, body interface{}, out interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+path, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("unauthorized: check your token")
	}
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func (c *RESTClient) GetInfo() (*HAInfo, error) {
	var info HAInfo
	return &info, c.get("/api/config", &info)
}

func (c *RESTClient) ListStates() ([]State, error) {
	var states []State
	return states, c.get("/api/states", &states)
}

func (c *RESTClient) GetState(entityID string) (*State, error) {
	var state State
	return &state, c.get("/api/states/"+entityID, &state)
}

func (c *RESTClient) SetState(entityID, state string, attributes map[string]interface{}) (*State, error) {
	body := map[string]interface{}{"state": state}
	if attributes != nil {
		body["attributes"] = attributes
	}
	var result State
	return &result, c.post("/api/states/"+entityID, body, &result)
}

func (c *RESTClient) ListActions() ([]ActionDomain, error) {
	var actions []ActionDomain
	return actions, c.get("/api/services", &actions)
}

func (c *RESTClient) CallAction(domain, action string, data map[string]interface{}) error {
	return c.post("/api/services/"+domain+"/"+action, data, nil)
}
```

**Step 5: Run tests to verify they pass**

```bash
go test ./internal/client/... -v -run TestGetInfo -run TestListStates -run TestGetState -run TestSetState -run TestListActions -run TestCallAction
```

Expected: all REST tests PASS.

**Step 6: Commit**

```bash
git add internal/client/
git commit -m "feat: add REST API client for state, action, and info operations"
```

---

## Task 5: WebSocket API client

**Files:**
- Create: `internal/client/ws.go`
- Create: `internal/client/ws_test.go`
- Add types to `internal/client/types.go`

**Step 1: Add registry types to `internal/client/types.go`**

Append to `types.go`:

```go
type Area struct {
	AreaID  string `json:"area_id" yaml:"area_id"`
	Name    string `json:"name" yaml:"name"`
	Picture string `json:"picture,omitempty" yaml:"picture,omitempty"`
}

type Device struct {
	ID           string `json:"id" yaml:"id"`
	Name         string `json:"name" yaml:"name"`
	AreaID       string `json:"area_id,omitempty" yaml:"area_id,omitempty"`
	Manufacturer string `json:"manufacturer,omitempty" yaml:"manufacturer,omitempty"`
	Model        string `json:"model,omitempty" yaml:"model,omitempty"`
	ConfigEntries []string `json:"config_entries,omitempty" yaml:"config_entries,omitempty"`
}

type EntityEntry struct {
	EntityID  string `json:"entity_id" yaml:"entity_id"`
	Name      string `json:"name,omitempty" yaml:"name,omitempty"`
	AreaID    string `json:"area_id,omitempty" yaml:"area_id,omitempty"`
	DeviceID  string `json:"device_id,omitempty" yaml:"device_id,omitempty"`
	Platform  string `json:"platform,omitempty" yaml:"platform,omitempty"`
	Disabled  bool   `json:"disabled_by,omitempty" yaml:"disabled,omitempty"`
}

type WSMessage struct {
	ID      int             `json:"id,omitempty"`
	Type    string          `json:"type"`
	Success bool            `json:"success,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Event   json.RawMessage `json:"event,omitempty"`
	Error   *WSError        `json:"error,omitempty"`
}

type WSError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
```

**Step 2: Write failing tests for `ws.go`**

Create `internal/client/ws_test.go`:

```go
package client_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/rnorth/ha-cli/internal/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

// mockWSServer sets up a test WebSocket server that handles HA auth + one command response.
func mockWSServer(t *testing.T, token string, cmdType string, response interface{}) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Auth required
		conn.WriteJSON(map[string]string{"type": "auth_required", "ha_version": "2024.1"})

		// Read auth message
		var authMsg map[string]string
		conn.ReadJSON(&authMsg)
		if authMsg["access_token"] != token {
			conn.WriteJSON(map[string]string{"type": "auth_invalid"})
			return
		}
		conn.WriteJSON(map[string]string{"type": "auth_ok"})

		// Read command
		var cmd client.WSMessage
		conn.ReadJSON(&cmd)
		// Check the command type matches (strip id)
		// Respond with result
		resultData, _ := json.Marshal(response)
		conn.WriteJSON(map[string]interface{}{
			"id":      cmd.ID,
			"type":    "result",
			"success": true,
			"result":  json.RawMessage(resultData),
		})
	}))
	return srv
}

func wsURL(srv *httptest.Server) string {
	return "ws" + strings.TrimPrefix(srv.URL, "http")
}

func TestListAreas(t *testing.T) {
	areas := []client.Area{{AreaID: "living_room", Name: "Living Room"}}
	srv := mockWSServer(t, "test-token", "config/area_registry/list", areas)
	defer srv.Close()

	wsc, err := client.NewWSClient(wsURL(srv), "test-token")
	require.NoError(t, err)
	defer wsc.Close()

	result, err := wsc.ListAreas()
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "Living Room", result[0].Name)
}

func TestListDevices(t *testing.T) {
	devices := []client.Device{{ID: "abc123", Name: "Desk Lamp"}}
	srv := mockWSServer(t, "test-token", "config/device_registry/list", devices)
	defer srv.Close()

	wsc, err := client.NewWSClient(wsURL(srv), "test-token")
	require.NoError(t, err)
	defer wsc.Close()

	result, err := wsc.ListDevices()
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "Desk Lamp", result[0].Name)
}

func TestListEntities(t *testing.T) {
	entities := []client.EntityEntry{{EntityID: "light.desk", Platform: "hue"}}
	srv := mockWSServer(t, "test-token", "config/entity_registry/list", entities)
	defer srv.Close()

	wsc, err := client.NewWSClient(wsURL(srv), "test-token")
	require.NoError(t, err)
	defer wsc.Close()

	result, err := wsc.ListEntities()
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "light.desk", result[0].EntityID)
}
```

**Step 3: Run tests to verify they fail**

```bash
go test ./internal/client/... -run TestListAreas -run TestListDevices -run TestListEntities 2>&1 | head -5
```

Expected: compile error — `WSClient` not defined.

**Step 4: Implement `internal/client/ws.go`**

```go
package client

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
)

type WSClient struct {
	conn    *websocket.Conn
	mu      sync.Mutex
	counter atomic.Int32
}

func NewWSClient(serverURL, token string) (*WSClient, error) {
	// Convert http:// → ws://, https:// → wss://
	wsURL := serverURL
	if len(wsURL) > 4 && wsURL[:5] == "http:" {
		wsURL = "ws:" + wsURL[5:]
	} else if len(wsURL) > 5 && wsURL[:6] == "https:" {
		wsURL = "wss:" + wsURL[6:]
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL+"/api/websocket", nil)
	if err != nil {
		return nil, fmt.Errorf("websocket connect failed: %w", err)
	}

	// Auth flow
	var authRequired WSMessage
	if err := conn.ReadJSON(&authRequired); err != nil {
		conn.Close()
		return nil, fmt.Errorf("read auth_required: %w", err)
	}

	if err := conn.WriteJSON(map[string]string{"type": "auth", "access_token": token}); err != nil {
		conn.Close()
		return nil, err
	}

	var authResult WSMessage
	if err := conn.ReadJSON(&authResult); err != nil {
		conn.Close()
		return nil, err
	}
	if authResult.Type != "auth_ok" {
		conn.Close()
		return nil, fmt.Errorf("authentication failed: invalid token")
	}

	return &WSClient{conn: conn}, nil
}

func (c *WSClient) Close() error {
	return c.conn.Close()
}

func (c *WSClient) send(msgType string, extra map[string]interface{}) (*WSMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := int(c.counter.Add(1))
	msg := map[string]interface{}{"id": id, "type": msgType}
	for k, v := range extra {
		msg[k] = v
	}

	if err := c.conn.WriteJSON(msg); err != nil {
		return nil, fmt.Errorf("send %s: %w", msgType, err)
	}

	var resp WSMessage
	if err := c.conn.ReadJSON(&resp); err != nil {
		return nil, fmt.Errorf("read response for %s: %w", msgType, err)
	}
	if !resp.Success {
		if resp.Error != nil {
			return nil, fmt.Errorf("WS error %s: %s", resp.Error.Code, resp.Error.Message)
		}
		return nil, fmt.Errorf("command %s failed", msgType)
	}
	return &resp, nil
}

func (c *WSClient) ListAreas() ([]Area, error) {
	resp, err := c.send("config/area_registry/list", nil)
	if err != nil {
		return nil, err
	}
	var areas []Area
	return areas, json.Unmarshal(resp.Result, &areas)
}

func (c *WSClient) CreateArea(name string) (*Area, error) {
	resp, err := c.send("config/area_registry/create", map[string]interface{}{"name": name})
	if err != nil {
		return nil, err
	}
	var area Area
	return &area, json.Unmarshal(resp.Result, &area)
}

func (c *WSClient) DeleteArea(areaID string) error {
	_, err := c.send("config/area_registry/delete", map[string]interface{}{"area_id": areaID})
	return err
}

func (c *WSClient) ListDevices() ([]Device, error) {
	resp, err := c.send("config/device_registry/list", nil)
	if err != nil {
		return nil, err
	}
	var devices []Device
	return devices, json.Unmarshal(resp.Result, &devices)
}

func (c *WSClient) ListEntities() ([]EntityEntry, error) {
	resp, err := c.send("config/entity_registry/list", nil)
	if err != nil {
		return nil, err
	}
	var entities []EntityEntry
	return entities, json.Unmarshal(resp.Result, &entities)
}

func (c *WSClient) GetEntity(entityID string) (*EntityEntry, error) {
	resp, err := c.send("config/entity_registry/get", map[string]interface{}{"entity_id": entityID})
	if err != nil {
		return nil, err
	}
	var entity EntityEntry
	return &entity, json.Unmarshal(resp.Result, &entity)
}

// SubscribeEvents subscribes to events and calls handler for each event received.
// Blocks until handler returns false or an error occurs.
func (c *WSClient) SubscribeEvents(eventType string, handler func(json.RawMessage) bool) error {
	c.mu.Lock()
	id := int(c.counter.Add(1))
	msg := map[string]interface{}{"id": id, "type": "subscribe_events"}
	if eventType != "" {
		msg["event_type"] = eventType
	}
	if err := c.conn.WriteJSON(msg); err != nil {
		c.mu.Unlock()
		return err
	}

	// Read subscription confirmation
	var ack WSMessage
	if err := c.conn.ReadJSON(&ack); err != nil {
		c.mu.Unlock()
		return err
	}
	c.mu.Unlock()

	// Stream events
	for {
		var event WSMessage
		if err := c.conn.ReadJSON(&event); err != nil {
			return err
		}
		if event.Type == "event" {
			if !handler(event.Event) {
				return nil
			}
		}
	}
}
```

**Step 5: Run tests to verify they pass**

```bash
go test ./internal/client/... -v
```

Expected: all tests PASS (REST and WS).

**Step 6: Commit**

```bash
git add internal/client/
git commit -m "feat: add WebSocket API client for registry and event operations"
```

---

## Task 6: login / logout / info commands

**Files:**
- Create: `cmd/login.go`
- Create: `cmd/info.go`
- Modify: `cmd/root.go`

**Step 1: Add a helper to `cmd/root.go` for resolving config**

Append to `cmd/root.go`:

```go
import (
	"github.com/rnorth/ha-cli/internal/config"
	"github.com/rnorth/ha-cli/internal/client"
	"github.com/rnorth/ha-cli/internal/output"
	"os"
)

func resolveConfig() (*config.Config, error) {
	cfg, err := config.Resolve(serverFlag, tokenFlag)
	if err != nil {
		return nil, err
	}
	return cfg, cfg.Validate()
}

func resolveFormat() output.Format {
	return output.DetectFormat(outputFormat, os.Stdout)
}
```

**Step 2: Create `cmd/login.go`**

```go
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/rnorth/ha-cli/internal/client"
	"github.com/rnorth/ha-cli/internal/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Store Home Assistant credentials",
	Long:  "Prompts for server URL and long-lived access token and stores them securely.",
	RunE: func(cmd *cobra.Command, args []string) error {
		reader := bufio.NewReader(os.Stdin)

		fmt.Print("Home Assistant server URL (e.g. http://homeassistant.local:8123): ")
		server, _ := reader.ReadString('\n')
		server = strings.TrimSpace(server)

		fmt.Print("Long-lived access token: ")
		tokenBytes, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			// Fallback for non-TTY (e.g. piped input in tests)
			tokenBytes, _ = reader.ReadBytes('\n')
		}
		token := strings.TrimSpace(string(tokenBytes))

		// Verify credentials work
		c, err := client.NewRESTClient(server, token)
		if err != nil {
			return fmt.Errorf("invalid server URL: %w", err)
		}
		if _, err := c.GetInfo(); err != nil {
			return fmt.Errorf("could not connect to Home Assistant: %w", err)
		}

		if err := config.SaveToKeychain(server, token); err != nil {
			return fmt.Errorf("failed to save credentials: %w", err)
		}
		fmt.Println("Credentials saved successfully.")
		return nil
	},
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove stored Home Assistant credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.DeleteFromKeychain(); err != nil {
			return fmt.Errorf("failed to remove credentials: %w", err)
		}
		fmt.Println("Credentials removed.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
}
```

**Step 3: Create `cmd/info.go`**

```go
package cmd

import (
	"os"

	"github.com/rnorth/ha-cli/internal/client"
	"github.com/rnorth/ha-cli/internal/output"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show Home Assistant server information",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := resolveConfig()
		if err != nil {
			return err
		}
		c, err := client.NewRESTClient(cfg.Server, cfg.Token)
		if err != nil {
			return err
		}
		info, err := c.GetInfo()
		if err != nil {
			return err
		}
		return output.Render(os.Stdout, resolveFormat(), info, nil)
	},
}

func init() {
	rootCmd.AddCommand(infoCmd)
}
```

**Step 4: Build and manual smoke test**

```bash
go build -o ha-client .
./ha-client --help
./ha-client info --help
```

Expected: `info` command appears in help output.

**Step 5: Commit**

```bash
git add cmd/login.go cmd/info.go cmd/root.go go.mod go.sum
git commit -m "feat: add login, logout, and info commands"
```

---

## Task 7: state commands

**Files:**
- Create: `cmd/state.go`

**Step 1: Create `cmd/state.go`**

```go
package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/rnorth/ha-cli/internal/client"
	"github.com/rnorth/ha-cli/internal/output"
	"github.com/spf13/cobra"
)

var stateCmd = &cobra.Command{
	Use:   "state",
	Short: "Manage entity states",
}

var stateListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all entity states",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := resolveConfig()
		if err != nil {
			return err
		}
		c, err := client.NewRESTClient(cfg.Server, cfg.Token)
		if err != nil {
			return err
		}
		states, err := c.ListStates()
		if err != nil {
			return err
		}
		return output.Render(os.Stdout, resolveFormat(), states, []string{"EntityID", "State", "LastUpdated"})
	},
}

var stateGetCmd = &cobra.Command{
	Use:   "get <entity_id>",
	Short: "Get state of an entity",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := resolveConfig()
		if err != nil {
			return err
		}
		c, err := client.NewRESTClient(cfg.Server, cfg.Token)
		if err != nil {
			return err
		}
		state, err := c.GetState(args[0])
		if err != nil {
			return err
		}
		return output.Render(os.Stdout, resolveFormat(), state, nil)
	},
}

var stateDescribeCmd = &cobra.Command{
	Use:   "describe <entity_id>",
	Short: "Show full state and attributes of an entity",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := resolveConfig()
		if err != nil {
			return err
		}
		c, err := client.NewRESTClient(cfg.Server, cfg.Token)
		if err != nil {
			return err
		}
		state, err := c.GetState(args[0])
		if err != nil {
			return err
		}
		// Always render describe as JSON/YAML (attributes map doesn't render well in table)
		format := resolveFormat()
		if format == output.FormatTable {
			format = output.FormatYAML
		}
		return output.Render(os.Stdout, format, state, nil)
	},
}

var stateSetCmd = &cobra.Command{
	Use:   "set <entity_id> <state>",
	Short: "Set the state of an entity",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := resolveConfig()
		if err != nil {
			return err
		}
		c, err := client.NewRESTClient(cfg.Server, cfg.Token)
		if err != nil {
			return err
		}
		var attrs map[string]interface{}
		if attrJSON != "" {
			if err := json.Unmarshal([]byte(attrJSON), &attrs); err != nil {
				return fmt.Errorf("invalid --attributes JSON: %w", err)
			}
		}
		state, err := c.SetState(args[0], args[1], attrs)
		if err != nil {
			return err
		}
		return output.Render(os.Stdout, resolveFormat(), state, nil)
	},
}

var attrJSON string

func init() {
	stateSetCmd.Flags().StringVar(&attrJSON, "attributes", "", "JSON attributes to set alongside the state")
	stateCmd.AddCommand(stateListCmd, stateGetCmd, stateDescribeCmd, stateSetCmd)
	rootCmd.AddCommand(stateCmd)
}
```

**Step 2: Build and verify subcommands appear**

```bash
go build -o ha-client . && ./ha-client state --help
```

Expected: `list`, `get`, `describe`, `set` subcommands listed.

**Step 3: Commit**

```bash
git add cmd/state.go
git commit -m "feat: add state list/get/describe/set commands"
```

---

## Task 8: action commands

**Files:**
- Create: `cmd/action.go`

**Step 1: Create `cmd/action.go`**

```go
package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/rnorth/ha-cli/internal/client"
	"github.com/rnorth/ha-cli/internal/output"
	"github.com/spf13/cobra"
)

var actionCmd = &cobra.Command{
	Use:   "action",
	Short: "Manage Home Assistant actions (formerly services)",
}

var actionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available actions",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := resolveConfig()
		if err != nil {
			return err
		}
		c, err := client.NewRESTClient(cfg.Server, cfg.Token)
		if err != nil {
			return err
		}
		domains, err := c.ListActions()
		if err != nil {
			return err
		}
		// Flatten to a list of "domain.action" rows for table display
		type row struct {
			Action      string `json:"action" yaml:"action"`
			Description string `json:"description" yaml:"description"`
		}
		var rows []row
		for _, d := range domains {
			for name, detail := range d.Services {
				rows = append(rows, row{
					Action:      d.Domain + "." + name,
					Description: detail.Description,
				})
			}
		}
		return output.Render(os.Stdout, resolveFormat(), rows, nil)
	},
}

var actionDataJSON string

var actionCallCmd = &cobra.Command{
	Use:   "call <domain.action>",
	Short: "Call a Home Assistant action",
	Long:  "Call a Home Assistant action. Example: ha-client action call light.turn_on --data '{\"entity_id\": \"light.desk\"}'",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := resolveConfig()
		if err != nil {
			return err
		}
		c, err := client.NewRESTClient(cfg.Server, cfg.Token)
		if err != nil {
			return err
		}

		// Parse "domain.action"
		parts := splitDomainAction(args[0])
		if parts == nil {
			return fmt.Errorf("invalid action format %q: expected domain.action (e.g. light.turn_on)", args[0])
		}

		var data map[string]interface{}
		if actionDataJSON != "" {
			if err := json.Unmarshal([]byte(actionDataJSON), &data); err != nil {
				return fmt.Errorf("invalid --data JSON: %w", err)
			}
		}

		if err := c.CallAction(parts[0], parts[1], data); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "Action called successfully.")
		return nil
	},
}

func splitDomainAction(s string) []string {
	for i, c := range s {
		if c == '.' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return nil
}

func init() {
	actionCallCmd.Flags().StringVar(&actionDataJSON, "data", "", "JSON data to pass to the action")
	actionCmd.AddCommand(actionListCmd, actionCallCmd)
	rootCmd.AddCommand(actionCmd)
}
```

**Step 2: Build and verify**

```bash
go build -o ha-client . && ./ha-client action --help
```

Expected: `list` and `call` subcommands appear.

**Step 3: Commit**

```bash
git add cmd/action.go
git commit -m "feat: add action list/call commands"
```

---

## Task 9: event watch command

**Files:**
- Create: `cmd/event.go`

**Step 1: Create `cmd/event.go`**

```go
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rnorth/ha-cli/internal/client"
	"github.com/spf13/cobra"
)

var eventCmd = &cobra.Command{
	Use:   "event",
	Short: "Subscribe to Home Assistant events",
}

var eventTypeFilter string

var eventWatchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Stream events in real-time (Ctrl+C to stop)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := resolveConfig()
		if err != nil {
			return err
		}

		// WebSocket URL: replace http(s) with ws(s)
		wsURL := cfg.Server

		wsc, err := client.NewWSClient(wsURL, cfg.Token)
		if err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
		defer wsc.Close()

		// Handle Ctrl+C gracefully
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
		done := make(chan error, 1)

		go func() {
			done <- wsc.SubscribeEvents(eventTypeFilter, func(event json.RawMessage) bool {
				select {
				case <-stop:
					return false
				default:
				}
				var pretty map[string]interface{}
				if json.Unmarshal(event, &pretty) == nil {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					enc.Encode(pretty)
				}
				return true
			})
		}()

		select {
		case <-stop:
			fmt.Fprintln(os.Stderr, "\nStopped.")
		case err := <-done:
			if err != nil {
				return err
			}
		}
		return nil
	},
}

func init() {
	eventWatchCmd.Flags().StringVar(&eventTypeFilter, "type", "", "filter to a specific event type (e.g. state_changed)")
	eventCmd.AddCommand(eventWatchCmd)
	rootCmd.AddCommand(eventCmd)
}
```

**Step 2: Build and verify**

```bash
go build -o ha-client . && ./ha-client event watch --help
```

Expected: `watch` subcommand with `--type` flag.

**Step 3: Commit**

```bash
git add cmd/event.go
git commit -m "feat: add event watch command (WebSocket SSE)"
```

---

## Task 10: area, device, entity commands

**Files:**
- Create: `cmd/area.go`
- Create: `cmd/device.go`
- Create: `cmd/entity.go`

All three follow the same pattern: connect via WebSocket, call the appropriate method, render output.

**Step 1: Create `cmd/area.go`**

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/rnorth/ha-cli/internal/client"
	"github.com/rnorth/ha-cli/internal/output"
	"github.com/spf13/cobra"
)

var areaCmd = &cobra.Command{Use: "area", Short: "Manage Home Assistant areas"}

var areaListCmd = &cobra.Command{
	Use: "list", Short: "List all areas",
	RunE: func(cmd *cobra.Command, args []string) error {
		wsc, err := newWSClient()
		if err != nil {
			return err
		}
		defer wsc.Close()
		areas, err := wsc.ListAreas()
		if err != nil {
			return err
		}
		return output.Render(os.Stdout, resolveFormat(), areas, nil)
	},
}

var areaGetCmd = &cobra.Command{
	Use: "get <area_id>", Short: "Get a specific area by ID",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		wsc, err := newWSClient()
		if err != nil {
			return err
		}
		defer wsc.Close()
		areas, err := wsc.ListAreas()
		if err != nil {
			return err
		}
		for _, a := range areas {
			if a.AreaID == args[0] || a.Name == args[0] {
				return output.Render(os.Stdout, resolveFormat(), a, nil)
			}
		}
		return fmt.Errorf("area %q not found", args[0])
	},
}

var areaCreateCmd = &cobra.Command{
	Use: "create <name>", Short: "Create a new area",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		wsc, err := newWSClient()
		if err != nil {
			return err
		}
		defer wsc.Close()
		area, err := wsc.CreateArea(args[0])
		if err != nil {
			return err
		}
		return output.Render(os.Stdout, resolveFormat(), area, nil)
	},
}

var areaDeleteCmd = &cobra.Command{
	Use: "delete <area_id>", Short: "Delete an area",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		wsc, err := newWSClient()
		if err != nil {
			return err
		}
		defer wsc.Close()
		if err := wsc.DeleteArea(args[0]); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "Area deleted.")
		return nil
	},
}

func init() {
	areaCmd.AddCommand(areaListCmd, areaGetCmd, areaCreateCmd, areaDeleteCmd)
	rootCmd.AddCommand(areaCmd)
}
```

**Step 2: Add `newWSClient()` helper to `cmd/root.go`**

Append to `cmd/root.go`:

```go
func newWSClient() (*client.WSClient, error) {
	cfg, err := resolveConfig()
	if err != nil {
		return nil, err
	}
	return client.NewWSClient(cfg.Server, cfg.Token)
}
```

**Step 3: Create `cmd/device.go`**

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/rnorth/ha-cli/internal/output"
	"github.com/spf13/cobra"
)

var deviceCmd = &cobra.Command{Use: "device", Short: "Manage Home Assistant devices"}

var deviceListCmd = &cobra.Command{
	Use: "list", Short: "List all devices",
	RunE: func(cmd *cobra.Command, args []string) error {
		wsc, err := newWSClient()
		if err != nil {
			return err
		}
		defer wsc.Close()
		devices, err := wsc.ListDevices()
		if err != nil {
			return err
		}
		return output.Render(os.Stdout, resolveFormat(), devices, []string{"ID", "Name", "Manufacturer", "Model", "AreaID"})
	},
}

var deviceGetCmd = &cobra.Command{
	Use: "get <device_id>", Short: "Get a device by ID",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		wsc, err := newWSClient()
		if err != nil {
			return err
		}
		defer wsc.Close()
		devices, err := wsc.ListDevices()
		if err != nil {
			return err
		}
		for _, d := range devices {
			if d.ID == args[0] || d.Name == args[0] {
				return output.Render(os.Stdout, resolveFormat(), d, nil)
			}
		}
		return fmt.Errorf("device %q not found", args[0])
	},
}

var deviceDescribeCmd = &cobra.Command{
	Use: "describe <device_id>", Short: "Show full device details",
	Args: cobra.ExactArgs(1),
	RunE: deviceGetCmd.RunE, // same implementation, output renderer handles detail level
}

func init() {
	deviceCmd.AddCommand(deviceListCmd, deviceGetCmd, deviceDescribeCmd)
	rootCmd.AddCommand(deviceCmd)
}
```

**Step 4: Create `cmd/entity.go`**

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/rnorth/ha-cli/internal/output"
	"github.com/spf13/cobra"
)

var entityCmd = &cobra.Command{Use: "entity", Short: "Manage the entity registry"}

var entityListCmd = &cobra.Command{
	Use: "list", Short: "List all registered entities",
	RunE: func(cmd *cobra.Command, args []string) error {
		wsc, err := newWSClient()
		if err != nil {
			return err
		}
		defer wsc.Close()
		entities, err := wsc.ListEntities()
		if err != nil {
			return err
		}
		return output.Render(os.Stdout, resolveFormat(), entities, nil)
	},
}

var entityGetCmd = &cobra.Command{
	Use: "get <entity_id>", Short: "Get an entity from the registry",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		wsc, err := newWSClient()
		if err != nil {
			return err
		}
		defer wsc.Close()
		entity, err := wsc.GetEntity(args[0])
		if err != nil {
			return err
		}
		return output.Render(os.Stdout, resolveFormat(), entity, nil)
	},
}

var entityDescribeCmd = &cobra.Command{
	Use: "describe <entity_id>", Short: "Show full entity registry details",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		wsc, err := newWSClient()
		if err != nil {
			return err
		}
		defer wsc.Close()
		entity, err := wsc.GetEntity(args[0])
		if err != nil {
			return err
		}
		format := resolveFormat()
		if format == output.FormatTable {
			format = output.FormatYAML
		}
		return fmt.Errorf("%w", output.Render(os.Stdout, format, entity, nil))
	},
}

func init() {
	entityCmd.AddCommand(entityListCmd, entityGetCmd, entityDescribeCmd)
	rootCmd.AddCommand(entityCmd)
}
```

**Step 5: Build and verify all three command groups**

```bash
go build -o ha-client . && ./ha-client area --help && ./ha-client device --help && ./ha-client entity --help
```

Expected: each shows its subcommands.

**Step 6: Commit**

```bash
git add cmd/area.go cmd/device.go cmd/entity.go cmd/root.go
git commit -m "feat: add area, device, and entity commands (WebSocket registry)"
```

---

## Task 11: automation commands

**Files:**
- Create: `cmd/automation.go`

Automations are implemented via REST (states + actions). The automation state entity carries the name, last_triggered, and friendly_name in attributes.

**Step 1: Create `cmd/automation.go`**

```go
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/rnorth/ha-cli/internal/client"
	"github.com/rnorth/ha-cli/internal/output"
	"github.com/spf13/cobra"
)

var automationCmd = &cobra.Command{Use: "automation", Short: "Manage Home Assistant automations"}

var automationListCmd = &cobra.Command{
	Use: "list", Short: "List all automations",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := resolveConfig()
		if err != nil {
			return err
		}
		c, err := client.NewRESTClient(cfg.Server, cfg.Token)
		if err != nil {
			return err
		}
		states, err := c.ListStates()
		if err != nil {
			return err
		}
		type row struct {
			EntityID     string `json:"entity_id" yaml:"entity_id"`
			FriendlyName string `json:"friendly_name" yaml:"friendly_name"`
			State        string `json:"state" yaml:"state"`
		}
		var rows []row
		for _, s := range states {
			if !strings.HasPrefix(s.EntityID, "automation.") {
				continue
			}
			name, _ := s.Attributes["friendly_name"].(string)
			rows = append(rows, row{EntityID: s.EntityID, FriendlyName: name, State: s.State})
		}
		return output.Render(os.Stdout, resolveFormat(), rows, nil)
	},
}

var automationGetCmd = &cobra.Command{
	Use: "get <entity_id>", Short: "Get automation state",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := resolveConfig()
		if err != nil {
			return err
		}
		c, err := client.NewRESTClient(cfg.Server, cfg.Token)
		if err != nil {
			return err
		}
		id := args[0]
		if !strings.HasPrefix(id, "automation.") {
			id = "automation." + id
		}
		state, err := c.GetState(id)
		if err != nil {
			return err
		}
		return output.Render(os.Stdout, resolveFormat(), state, nil)
	},
}

var automationDescribeCmd = &cobra.Command{
	Use: "describe <entity_id>", Short: "Show full automation details including attributes",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := resolveConfig()
		if err != nil {
			return err
		}
		c, err := client.NewRESTClient(cfg.Server, cfg.Token)
		if err != nil {
			return err
		}
		id := args[0]
		if !strings.HasPrefix(id, "automation.") {
			id = "automation." + id
		}
		state, err := c.GetState(id)
		if err != nil {
			return err
		}
		format := resolveFormat()
		if format == output.FormatTable {
			format = output.FormatYAML
		}
		return output.Render(os.Stdout, format, state, nil)
	},
}

func automationAction(action string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		cfg, err := resolveConfig()
		if err != nil {
			return err
		}
		c, err := client.NewRESTClient(cfg.Server, cfg.Token)
		if err != nil {
			return err
		}
		id := args[0]
		if !strings.HasPrefix(id, "automation.") {
			id = "automation." + id
		}
		if err := c.CallAction("automation", action, map[string]interface{}{"entity_id": id}); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "automation.%s called for %s\n", action, id)
		return nil
	}
}

func init() {
	automationCmd.AddCommand(
		automationListCmd,
		automationGetCmd,
		automationDescribeCmd,
		&cobra.Command{Use: "trigger <entity_id>", Short: "Trigger an automation", Args: cobra.ExactArgs(1), RunE: automationAction("trigger")},
		&cobra.Command{Use: "enable <entity_id>", Short: "Enable an automation", Args: cobra.ExactArgs(1), RunE: automationAction("turn_on")},
		&cobra.Command{Use: "disable <entity_id>", Short: "Disable an automation", Args: cobra.ExactArgs(1), RunE: automationAction("turn_off")},
	)
	rootCmd.AddCommand(automationCmd)
}
```

**Step 2: Build and verify**

```bash
go build -o ha-client . && ./ha-client automation --help
```

Expected: `list`, `get`, `describe`, `trigger`, `enable`, `disable` all shown.

**Step 3: Final build test — all commands**

```bash
./ha-client --help
```

Expected: all resource groups (`state`, `action`, `event`, `area`, `device`, `entity`, `automation`) visible plus `login`, `logout`, `info`.

**Step 4: Run all tests**

```bash
go test ./... -v
```

Expected: all tests PASS, no compile errors.

**Step 5: Commit**

```bash
git add cmd/automation.go
git commit -m "feat: add automation list/get/describe/trigger/enable/disable commands"
```

---

## Task 12: Final wiring, error polish, and release build

**Files:**
- Modify: `cmd/root.go` (version flag)
- Create: `Makefile`

**Step 1: Add version flag to `cmd/root.go`**

Add to `root.go` init():

```go
rootCmd.Version = "0.1.0"
```

This adds `--version` / `-v` automatically via Cobra.

**Step 2: Create `Makefile`**

```makefile
BINARY := ha-client
VERSION := 0.1.0

.PHONY: build test clean install

build:
	go build -ldflags="-s -w -X main.version=$(VERSION)" -o $(BINARY) .

test:
	go test ./... -v

clean:
	rm -f $(BINARY)

install: build
	cp $(BINARY) /usr/local/bin/$(BINARY)
```

**Step 3: Build release binary**

```bash
make build
ls -lh ha-client
```

Expected: binary size ~10-15 MB (Go + deps, no debug symbols).

**Step 4: Run final tests**

```bash
make test
```

Expected: all tests PASS.

**Step 5: Final commit**

```bash
git add Makefile cmd/root.go
git commit -m "feat: add version flag and Makefile for build/test/install"
```

---

## Summary

| Task | What gets built |
|------|----------------|
| 1 | Go module, root Cobra command, binary scaffold |
| 2 | Credential resolution (flags > env > keychain > file) |
| 3 | Output renderer (table/JSON/YAML, TTY auto-detect) |
| 4 | REST client (states, actions, info) |
| 5 | WebSocket client (areas, devices, entities, events) |
| 6 | `login`, `logout`, `info` commands |
| 7 | `state list/get/describe/set` |
| 8 | `action list/call` |
| 9 | `event watch` |
| 10 | `area`, `device`, `entity` commands |
| 11 | `automation list/get/describe/trigger/enable/disable` |
| 12 | Version flag, Makefile, release build |
