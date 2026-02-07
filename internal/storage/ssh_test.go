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

func TestSSHStorage_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	// Start atmoz/sftp container
	// Format: user:pass:uid:gid:dir
	username := "testuser"
	password := "testpass"
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "atmoz/sftp",
			Env: map[string]string{
				"SFTP_USERS": fmt.Sprintf("%s:%s:::upload", username, password),
			},
			ExposedPorts: []string{"22/tcp"},
			WaitingFor:   wait.ForLog("Server listening on"),
		},
		Started: true,
	})
	require.NoError(t, err)
	defer container.Terminate(ctx)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "22")
	require.NoError(t, err)

	uri := fmt.Sprintf("sftp://%s:%s@%s:%d/upload", username, password, host, port.Int())
	u, err := url.Parse(uri)
	require.NoError(t, err)

	s, err := NewSSHStorage(u)
	require.NoError(t, err)
	defer s.Close()

	t.Run("SaveAndOpen", func(t *testing.T) {
		content := []byte("hello sftp")
		name := "test.txt"
		path, err := s.Save(ctx, name, bytes.NewReader(content))
		assert.NoError(t, err)
		assert.Contains(t, path, name)

		r, err := s.Open(ctx, name)
		assert.NoError(t, err)
		defer r.Close()

		got, err := io.ReadAll(r)
		assert.NoError(t, err)
		assert.Equal(t, content, got)
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

	t.Run("Runner", func(t *testing.T) {
		t.Skip("atmoz/sftp restricts shell access, skipping Runner test")
		var buf bytes.Buffer
		err := s.Run(ctx, "echo", []string{"hello from ssh"}, &buf)
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "hello from ssh")
	})
}
