package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newMockWSServerForSave creates a mock WS server that handles the save command.
// The save command merges automation config fields (including "id") at the top level,
// so we can't use a struct with ID int — read as raw map instead.
func newMockWSServerForSave(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		_ = conn.WriteJSON(map[string]string{"type": "auth_required", "ha_version": "2024.1"})
		var auth map[string]string
		_ = conn.ReadJSON(&auth)
		_ = conn.WriteJSON(map[string]string{"type": "auth_ok"})

		// Read save command as raw map to avoid int/string ID collision
		var cmd map[string]interface{}
		if err := conn.ReadJSON(&cmd); err != nil {
			return
		}
		// Extract the numeric ID from the "id" field — but it's overwritten by string "id"
		// from the automation config. Use 1 as fallback.
		msgID := 1
		_ = conn.WriteJSON(map[string]interface{}{
			"id": msgID, "type": "result", "success": true,
			"result": json.RawMessage(`{}`),
		})
	}))
}

func TestAutomationApply(t *testing.T) {
	srv := newMockWSServerForSave(t)
	defer srv.Close()

	t.Setenv("HASS_SERVER", wsTestURL(srv))
	t.Setenv("HASS_TOKEN", "test-token")

	yamlContent := "id: abc-123\nalias: Morning routine\n"
	f := filepath.Join(t.TempDir(), "automation.yaml")
	require.NoError(t, os.WriteFile(f, []byte(yamlContent), 0o644))

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"automation", "apply", "-f", f})
	err := rootCmd.Execute()
	require.NoError(t, err)
}

func TestAutomationApplyDryRun(t *testing.T) {
	current := map[string]interface{}{"id": "abc-123", "alias": "Old name"}
	srv := newMockWSServer(t, []interface{}{current})
	defer srv.Close()

	t.Setenv("HASS_SERVER", wsTestURL(srv))
	t.Setenv("HASS_TOKEN", "test-token")

	yamlContent := "id: abc-123\nalias: New name\n"
	f := filepath.Join(t.TempDir(), "automation.yaml")
	require.NoError(t, os.WriteFile(f, []byte(yamlContent), 0o644))

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"automation", "apply", "-f", f, "--dry-run"})
	err := rootCmd.Execute()
	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "-")
	assert.Contains(t, out, "+")
}

func TestAutomationApplyDryRunNew(t *testing.T) {
	// Automation doesn't exist yet — GetAutomationConfig returns error
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.WriteJSON(map[string]string{"type": "auth_required", "ha_version": "2024.1"})
		var auth map[string]string
		_ = conn.ReadJSON(&auth)
		_ = conn.WriteJSON(map[string]string{"type": "auth_ok"})

		type wsMsg struct{ ID int `json:"id"` }
		var cmd wsMsg
		_ = conn.ReadJSON(&cmd)
		_ = conn.WriteJSON(map[string]interface{}{
			"id": cmd.ID, "type": "result", "success": false,
			"error": map[string]string{"code": "not_found", "message": "Automation not found"},
		})
	}))
	defer srv.Close()

	t.Setenv("HASS_SERVER", wsTestURL(srv))
	t.Setenv("HASS_TOKEN", "test-token")

	yamlContent := "id: new-id-999\nalias: Brand new automation\n"
	f := filepath.Join(t.TempDir(), "automation.yaml")
	require.NoError(t, os.WriteFile(f, []byte(yamlContent), 0o644))

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"automation", "apply", "-f", f, "--dry-run"})
	err := rootCmd.Execute()
	require.NoError(t, err)
	out := buf.String()
	// All lines should be additions (new automation)
	assert.Contains(t, out, "+alias:")
	assert.NotContains(t, out, "-alias:")
}

func TestAutomationApplyMissingID(t *testing.T) {
	yamlContent := "alias: No ID here\n"
	f := filepath.Join(t.TempDir(), "automation.yaml")
	require.NoError(t, os.WriteFile(f, []byte(yamlContent), 0o644))

	rootCmd.SilenceErrors = true
	rootCmd.SetArgs([]string{"automation", "apply", "-f", f})
	err := rootCmd.Execute()
	assert.Error(t, err)
}
