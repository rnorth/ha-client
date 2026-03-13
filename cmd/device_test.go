package cmd

import (
	"io"
	"os"
	"testing"

	"github.com/rnorth/ha-client/internal/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeviceList(t *testing.T) {
	devices := []client.Device{
		{ID: "device-1", Name: "Smart Bulb", Manufacturer: "Philips", Model: "Hue"},
	}
	srv := newMockWSServer(t, []interface{}{devices})
	defer srv.Close()

	t.Setenv("HASS_SERVER", srv.URL)
	t.Setenv("HASS_TOKEN", "test-token")

	rootCmd.SetArgs([]string{"device", "list", "-o", "json"})
	require.NoError(t, rootCmd.Execute())
}

func TestDeviceGet(t *testing.T) {
	devices := []client.Device{
		{ID: "device-1", Name: "Smart Bulb", Manufacturer: "Philips", Model: "Hue"},
	}
	srv := newMockWSServer(t, []interface{}{devices})
	defer srv.Close()

	t.Setenv("HASS_SERVER", srv.URL)
	t.Setenv("HASS_TOKEN", "test-token")

	rootCmd.SetArgs([]string{"device", "get", "device-1", "-o", "json"})
	require.NoError(t, rootCmd.Execute())
}

func TestDeviceList_AreaFilter(t *testing.T) {
	devices := []client.Device{
		{ID: "dev-1", Name: "Bulb", AreaID: "living_room"},
		{ID: "dev-2", Name: "Fan", AreaID: "bedroom"},
	}
	srv := newMockWSServer(t, []interface{}{devices})
	defer srv.Close()

	t.Setenv("HASS_SERVER", srv.URL)
	t.Setenv("HASS_TOKEN", "test-token")
	t.Cleanup(func() { deviceListArea = "" })

	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = origStdout })

	rootCmd.SetArgs([]string{"device", "list", "--area", "living_room", "-o", "json"})
	require.NoError(t, rootCmd.Execute())

	_ = w.Close()
	out, _ := io.ReadAll(r)
	assert.Contains(t, string(out), "Bulb")
	assert.NotContains(t, string(out), "Fan")
}
