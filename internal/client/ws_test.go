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
		_ = conn.WriteJSON(map[string]string{"type": "auth_required", "ha_version": "2024.1"})

		// Read auth message
		var authMsg map[string]string
		_ = conn.ReadJSON(&authMsg)
		if authMsg["access_token"] != token {
			_ = conn.WriteJSON(map[string]string{"type": "auth_invalid"})
			return
		}
		_ = conn.WriteJSON(map[string]string{"type": "auth_ok"})

		// Read command
		var cmd client.WSMessage
		_ = conn.ReadJSON(&cmd)
		// Respond with result
		resultData, _ := json.Marshal(response)
		_ = conn.WriteJSON(map[string]interface{}{
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

// mockWSServerSeq handles HA auth + multiple sequential command responses in order.
func mockWSServerSeq(t *testing.T, token string, responses []interface{}) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		_ = conn.WriteJSON(map[string]string{"type": "auth_required", "ha_version": "2024.1"})
		var authMsg map[string]string
		_ = conn.ReadJSON(&authMsg)
		if authMsg["access_token"] != token {
			_ = conn.WriteJSON(map[string]string{"type": "auth_invalid"})
			return
		}
		_ = conn.WriteJSON(map[string]string{"type": "auth_ok"})

		for _, response := range responses {
			var cmd client.WSMessage
			if err := conn.ReadJSON(&cmd); err != nil {
				return
			}
			resultData, _ := json.Marshal(response)
			_ = conn.WriteJSON(map[string]interface{}{
				"id":      cmd.ID,
				"type":    "result",
				"success": true,
				"result":  json.RawMessage(resultData),
			})
		}
	}))
	return srv
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
func TestGetEntityUniqueID(t *testing.T) {
	entity := client.EntityEntry{EntityID: "automation.morning", UniqueID: "abc-123"}
	srv := mockWSServer(t, "test-token", "config/entity_registry/get", entity)
	defer srv.Close()

	wsc, err := client.NewWSClient(wsURL(srv), "test-token")
	require.NoError(t, err)
	defer wsc.Close()

	result, err := wsc.GetEntity("automation.morning")
	require.NoError(t, err)
	assert.Equal(t, "abc-123", result.UniqueID)
}

func TestWSClientGetAutomationConfig(t *testing.T) {
	cfg := map[string]interface{}{
		"id":    "abc-123",
		"alias": "Morning routine",
		"trigger": []interface{}{
			map[string]interface{}{"platform": "time", "at": "07:00:00"},
		},
	}
	srv := mockWSServer(t, "test-token", "automation/config", cfg)
	defer srv.Close()

	wsc, err := client.NewWSClient(wsURL(srv), "test-token")
	require.NoError(t, err)
	defer wsc.Close()

	result, err := wsc.GetAutomationConfig("automation.morning")
	require.NoError(t, err)
	assert.Equal(t, "Morning routine", result["alias"])
	assert.Equal(t, "abc-123", result["id"])
}

