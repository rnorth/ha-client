package cmd

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/rnorth/ha-client/internal/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateList(t *testing.T) {
	srv := newMockRESTServer(t, map[string]http.HandlerFunc{
		"/api/states": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			_ = json.NewEncoder(w).Encode([]client.State{
				{EntityID: "light.desk", State: "on"},
				{EntityID: "switch.fan", State: "off"},
			})
		},
	})
	defer srv.Close()

	t.Setenv("HASS_SERVER", srv.URL)
	t.Setenv("HASS_TOKEN", "test-token")

	rootCmd.SetArgs([]string{"state", "list", "-o", "json"})
	require.NoError(t, rootCmd.Execute())
}

func TestStateGet(t *testing.T) {
	srv := newMockRESTServer(t, map[string]http.HandlerFunc{
		"/api/states/light.desk": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			_ = json.NewEncoder(w).Encode(client.State{EntityID: "light.desk", State: "on"})
		},
	})
	defer srv.Close()

	t.Setenv("HASS_SERVER", srv.URL)
	t.Setenv("HASS_TOKEN", "test-token")

	rootCmd.SetArgs([]string{"state", "get", "light.desk", "-o", "json"})
	require.NoError(t, rootCmd.Execute())
}
