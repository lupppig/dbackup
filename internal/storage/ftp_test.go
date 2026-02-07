package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestFTPStorage_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	// stilliard/pure-ftpd
	username := "testuser"
	password := "testpass"
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "stilliard/pure-ftpd",
			Env: map[string]string{
				"FTP_USER_NAME": username,
				"FTP_USER_PASS": password,
				"FTP_USER_HOME": "/home/testuser",
				"PUBLICHOST":    "localhost",
			},
			ExposedPorts: []string{"21/tcp", "30000-30009/tcp"},
			WaitingFor:   wait.ForLog("Starting Pure-FTPd"),
		},
		Started: true,
	})
	require.NoError(t, err)
	defer container.Terminate(ctx)

	host, err := container.Host(ctx)
	require.NoError(t, err)
	if host == "localhost" || host == "::1" {
		host = "127.0.0.1"
	}

	port, err := container.MappedPort(ctx, "21")
	require.NoError(t, err)

	uri := fmt.Sprintf("ftp://%s:%s@%s:%d/", username, password, host, port.Int())
	u, err := url.Parse(uri)
	require.NoError(t, err)

	s, err := NewFTPStorage(u, StorageOptions{AllowInsecure: true})
	require.NoError(t, err)
	defer s.Close()

	t.Run("SaveAndOpen", func(t *testing.T) {
		content := []byte("hello ftp")
		name := "test.txt"
		path, err := s.Save(ctx, name, bytes.NewReader(content))
		assert.NoError(t, err)
		assert.Contains(t, path, name)

		r, err := s.Open(ctx, name)
		if assert.NoError(t, err) {
			defer r.Close()
			got, err := io.ReadAll(r)
			assert.NoError(t, err)
			assert.Equal(t, content, got)
		}
	})

	t.Run("MetadataOperations", func(t *testing.T) {
		metaData := []byte("meta")
		name := "backups/test.manifest"
		err := s.PutMetadata(ctx, name, metaData)
		assert.NoError(t, err)

		got, err := s.GetMetadata(ctx, name)
		assert.NoError(t, err)
		assert.Equal(t, metaData, got)

		files, err := s.ListMetadata(ctx, "backups/")
		assert.NoError(t, err)
		assert.Contains(t, files, name)
	})

	t.Run("Delete", func(t *testing.T) {
		name := "to_delete.txt"
		_, err := s.Save(ctx, name, bytes.NewReader([]byte("bye")))
		assert.NoError(t, err)

		err = s.Delete(ctx, name)
		assert.NoError(t, err)

		_, err = s.Open(ctx, name)
		assert.Error(t, err)
	})
}
