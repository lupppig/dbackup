package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockAdapter struct {
	name string
}

func (m MockAdapter) Name() string {
	return m.name
}

func (m MockAdapter) TestConnection(ctx context.Context, conn ConnectionParams) error {
	return nil
}

func (m MockAdapter) BuildConnection(ctx context.Context, conn ConnectionParams) (string, error) {
	return "", nil
}

func TestRegisterAndGetAdapter(t *testing.T) {
	name := "mock_db"
	adapter := MockAdapter{name: name}

	RegisterAdapter(adapter)

	got, err := GetAdapter(name)
	require.NoError(t, err)
	assert.Equal(t, name, got.Name())
}

func TestGetUnsupportedAdapter(t *testing.T) {
	_, err := GetAdapter("non_existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported database: non_existent")
}
