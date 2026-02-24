package cmd

import (
	"testing"

	"github.com/rnorth/ha-client/internal/client"
	"github.com/stretchr/testify/require"
)

func TestEntityList(t *testing.T) {
	entities := []client.EntityEntry{
		{EntityID: "light.desk", Name: "Desk Light", Platform: "hue"},
	}
	srv := newMockWSServer(t, []interface{}{entities})
	defer srv.Close()

	t.Setenv("HASS_SERVER", srv.URL)
	t.Setenv("HASS_TOKEN", "test-token")

	rootCmd.SetArgs([]string{"entity", "list", "-o", "json"})
	require.NoError(t, rootCmd.Execute())
}

func TestEntityGet(t *testing.T) {
	entity := client.EntityEntry{EntityID: "light.desk", Name: "Desk Light", Platform: "hue"}
	srv := newMockWSServer(t, []interface{}{entity})
	defer srv.Close()

	t.Setenv("HASS_SERVER", srv.URL)
	t.Setenv("HASS_TOKEN", "test-token")

	rootCmd.SetArgs([]string{"entity", "get", "light.desk", "-o", "json"})
	require.NoError(t, rootCmd.Execute())
}
