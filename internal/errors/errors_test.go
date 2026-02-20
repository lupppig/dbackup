package apperrors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppError_ErrorFormatting(t *testing.T) {
	err := New(TypeConnection, "database unreachable", "Check your firewall settings.")

	assert.Equal(t, "database unreachable", err.Error())
	assert.Equal(t, TypeConnection, err.Type)
	assert.Equal(t, "database unreachable", err.Message)
	assert.Equal(t, "Check your firewall settings.", err.Hint)
}

func TestAppError_Unwrap(t *testing.T) {
	baseErr := errors.New("underlying socket error")
	appErr := Wrap(baseErr, TypeConnection, "database unreachable", "Check your firewall settings.")

	assert.Equal(t, "database unreachable: underlying socket error", appErr.Error())

	assert.True(t, errors.Is(appErr, baseErr))

	unwrapped := errors.Unwrap(appErr)
	assert.Equal(t, baseErr, unwrapped)
}

func TestAppError_IsType(t *testing.T) {
	err := New(TypeAuth, "access denied", "Check credentials")
	assert.True(t, IsType(err, TypeAuth))
	assert.False(t, IsType(err, TypeConnection))

	stdErr := errors.New("standard error")
	assert.False(t, IsType(stdErr, TypeAuth))

	wrapped := fmt.Errorf("wrapped: %w", err)
	assert.True(t, IsType(wrapped, TypeAuth))
}
