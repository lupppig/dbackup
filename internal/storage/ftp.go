package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/jlaffaye/ftp"
)

type FTPStorage struct {
	client     *ftp.ServerConn
	remotePath string
	host       string
}

func NewFTPStorage(u *url.URL, opts StorageOptions) (*FTPStorage, error) {
	if !opts.AllowInsecure {
		return nil, fmt.Errorf("insecure protocol FTP requires explicit opt-in with --allow-insecure")
	}
	user := u.User.Username()
	pass, _ := u.User.Password()
	host := u.Host
	if !strings.Contains(host, ":") {
		host = host + ":21"
	}

	c, err := ftp.Dial(host, ftp.DialWithTimeout(5*time.Second))
	if err != nil {
		return nil, err
	}

	err = c.Login(user, pass)
	if err != nil {
		c.Quit()
		return nil, err
	}

	return &FTPStorage{
		client:     c,
		remotePath: u.Path,
		host:       host,
	}, nil
}

func (s *FTPStorage) Save(ctx context.Context, name string, r io.Reader) (string, error) {
	path := filepath.Join(s.remotePath, name)
	if err := s.ensureDir(filepath.Dir(path)); err != nil {
		return "", err
	}
	err := s.client.Stor(path, r)
	if err != nil {
		return "", err
	}
	return "ftp://" + s.host + path, nil
}

func (s *FTPStorage) Open(ctx context.Context, name string) (io.ReadCloser, error) {
	return s.client.Retr(filepath.Join(s.remotePath, name))
}

func (s *FTPStorage) Exists(ctx context.Context, name string) (bool, error) {
	target := filepath.Join(s.remotePath, name)
	_, err := s.client.FileSize(target)
	if err == nil {
		return true, nil
	}
	return false, nil
}

func (s *FTPStorage) Delete(ctx context.Context, name string) error {
	return s.client.Delete(filepath.Join(s.remotePath, name))
}

func (s *FTPStorage) Location() string {
	return "ftp://" + s.host + s.remotePath
}

func (s *FTPStorage) PutMetadata(ctx context.Context, name string, data []byte) error {
	path := filepath.Join(s.remotePath, name)
	if err := s.ensureDir(filepath.Dir(path)); err != nil {
		return err
	}
	return s.client.Stor(path, bytes.NewReader(data))
}

func (s *FTPStorage) GetMetadata(ctx context.Context, name string) ([]byte, error) {
	path := filepath.Join(s.remotePath, name)
	r, err := s.client.Retr(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func (s *FTPStorage) ListMetadata(ctx context.Context, prefix string) ([]string, error) {
	searchDir := s.remotePath
	basePrefix := prefix

	if strings.Contains(prefix, "/") {
		if strings.HasSuffix(prefix, "/") {
			searchDir = filepath.Join(s.remotePath, prefix)
			basePrefix = ""
		} else {
			searchDir = filepath.Join(s.remotePath, filepath.Dir(prefix))
			basePrefix = filepath.Base(prefix)
		}
	}

	entries, err := s.client.NameList(searchDir)
	if err != nil {
		return nil, nil // Assume dir doesn't exist
	}

	var files []string
	for _, entry := range entries {
		name := filepath.Base(entry)
		if basePrefix == "" || strings.HasPrefix(name, basePrefix) {
			relDir := ""
			if strings.Contains(prefix, "/") {
				if strings.HasSuffix(prefix, "/") {
					relDir = prefix
				} else {
					relDir = filepath.Dir(prefix) + "/"
				}
			}
			files = append(files, relDir+name)
		}
	}
	return files, nil
}

func (s *FTPStorage) ensureDir(path string) error {
	if path == "." || path == "/" {
		return nil
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	current := ""
	if strings.HasPrefix(path, "/") {
		current = "/"
	}
	for _, part := range parts {
		current = filepath.Join(current, part)
		_ = s.client.MakeDir(current) // Ignore error if it already exists
	}
	return nil
}

func (s *FTPStorage) Close() error {
	return s.client.Quit()
}
