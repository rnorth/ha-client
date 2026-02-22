package config_test

import (
	"os"
	"testing"

	"github.com/rnorth/ha-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
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
	_ = os.Unsetenv("HASS_SERVER")
	_ = os.Unsetenv("HASS_TOKEN")

	// Keychain takes priority over the file (by design). Skip rather than
	// delete real credentials — this test is reliable in CI where keychain is empty.
	if s, _ := keyring.Get("ha-client", "server"); s != "" {
		t.Skip("keychain has credentials; file-fallback test requires empty keychain")
	}

	dir := t.TempDir()
	cfgFile := dir + "/config.yaml"
	err := os.WriteFile(cfgFile, []byte("server: http://from-file:8123\ntoken: file-token\n"), 0600)
	require.NoError(t, err)

	cfg, err := config.ResolveWithFile("", "", cfgFile)
	require.NoError(t, err)
	assert.Equal(t, "http://from-file:8123", cfg.Server)
	assert.Equal(t, "file-token", cfg.Token)
}
