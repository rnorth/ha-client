package cmd

import (
	"testing"

	"github.com/rnorth/ha-client/internal/client"
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
