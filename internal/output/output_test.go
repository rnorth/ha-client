package output_test

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rnorth/ha-client/internal/client"
	"github.com/rnorth/ha-client/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type item struct {
	Name  string `json:"name" yaml:"name"`
	State string `json:"state" yaml:"state"`
}

func TestJSONOutput(t *testing.T) {
	var buf bytes.Buffer
	data := []item{{"light.desk", "on"}, {"switch.fan", "off"}}

	err := output.Render(&buf, output.FormatJSON, data, nil)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), `"name": "light.desk"`)
	assert.Contains(t, buf.String(), `"state": "on"`)
}

func TestYAMLOutput(t *testing.T) {
	var buf bytes.Buffer
	data := []item{{"light.desk", "on"}}

	err := output.Render(&buf, output.FormatYAML, data, nil)
	require.NoError(t, err)

	var out []item
	require.NoError(t, yaml.Unmarshal(buf.Bytes(), &out))
	assert.Equal(t, "light.desk", out[0].Name)
}

func TestAutoFormatNoTTY(t *testing.T) {
	// When stdout is not a TTY (like in tests/pipes), auto → JSON
	fmt := output.DetectFormat("", os.Stdout)
	// In test runner, stdout is not a TTY
	assert.Equal(t, output.FormatJSON, fmt)
}

func TestExplicitOverride(t *testing.T) {
	fmt := output.DetectFormat("yaml", os.Stdout)
	assert.Equal(t, output.FormatYAML, fmt)
}

func TestTableOutput(t *testing.T) {
	var buf bytes.Buffer
	data := []item{
		{"update.home_assistant_supervisor_update", "off"},
		{"light.desk", "on"},
	}

	err := output.Render(&buf, output.FormatTable, data, nil)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	// header + 2 data rows = exactly 3 lines, no cell wrapping
	assert.Len(t, lines, 3)
	// Header uses HA JSON tag names uppercased, not Go field names
	assert.Contains(t, lines[0], "NAME")   // json:"name"
	assert.Contains(t, lines[0], "STATE")  // json:"state"
	// Each entity is on its own line
	assert.Contains(t, lines[1], "update.home_assistant_supervisor_update")
	assert.Contains(t, lines[2], "light.desk")
	// No border characters
	assert.NotContains(t, buf.String(), "│")
	assert.NotContains(t, buf.String(), "┌")
	assert.NotContains(t, buf.String(), "|")
}

func TestTableColumnHeadersUseJSONTags(t *testing.T) {
	// When explicit Go field names are given as column overrides, the header
	// must use the JSON tag (HA-familiar) not the Go field name.
	var buf bytes.Buffer
	states := []client.State{
		{EntityID: "light.desk", State: "on", LastUpdated: time.Time{}},
	}
	err := output.Render(&buf, output.FormatTable, states, []string{"EntityID", "State", "LastUpdated"})
	require.NoError(t, err)

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	header := lines[0]
	assert.Contains(t, header, "ENTITY_ID")    // json:"entity_id", not "EntityID"
	assert.Contains(t, header, "STATE")         // json:"state"
	assert.Contains(t, header, "LAST_UPDATED")  // json:"last_updated", not "LastUpdated"
	assert.NotContains(t, header, "EntityID")
	assert.NotContains(t, header, "LastUpdated")
}
