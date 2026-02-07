package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestS3Storage_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	accessKey := "minioadmin"
	secretKey := "minioadmin"
	bucketName := "testbucket"

	// Start MinIO container
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "minio/minio",
			Env: map[string]string{
				"MINIO_ACCESS_KEY": accessKey,
				"MINIO_SECRET_KEY": secretKey,
			},
			Cmd:          []string{"server", "/data"},
			ExposedPorts: []string{"9000/tcp"},
			WaitingFor:   wait.ForHTTP("/minio/health/ready").WithPort("9000/tcp"),
		},
		Started: true,
	})
	require.NoError(t, err)
	defer container.Terminate(ctx)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "9000")
	require.NoError(t, err)

	endpoint := fmt.Sprintf("%s:%d", host, port.Int())
	uri := fmt.Sprintf("s3://%s:%s@%s/%s/backups?ssl=false", accessKey, secretKey, endpoint, bucketName)
	u, err := url.Parse(uri)
	require.NoError(t, err)

	s, err := NewS3Storage(u)
	require.NoError(t, err)

	// Create bucket
	err = s.client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
	require.NoError(t, err)

	t.Run("SaveAndOpen", func(t *testing.T) {
		content := []byte("hello s3")
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
		metaData := []byte("{\"version\":\"1.0\"}")
		name := "manifests/v1.json"
		err := s.PutMetadata(ctx, name, metaData)
		assert.NoError(t, err)

		got, err := s.GetMetadata(ctx, name)
		assert.NoError(t, err)
		assert.Equal(t, metaData, got)

		files, err := s.ListMetadata(ctx, "manifests/")
		assert.NoError(t, err)
		assert.Contains(t, files, name)
	})

	t.Run("Delete", func(t *testing.T) {
		name := "to_delete.txt"
		_, err := s.Save(ctx, name, bytes.NewReader([]byte("bye")))
		assert.NoError(t, err)

		err = s.Delete(ctx, name)
		assert.NoError(t, err)

		_, err = s.client.StatObject(ctx, s.bucketName, s.getObjectName(name), minio.StatObjectOptions{})
		assert.Error(t, err)
	})

	t.Run("SaveAndOpen_UnknownSize", func(t *testing.T) {
		content := []byte("hello s3 with unknown size")
		name := "test_unknown.txt"
		// Wrap in a plain io.Reader to hide the size
		wrappedReader := struct{ io.Reader }{bytes.NewReader(content)}
		path, err := s.Save(ctx, name, wrappedReader)
		assert.NoError(t, err)
		assert.Contains(t, path, name)

		r, err := s.Open(ctx, name)
		assert.NoError(t, err)
		defer r.Close()

		got, err := io.ReadAll(r)
		assert.NoError(t, err)
		assert.Equal(t, content, got)
	})
}
