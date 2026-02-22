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

// newMockRESTServer returns an httptest.Server that handles automation config REST endpoints.
// handlers maps URL path → handler func; unmatched paths return 404.
func newMockRESTServer(t *testing.T, handlers map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h, ok := handlers[r.URL.Path]; ok {
			h(w, r)
		} else {
			http.NotFound(w, r)
		}
	}))
}

func TestAutomationApply(t *testing.T) {
	srv := newMockRESTServer(t, map[string]http.HandlerFunc{
		"/api/config/automation/config/abc-123": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			w.WriteHeader(http.StatusOK)
		},
	})
	defer srv.Close()

	t.Setenv("HASS_SERVER", srv.URL)
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
	srv := newMockRESTServer(t, map[string]http.HandlerFunc{
		"/api/config/automation/config/abc-123": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": "abc-123", "alias": "Old name"})
		},
	})
	defer srv.Close()

	t.Setenv("HASS_SERVER", srv.URL)
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
	assert.Contains(t, out, "-alias: Old name")
	assert.Contains(t, out, "+alias: New name")
}

func TestAutomationApplyDryRunNew(t *testing.T) {
	// Automation doesn't exist yet — GET returns 404
	srv := newMockRESTServer(t, map[string]http.HandlerFunc{
		"/api/config/automation/config/new-id-999": func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		},
	})
	defer srv.Close()

	t.Setenv("HASS_SERVER", srv.URL)
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
