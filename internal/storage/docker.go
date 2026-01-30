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
	// Ensure directory exists
	mkdirCmd := exec.CommandContext(ctx, "docker", "exec", s.containerName, "mkdir", "-p", filepath.Dir(path))
	if err := mkdirCmd.Run(); err != nil {
		return "", fmt.Errorf("docker mkdir failed: %w", err)
	}

	// Stream to container
	cmd := exec.CommandContext(ctx, "docker", "exec", "-i", s.containerName, "sh", "-c", fmt.Sprintf("cat > %s", path))
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

func (s *DockerStorage) Location() string {
	return "docker://" + s.containerName + s.remotePath
}

func (s *DockerStorage) PutMetadata(ctx context.Context, name string, data []byte) error {
	path := filepath.Join(s.remotePath, name)
	cmd := exec.CommandContext(ctx, "docker", "exec", "-i", s.containerName, "sh", "-c", fmt.Sprintf("cat > %s", path))
	cmd.Stdin = bytes.NewReader(data)
	return cmd.Run()
}

func (s *DockerStorage) GetMetadata(ctx context.Context, name string) ([]byte, error) {
	path := filepath.Join(s.remotePath, name)
	cmd := exec.CommandContext(ctx, "docker", "exec", s.containerName, "cat", path)
	return cmd.Output()
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
