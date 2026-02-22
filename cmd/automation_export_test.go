package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/pmezard/go-difflib/difflib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// blank import to promote go-difflib to direct dependency
var _ = difflib.SplitLines

var wsUpgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

func newMockWSServer(t *testing.T, responses []interface{}) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		type wsMsg struct {
			ID   int    `json:"id"`
			Type string `json:"type"`
		}

		_ = conn.WriteJSON(map[string]string{"type": "auth_required", "ha_version": "2024.1"})
		var auth map[string]string
		_ = conn.ReadJSON(&auth)
		_ = conn.WriteJSON(map[string]string{"type": "auth_ok"})

		for _, resp := range responses {
			var cmd wsMsg
			if err := conn.ReadJSON(&cmd); err != nil {
				return
			}
			data, _ := json.Marshal(resp)
			_ = conn.WriteJSON(map[string]interface{}{
				"id": cmd.ID, "type": "result", "success": true,
				"result": json.RawMessage(data),
			})
		}
	}))
	return srv
}

func wsTestURL(srv *httptest.Server) string {
	return "ws" + strings.TrimPrefix(srv.URL, "http")
}

func TestAutomationExport(t *testing.T) {
	entity := map[string]interface{}{
		"entity_id": "automation.morning",
		"unique_id": "abc-123",
		"platform":  "automation",
	}
	cfg := map[string]interface{}{
		"id":    "abc-123",
		"alias": "Morning routine",
		"trigger": []interface{}{
			map[string]interface{}{"platform": "time", "at": "07:00:00"},
		},
	}
	srv := newMockWSServer(t, []interface{}{entity, cfg})
	defer srv.Close()

	t.Setenv("HASS_SERVER", wsTestURL(srv))
	t.Setenv("HASS_TOKEN", "test-token")

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"automation", "export", "automation.morning", "-o", "yaml"})
	err := rootCmd.Execute()
	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "alias: Morning routine")
	assert.Contains(t, out, "id: abc-123")
}

func TestAutomationExportNotFound(t *testing.T) {
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
			"error": map[string]string{"code": "not_found", "message": "Entity not found"},
		})
	}))
	defer srv.Close()

	t.Setenv("HASS_SERVER", wsTestURL(srv))
	t.Setenv("HASS_TOKEN", "test-token")

	rootCmd.SetArgs([]string{"automation", "export", "automation.missing", "-o", "yaml"})
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
	err := rootCmd.Execute()
	assert.Error(t, err)
}
