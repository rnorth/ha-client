package errors

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassify_AuthError(t *testing.T) {
	ce := Classify(fmt.Errorf("unauthorized"))
	require.NotNil(t, ce)
	assert.Equal(t, ExitAuth, ce.ExitCode)
	assert.Equal(t, "auth_failed", ce.Code)
	assert.Equal(t, "unauthorized", ce.Error())
}

func TestClassify_NotFoundError(t *testing.T) {
	ce := Classify(fmt.Errorf("entity not found"))
	require.NotNil(t, ce)
	assert.Equal(t, ExitNotFound, ce.ExitCode)
	assert.Equal(t, "not_found", ce.Code)
}

func TestClassify_ServerError(t *testing.T) {
	ce := Classify(fmt.Errorf("HTTP 500 Internal Server Error"))
	require.NotNil(t, ce)
	assert.Equal(t, ExitServer, ce.ExitCode)
	assert.Equal(t, "server_error", ce.Code)
}

func TestClassify_GeneralError(t *testing.T) {
	ce := Classify(fmt.Errorf("something went wrong"))
	require.NotNil(t, ce)
	assert.Equal(t, ExitGeneral, ce.ExitCode)
	assert.Equal(t, "error", ce.Code)
}

func TestClassify_AlreadyClassified(t *testing.T) {
	original := &CLIError{
		Err:      fmt.Errorf("custom error"),
		ExitCode: ExitUsage,
		Code:     "usage_error",
	}
	ce := Classify(original)
	require.NotNil(t, ce)
	assert.Same(t, original, ce)
	assert.Equal(t, ExitUsage, ce.ExitCode)
	assert.Equal(t, "usage_error", ce.Code)
}

func TestClassify_Nil(t *testing.T) {
	ce := Classify(nil)
	assert.Nil(t, ce)
}

func TestCLIError_Unwrap(t *testing.T) {
	inner := fmt.Errorf("inner error")
	ce := &CLIError{Err: inner, ExitCode: ExitGeneral, Code: "error"}
	assert.Equal(t, inner, ce.Unwrap())
}
