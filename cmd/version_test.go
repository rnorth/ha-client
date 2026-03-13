package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersion_JSON(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"version", "-o", "json"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), `"version"`)
	assert.Contains(t, buf.String(), `"go"`)
	assert.Contains(t, buf.String(), "0.1.0")
}
