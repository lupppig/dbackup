package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3Storage struct {
	client     *minio.Client
	bucketName string
	prefix     string
	endpoint   string
	useSSL     bool
}

func NewS3Storage(u *url.URL) (*S3Storage, error) {
	endpoint := u.Host
	bucketName := ""
	prefix := ""

	pathParts := strings.SplitN(strings.TrimPrefix(u.Path, "/"), "/", 2)
	if len(pathParts) > 0 {
		bucketName = pathParts[0]
	}
	if len(pathParts) > 1 {
		prefix = pathParts[1]
	}

	if bucketName == "" {
		return nil, fmt.Errorf("S3/MinIO bucket name is required in URI path")
	}

	accessKey := u.User.Username()
	secretKey, _ := u.User.Password()

	useSSL := u.Query().Get("ssl") != "false"
	if u.Scheme == "minio" && !strings.Contains(endpoint, ":") {
		// Default MinIO port if not specified
		// endpoint = endpoint + ":9000"
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize S3 client: %w", err)
	}

	return &S3Storage{
		client:     client,
		bucketName: bucketName,
		prefix:     prefix,
		endpoint:   endpoint,
		useSSL:     useSSL,
	}, nil
}

func (s *S3Storage) Save(ctx context.Context, name string, r io.Reader) (string, error) {
	objectName := s.getObjectName(name)

	// We might need to handle large uploads, but for now we use PutObject
	// For better performance with unknown sizes, we might need to wrap r in a temporary file
	// or use a buffer. Minio-go can handle some of this.

	// Since we don't know the size, we set it to -1 (minio-go will use multipart upload)
	_, err := s.client.PutObject(ctx, s.bucketName, objectName, r, -1, minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload object to S3: %w", err)
	}

	scheme := "s3"
	if !s.useSSL {
		scheme = "http"
	} else {
		scheme = "https"
	}

	return fmt.Sprintf("%s://%s/%s/%s", scheme, s.endpoint, s.bucketName, objectName), nil
}

func (s *S3Storage) Open(ctx context.Context, name string) (io.ReadCloser, error) {
	objectName := s.getObjectName(name)
	return s.client.GetObject(ctx, s.bucketName, objectName, minio.GetObjectOptions{})
}

func (s *S3Storage) Delete(ctx context.Context, name string) error {
	objectName := s.getObjectName(name)
	return s.client.RemoveObject(ctx, s.bucketName, objectName, minio.RemoveObjectOptions{})
}

func (s *S3Storage) Location() string {
	return fmt.Sprintf("s3://%s/%s/%s", s.endpoint, s.bucketName, s.prefix)
}

func (s *S3Storage) PutMetadata(ctx context.Context, name string, data []byte) error {
	objectName := s.getObjectName(name)
	_, err := s.client.PutObject(ctx, s.bucketName, objectName, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: "application/json",
	})
	return err
}

func (s *S3Storage) GetMetadata(ctx context.Context, name string) ([]byte, error) {
	objectName := s.getObjectName(name)
	obj, err := s.client.GetObject(ctx, s.bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()
	return io.ReadAll(obj)
}

func (s *S3Storage) ListMetadata(ctx context.Context, prefix string) ([]string, error) {
	fullPrefix := s.getObjectName(prefix)

	objects := s.client.ListObjects(ctx, s.bucketName, minio.ListObjectsOptions{
		Prefix:    fullPrefix,
		Recursive: true,
	})

	var files []string
	for obj := range objects {
		if obj.Err != nil {
			return nil, obj.Err
		}
		// Strip the internal prefix to return relative names
		name := strings.TrimPrefix(obj.Key, s.prefix)
		name = strings.TrimPrefix(name, "/")
		files = append(files, name)
	}
	return files, nil
}

func (s *S3Storage) getObjectName(name string) string {
	if s.prefix == "" {
		return name
	}
	return strings.TrimSuffix(s.prefix, "/") + "/" + strings.TrimPrefix(name, "/")
}
