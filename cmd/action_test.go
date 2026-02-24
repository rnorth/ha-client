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
