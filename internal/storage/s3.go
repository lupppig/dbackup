package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strings"

	"time"

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

	transport, err := minio.DefaultTransport(useSSL)
	if err == nil {
		transport.DialContext = (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext
		transport.ResponseHeaderTimeout = 10 * time.Second
		transport.IdleConnTimeout = 30 * time.Second
	}

	region := u.Query().Get("region")
	if region == "" {
		region = "us-east-1"
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure:       useSSL,
		Transport:    transport,
		Region:       region,
		BucketLookup: minio.BucketLookupPath, // Force path-style to avoid location probes
	})
	if err == nil {
		client.SetAppInfo("dbackup", "1.0.0")
		// Note: minio-go doesn't have a simple MaxRetries option in minio.Options,
		// but using a fixed region and our custom transport with DialTimeout handles it.
	}
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

	var size int64 = -1
	var readerToUpload io.Reader = r

	// Try to determine size if possible
	switch v := r.(type) {
	case *bytes.Buffer:
		size = int64(v.Len())
	case *bytes.Reader:
		size = int64(v.Len())
	case *strings.Reader:
		size = int64(v.Len())
	case *os.File:
		if fi, err := v.Stat(); err == nil {
			size = fi.Size()
		}
	}

	// If size is unknown, buffer to a temporary file to ensure known size
	// and avoid high memory pressure from minio-go's internal buffering.
	if size == -1 {
		tmpFile, err := os.CreateTemp("", "dbackup-s3-upload-*")
		if err != nil {
			return "", fmt.Errorf("failed to create temporary file for S3 upload: %w", err)
		}
		defer os.Remove(tmpFile.Name())
		defer tmpFile.Close()

		size, err = io.Copy(tmpFile, r)
		if err != nil {
			return "", fmt.Errorf("failed to buffer stream to temporary file: %w", err)
		}

		if _, err := tmpFile.Seek(0, 0); err != nil {
			return "", fmt.Errorf("failed to seek to start of temporary file: %w", err)
		}
		readerToUpload = tmpFile
	}

	_, err := s.client.PutObject(ctx, s.bucketName, objectName, readerToUpload, size, minio.PutObjectOptions{
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

		// Optimization: skip listing chunks unless specifically requested
		if strings.HasPrefix(name, "chunks/") && !strings.HasPrefix(prefix, "chunks/") {
			continue
		}

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
