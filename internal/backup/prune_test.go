package backup

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/lupppig/dbackup/internal/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockStorage struct {
	mock.Mock
}

func (m *MockStorage) Save(ctx context.Context, name string, r io.Reader) (string, error) {
	args := m.Called(ctx, name, r)
	return args.String(0), args.Error(1)
}

func (m *MockStorage) Open(ctx context.Context, name string) (io.ReadCloser, error) {
	args := m.Called(ctx, name)
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *MockStorage) Exists(ctx context.Context, name string) (bool, error) {
	return true, nil
}

func (m *MockStorage) Delete(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

func (m *MockStorage) Location() string {
	return "mock://"
}

func (m *MockStorage) PutMetadata(ctx context.Context, name string, data []byte) error {
	args := m.Called(ctx, name, data)
	return args.Error(0)
}

func (m *MockStorage) GetMetadata(ctx context.Context, name string) ([]byte, error) {
	args := m.Called(ctx, name)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockStorage) ListMetadata(ctx context.Context, prefix string) ([]string, error) {
	args := m.Called(ctx, prefix)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockStorage) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestPruneManager_Prune(t *testing.T) {
	ctx := context.Background()
	ms := new(MockStorage)

	// Create some manifests
	m1 := &manifest.Manifest{ID: "m1", Engine: "postgres", DBName: "db1", CreatedAt: time.Now().Add(-24 * time.Hour)}
	m2 := &manifest.Manifest{ID: "m2", Engine: "postgres", DBName: "db1", CreatedAt: time.Now().Add(-12 * time.Hour)}
	m3 := &manifest.Manifest{ID: "m3", Engine: "postgres", DBName: "db1", CreatedAt: time.Now()}

	m1b, _ := m1.Serialize()
	m2b, _ := m2.Serialize()
	m3b, _ := m3.Serialize()

	ms.On("ListMetadata", ctx, "").Return([]string{"b1.manifest", "b2.manifest", "b3.manifest"}, nil)
	ms.On("GetMetadata", ctx, "b1.manifest").Return(m1b, nil)
	ms.On("GetMetadata", ctx, "b2.manifest").Return(m2b, nil)
	ms.On("GetMetadata", ctx, "b3.manifest").Return(m3b, nil)

	// Expected retention of 2 backups: b1 should be deleted as it is the oldest.
	ms.On("Delete", ctx, "b1").Return(nil)
	ms.On("Delete", ctx, "b1.manifest").Return(nil)

	pm := NewPruneManager(ms, PruneOptions{
		Keep:   2,
		DBType: "postgres",
		DBName: "db1",
	})

	err := pm.Prune(ctx)
	assert.NoError(t, err)

	ms.AssertExpectations(t)
}

func TestPruneManager_Retention(t *testing.T) {
	ctx := context.Background()
	ms := new(MockStorage)

	// m1 is 2 days old, m2 is 1 hour old
	m1 := &manifest.Manifest{ID: "m1", Engine: "postgres", DBName: "db1", CreatedAt: time.Now().Add(-48 * time.Hour)}
	m2 := &manifest.Manifest{ID: "m2", Engine: "postgres", DBName: "db1", CreatedAt: time.Now().Add(-1 * time.Hour)}

	m1b, _ := m1.Serialize()
	m2b, _ := m2.Serialize()

	ms.On("ListMetadata", ctx, "").Return([]string{"old.manifest", "new.manifest"}, nil)
	ms.On("GetMetadata", ctx, "old.manifest").Return(m1b, nil)
	ms.On("GetMetadata", ctx, "new.manifest").Return(m2b, nil)

	// Retention is 1 day, so m1 (old.manifest) should be deleted
	ms.On("Delete", ctx, "old").Return(nil)
	ms.On("Delete", ctx, "old.manifest").Return(nil)

	pm := NewPruneManager(ms, PruneOptions{
		Retention: 24 * time.Hour,
		DBType:    "postgres",
		DBName:    "db1",
	})

	err := pm.Prune(ctx)
	assert.NoError(t, err)

	ms.AssertExpectations(t)
}
