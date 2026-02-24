package cmd

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/rnorth/ha-client/internal/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionList(t *testing.T) {
	srv := newMockRESTServer(t, map[string]http.HandlerFunc{
		"/api/services": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			_ = json.NewEncoder(w).Encode([]client.ActionDomain{
				{Domain: "light", Services: map[string]client.ActionDetail{
					"turn_on": {Name: "Turn on", Description: "Turn on a light"},
				}},
			})
		},
	})
	defer srv.Close()

	t.Setenv("HASS_SERVER", srv.URL)
	t.Setenv("HASS_TOKEN", "test-token")

	rootCmd.SetArgs([]string{"action", "list", "-o", "json"})
	require.NoError(t, rootCmd.Execute())
}

func TestActionCall_EntityID(t *testing.T) {
	var gotBody map[string]interface{}
	srv := newMockRESTServer(t, map[string]http.HandlerFunc{
		"/api/services/light/turn_on": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
			w.WriteHeader(http.StatusOK)
		},
	})
	defer srv.Close()

	t.Setenv("HASS_SERVER", srv.URL)
	t.Setenv("HASS_TOKEN", "test-token")
	t.Cleanup(func() { actionEntityID = "" })

	rootCmd.SetArgs([]string{"action", "call", "light.turn_on", "--entity_id=light.desk"})
	require.NoError(t, rootCmd.Execute())
	assert.Equal(t, "light.desk", gotBody["entity_id"])
}

func TestActionCall_DataFields(t *testing.T) {
	var gotBody map[string]interface{}
	srv := newMockRESTServer(t, map[string]http.HandlerFunc{
		"/api/services/light/turn_on": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			w.WriteHeader(http.StatusOK)
		},
	})
	defer srv.Close()

	t.Setenv("HASS_SERVER", srv.URL)
	t.Setenv("HASS_TOKEN", "test-token")
	t.Cleanup(func() { actionDataFields = nil })

	rootCmd.SetArgs([]string{
		"action", "call", "light.turn_on",
		"-d", "transition=5",
		"-d", "brightness_pct=80",
	})
	require.NoError(t, rootCmd.Execute())
	assert.Equal(t, "5", gotBody["transition"])
	assert.Equal(t, "80", gotBody["brightness_pct"])
}

func TestActionCall_DataJSON(t *testing.T) {
	var gotBody map[string]interface{}
	srv := newMockRESTServer(t, map[string]http.HandlerFunc{
		"/api/services/light/turn_on": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			w.WriteHeader(http.StatusOK)
		},
	})
	defer srv.Close()

	t.Setenv("HASS_SERVER", srv.URL)
	t.Setenv("HASS_TOKEN", "test-token")
	t.Cleanup(func() { actionDataJSONRaw = "" })

	rootCmd.SetArgs([]string{
		"action", "call", "light.turn_on",
		"--data-json", `{"entity_id":"light.desk","transition":5}`,
	})
	require.NoError(t, rootCmd.Execute())
	assert.Equal(t, "light.desk", gotBody["entity_id"])
	assert.Equal(t, float64(5), gotBody["transition"])
}

func TestActionCall_MergeOrder(t *testing.T) {
	var gotBody map[string]interface{}
	srv := newMockRESTServer(t, map[string]http.HandlerFunc{
		"/api/services/light/turn_on": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			w.WriteHeader(http.StatusOK)
		},
	})
	defer srv.Close()

	t.Setenv("HASS_SERVER", srv.URL)
	t.Setenv("HASS_TOKEN", "test-token")
	t.Cleanup(func() {
		actionDataJSONRaw = ""
		actionDataFields = nil
		actionEntityID = ""
	})

	// data-json sets base; -d overrides transition; --entity_id wins over everything
	rootCmd.SetArgs([]string{
		"action", "call", "light.turn_on",
		"--data-json", `{"brightness_pct":50,"transition":1}`,
		"-d", "transition=5",
		"--entity_id=light.desk",
	})
	require.NoError(t, rootCmd.Execute())
	assert.Equal(t, float64(50), gotBody["brightness_pct"]) // from --data-json, not overridden
	assert.Equal(t, "5", gotBody["transition"])             // -d overrides data-json
	assert.Equal(t, "light.desk", gotBody["entity_id"])    // --entity_id wins
}

func TestActionCall_InvalidDataField(t *testing.T) {
	t.Cleanup(func() { actionDataFields = nil })

	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
	rootCmd.SetArgs([]string{"action", "call", "light.turn_on", "-d", "notakvpair"})
	err := rootCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `invalid -d flag "notakvpair": expected key=value`)
}
