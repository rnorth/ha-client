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
