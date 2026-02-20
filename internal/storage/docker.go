package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lupppig/dbackup/internal/db"
)

type DockerStorage struct {
	containerName string
	remotePath    string
}

func NewDockerStorage(u *url.URL) (*DockerStorage, error) {
	if u.Host == "" {
		return nil, fmt.Errorf("missing container name in docker URI")
	}
	return &DockerStorage{
		containerName: u.Host,
		remotePath:    u.Path,
	}, nil
}

func (s *DockerStorage) Save(ctx context.Context, name string, r io.Reader) (string, error) {
	path := filepath.Join(s.remotePath, name)
	// Ensure directory exists (safe exec)
	mkdirCmd := exec.CommandContext(ctx, "docker", "exec", s.containerName, "mkdir", "-p", filepath.Dir(path))
	_ = mkdirCmd.Run() // Ignore errors if directory exists or mkdir fails (cp will fail anyway if truly bad)

	// Stream to container using 'docker cp -'
	cmd := exec.CommandContext(ctx, "docker", "cp", "-", fmt.Sprintf("%s:%s", s.containerName, path))
	cmd.Stdin = r
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker save failed: %w", err)
	}

	return "docker://" + s.containerName + path, nil
}

func (s *DockerStorage) Open(ctx context.Context, name string) (io.ReadCloser, error) {
	path := filepath.Join(s.remotePath, name)
	cmd := exec.CommandContext(ctx, "docker", "exec", "-i", s.containerName, "cat", path)
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = os.Stderr

	go func() {
		err := cmd.Run()
		pw.CloseWithError(err)
	}()

	return pr, nil
}

func (s *DockerStorage) Exists(ctx context.Context, name string) (bool, error) {
	target := filepath.Join(s.remotePath, name)
	args := []string{"exec", s.containerName, "stat", target}
	cmd := exec.CommandContext(ctx, "docker", args...)
	if err := cmd.Run(); err != nil {
		return false, nil
	}
	return true, nil
}

func (s *DockerStorage) Delete(ctx context.Context, name string) error {
	path := filepath.Join(s.remotePath, name)
	cmd := exec.CommandContext(ctx, "docker", "exec", s.containerName, "rm", path)
	return cmd.Run()
}

func (s *DockerStorage) Location() string {
	return "docker://" + s.containerName + s.remotePath
}

func (s *DockerStorage) PutMetadata(ctx context.Context, name string, data []byte) error {
	path := filepath.Join(s.remotePath, name)
	cmd := exec.CommandContext(ctx, "docker", "cp", "-", fmt.Sprintf("%s:%s", s.containerName, path))
	cmd.Stdin = bytes.NewReader(data)
	return cmd.Run()
}

func (s *DockerStorage) GetMetadata(ctx context.Context, name string) ([]byte, error) {
	path := filepath.Join(s.remotePath, name)
	cmd := exec.CommandContext(ctx, "docker", "exec", s.containerName, "cat", path)
	return cmd.Output()
}

func (s *DockerStorage) ListMetadata(ctx context.Context, prefix string) ([]string, error) {
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

	cmd := exec.CommandContext(ctx, "docker", "exec", s.containerName, "ls", "-1", searchDir)
	out, err := cmd.Output()
	if err != nil {
		return nil, nil // Assume dir doesn't exist
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var files []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if basePrefix == "" || strings.HasPrefix(line, basePrefix) {
			relDir := ""
			if strings.Contains(prefix, "/") {
				if strings.HasSuffix(prefix, "/") {
					relDir = prefix
				} else {
					relDir = filepath.Dir(prefix) + "/"
				}
			}
			files = append(files, relDir+line)
		}
	}
	return files, nil
}

func (s *DockerStorage) Close() error {
	return nil
}

// Runner implementation

func (s *DockerStorage) Run(ctx context.Context, name string, args []string, w io.Writer) error {
	return s.RunWithIO(ctx, name, args, nil, w)
}

func (s *DockerStorage) RunWithIO(ctx context.Context, name string, args []string, r io.Reader, w io.Writer) error {
	dockerArgs := []string{"exec"}
	if r != nil {
		dockerArgs = append(dockerArgs, "-i")
	}
	dockerArgs = append(dockerArgs, s.containerName, name)
	dockerArgs = append(dockerArgs, args...)

	cmd := exec.CommandContext(ctx, "docker", dockerArgs...)
	cmd.Stdout = w
	cmd.Stdin = r
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

var _ db.Runner = (*DockerStorage)(nil)
