package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateEval_InlineArg(t *testing.T) {
	var gotBody map[string]string
	srv := newMockRESTServer(t, map[string]http.HandlerFunc{
		"/api/template": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
			_, _ = w.Write([]byte("22.5"))
		},
	})
	defer srv.Close()

	t.Setenv("HASS_SERVER", srv.URL)
	t.Setenv("HASS_TOKEN", "test-token")

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"template", "eval", `{{ states("sensor.temperature") }}`})
	require.NoError(t, rootCmd.Execute())
	assert.Equal(t, `{{ states("sensor.temperature") }}`, gotBody["template"])
	assert.Equal(t, "22.5\n", buf.String())
}

func TestTemplateEval_File(t *testing.T) {
	var gotBody map[string]string
	srv := newMockRESTServer(t, map[string]http.HandlerFunc{
		"/api/template": func(w http.ResponseWriter, r *http.Request) {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
			_, _ = w.Write([]byte("rendered"))
		},
	})
	defer srv.Close()

	t.Setenv("HASS_SERVER", srv.URL)
	t.Setenv("HASS_TOKEN", "test-token")
	t.Cleanup(func() { templateEvalFile = "" })

	tmpFile := filepath.Join(t.TempDir(), "tpl.j2")
	require.NoError(t, os.WriteFile(tmpFile, []byte("{{ now() }}"), 0644))

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"template", "eval", "-f", tmpFile})
	require.NoError(t, rootCmd.Execute())
	assert.Equal(t, "{{ now() }}", gotBody["template"])
	assert.Equal(t, "rendered\n", buf.String())
}

func TestTemplateEval_NoInput(t *testing.T) {
	t.Setenv("HASS_SERVER", "http://localhost")
	t.Setenv("HASS_TOKEN", "test-token")

	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
	rootCmd.SetArgs([]string{"template", "eval"})
	err := rootCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "provide a template")
}
