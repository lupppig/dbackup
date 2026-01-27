package database

import (
	"context"
	"testing"
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
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got.Name() != name {
		t.Errorf("expected %s, got %s", name, got.Name())
	}
}

func TestGetUnsupportedAdapter(t *testing.T) {
	_, err := GetAdapter("non_existent")
	if err == nil {
		t.Fatal("expected error for unsupported adapter, got nil")
	}

	expectedMsg := "unsupported database: non_existent"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}
