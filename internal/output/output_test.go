package output_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/rnorth/ha-cli/internal/output"
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
	// Header is uppercase
	assert.Contains(t, lines[0], "NAME")
	assert.Contains(t, lines[0], "STATE")
	// Each entity is on its own line
	assert.Contains(t, lines[1], "update.home_assistant_supervisor_update")
	assert.Contains(t, lines[2], "light.desk")
	// No border characters
	assert.NotContains(t, buf.String(), "│")
	assert.NotContains(t, buf.String(), "┌")
	assert.NotContains(t, buf.String(), "|")
}
