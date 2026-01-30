package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strings"
)

func FromURI(uriStr string) (Storage, error) {
	if uriStr == "" {
		return NewLocalStorage(""), nil
	}

	if !strings.Contains(uriStr, "://") {
		if strings.Contains(uriStr, "@") {
			if strings.Contains(uriStr, ":") {
				parts := strings.SplitN(uriStr, ":", 2)
				uriStr = "sftp://" + parts[0] + "/" + strings.TrimPrefix(parts[1], "/")
			} else {
				uriStr = "sftp://" + uriStr
			}
		} else if strings.HasPrefix(uriStr, "docker:") {
			// Inferred Docker: docker:container[:path]
			trimmed := strings.TrimPrefix(uriStr, "docker:")
			if strings.Contains(trimmed, ":") {
				parts := strings.SplitN(trimmed, ":", 2)
				uriStr = "docker://" + parts[0] + "/" + strings.TrimPrefix(parts[1], "/")
			} else {
				uriStr = "docker://" + trimmed
			}
		} else if strings.HasPrefix(uriStr, "s3:") {
			uriStr = "s3://" + strings.TrimPrefix(uriStr, "s3:")
		} else if strings.HasPrefix(uriStr, "gs:") {
			uriStr = "gs://" + strings.TrimPrefix(uriStr, "gs:")
		}
	}

	if !strings.Contains(uriStr, "://") {
		return NewLocalStorage(uriStr), nil
	}

	u, err := url.Parse(uriStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse storage URI: %w", err)
	}

	switch u.Scheme {
	case "local", "file":
		path := u.Path
		if u.Host != "" {
			path = filepath.Join(u.Host, path)
		}
		return NewLocalStorage(path), nil
	case "sftp", "ssh":
		return NewSSHStorage(u)
	case "ftp":
		return NewFTPStorage(u)
	case "docker":
		return NewDockerStorage(u)
	case "s3":
		return &S3Storage{Bucket: u.Host, Region: u.Query().Get("region")}, nil
	case "gs":
		return &GCSStorage{Bucket: u.Host}, nil
	case "dedupe":
		wrapped, err := FromURI(u.Query().Get("target"))
		if err != nil {
			return nil, err
		}
		return NewDedupeStorage(wrapped), nil
	default:
		return nil, fmt.Errorf("unsupported storage scheme: %s", u.Scheme)
	}
}

// Scrub removes sensitive information from a URI for logging
func Scrub(uriStr string) string {
	u, err := url.Parse(uriStr)
	if err != nil {
		return uriStr
	}
	if _, ok := u.User.Password(); ok {
		u.User = url.UserPassword(u.User.Username(), "********")
	}
	return u.String()
}

type Storage interface {
	Save(ctx context.Context, name string, r io.Reader) (string, error)
	Open(ctx context.Context, name string) (io.ReadCloser, error)
	Location() string

	// Metadata support
	PutMetadata(ctx context.Context, name string, data []byte) error
	GetMetadata(ctx context.Context, name string) ([]byte, error)
}
