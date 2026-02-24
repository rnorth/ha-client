package cmd

import (
	"testing"

	"github.com/rnorth/ha-client/internal/client"
	"github.com/stretchr/testify/require"
)

func TestAreaList(t *testing.T) {
	areas := []client.Area{
		{AreaID: "living_room", Name: "Living Room"},
		{AreaID: "bedroom", Name: "Bedroom"},
	}
	srv := newMockWSServer(t, []interface{}{areas})
	defer srv.Close()

	t.Setenv("HASS_SERVER", srv.URL)
	t.Setenv("HASS_TOKEN", "test-token")

	rootCmd.SetArgs([]string{"area", "list", "-o", "json"})
	require.NoError(t, rootCmd.Execute())
}

func TestAreaGet(t *testing.T) {
	areas := []client.Area{
		{AreaID: "living_room", Name: "Living Room"},
	}
	srv := newMockWSServer(t, []interface{}{areas})
	defer srv.Close()

	t.Setenv("HASS_SERVER", srv.URL)
	t.Setenv("HASS_TOKEN", "test-token")

	rootCmd.SetArgs([]string{"area", "get", "living_room", "-o", "json"})
	require.NoError(t, rootCmd.Execute())
}
