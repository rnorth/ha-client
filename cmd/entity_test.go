package cmd

import (
	"io"
	"os"
	"testing"

	"github.com/rnorth/ha-client/internal/client"
	"github.com/stretchr/testify/assert"
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

func TestEntityList_DomainFilter(t *testing.T) {
	entities := []client.EntityEntry{
		{EntityID: "light.desk", Name: "Desk Light", Platform: "hue"},
		{EntityID: "switch.fan", Name: "Fan", Platform: "zwave"},
		{EntityID: "light.bedroom", Name: "Bedroom", Platform: "hue"},
	}
	srv := newMockWSServer(t, []interface{}{entities})
	defer srv.Close()

	t.Setenv("HASS_SERVER", srv.URL)
	t.Setenv("HASS_TOKEN", "test-token")
	t.Cleanup(func() { entityListDomain = "" })

	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = origStdout })

	rootCmd.SetArgs([]string{"entity", "list", "--domain", "light", "-o", "json"})
	require.NoError(t, rootCmd.Execute())

	w.Close()
	out, _ := io.ReadAll(r)
	assert.Contains(t, string(out), "light.desk")
	assert.NotContains(t, string(out), "switch.fan")
}
