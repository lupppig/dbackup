package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strings"
)

type StorageOptions struct {
	AllowInsecure bool
}

func FromURI(uriStr string, opts StorageOptions) (Storage, error) {
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
	case "ssh", "sftp":
		return NewSSHStorage(u)
	case "ftp":
		return NewFTPStorage(u, opts)
	case "docker":
		return NewDockerStorage(u)
	case "dedupe":
		wrapped, err := FromURI(u.Query().Get("target"), opts)
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
	if u.User != nil {
		if _, ok := u.User.Password(); ok {
			// Manually construct the string to avoid URL encoding of the mask
			userStr := u.User.Username() + ":********"
			if u.Host != "" {
				userStr += "@" + u.Host
			}
			res := u.Scheme + "://" + userStr + u.Path
			if u.RawQuery != "" {
				res += "?" + u.RawQuery
			}
			return res
		}
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
