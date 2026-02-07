package storage

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/lupppig/dbackup/internal/db"
	apperrors "github.com/lupppig/dbackup/internal/errors"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type SSHStorage struct {
	client     *ssh.Client
	sftpClient *sftp.Client
	remotePath string
	host       string
	user       *url.Userinfo
}

func NewSSHStorage(u *url.URL) (*SSHStorage, error) {
	host := u.Host
	if !strings.Contains(host, ":") {
		host = host + ":22"
	}

	remotePath := u.Path
	remotePath = strings.TrimPrefix(remotePath, "/./")

	return &SSHStorage{
		remotePath: remotePath,
		host:       host,
		user:       u.User,
	}, nil
}

func (s *SSHStorage) connect() error {
	if s.sftpClient != nil {
		return nil
	}

	user := s.user.Username()
	pass, _ := s.user.Password()

	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	if pass != "" {
		config.Auth = append(config.Auth, ssh.Password(pass))
	} else {
		// 1. Try SSH Agent
		if authSock := os.Getenv("SSH_AUTH_SOCK"); authSock != "" {
			if conn, err := net.Dial("unix", authSock); err == nil {
				ag := agent.NewClient(conn)
				signers, err := ag.Signers()
				if err == nil && len(signers) > 0 {
					config.Auth = append(config.Auth, ssh.PublicKeysCallback(ag.Signers))
				}
			}
		}

		// 2. Try common private keys
		home, err := os.UserHomeDir()
		if err == nil {
			commonKeys := []string{"id_rsa", "id_ed25519", "id_ecdsa"}
			for _, k := range commonKeys {
				keyPath := filepath.Join(home, ".ssh", k)
				if key, err := os.ReadFile(keyPath); err == nil {
					signer, err := ssh.ParsePrivateKey(key)
					if err == nil {
						config.Auth = append(config.Auth, ssh.PublicKeys(signer))
					}
				}
			}
		}
	}

	if len(config.Auth) == 0 {
		return apperrors.New(apperrors.TypeAuth, "no supported SSH authentication methods found", "Ensure you have an SSH agent running or provide valid private keys/passwords.")
	}

	client, err := ssh.Dial("tcp", s.host, config)
	if err != nil {
		return apperrors.Wrap(err, apperrors.TypeConnection, "failed to connect via SSH", "Check host reachability, SSH port, and credentials.")
	}

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		client.Close()
		return apperrors.Wrap(err, apperrors.TypeInternal, "failed to create SFTP client", "Verify the SFTP subsystem is enabled on the remote host.")
	}

	s.client = client
	s.sftpClient = sftpClient
	return nil
}

func (s *SSHStorage) Save(ctx context.Context, name string, r io.Reader) (string, error) {
	if err := s.connect(); err != nil {
		return "", err
	}
	path := filepath.Join(s.remotePath, name)
	if err := s.sftpClient.MkdirAll(filepath.Dir(path)); err != nil {
		return "", fmt.Errorf("failed to create remote directory %s: %w", filepath.Dir(path), err)
	}

	f, err := s.sftpClient.Create(path)
	if err != nil {
		return "", fmt.Errorf("failed to create remote file %s: %w", path, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return "", err
	}
	return "sftp://" + s.host + path, nil
}

func (s *SSHStorage) Open(ctx context.Context, name string) (io.ReadCloser, error) {
	if err := s.connect(); err != nil {
		return nil, err
	}
	return s.sftpClient.Open(filepath.Join(s.remotePath, name))
}

func (s *SSHStorage) Delete(ctx context.Context, name string) error {
	if err := s.connect(); err != nil {
		return err
	}
	return s.sftpClient.Remove(filepath.Join(s.remotePath, name))
}

func (s *SSHStorage) Location() string {
	return "sftp://" + s.host + s.remotePath
}

func (s *SSHStorage) PutMetadata(ctx context.Context, name string, data []byte) error {
	if err := s.connect(); err != nil {
		return err
	}
	path := filepath.Join(s.remotePath, name)
	if err := s.sftpClient.MkdirAll(filepath.Dir(path)); err != nil {
		return fmt.Errorf("failed to create remote directory %s: %w", filepath.Dir(path), err)
	}
	f, err := s.sftpClient.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create remote file %s: %w", path, err)
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

func (s *SSHStorage) GetMetadata(ctx context.Context, name string) ([]byte, error) {
	if err := s.connect(); err != nil {
		return nil, err
	}
	path := filepath.Join(s.remotePath, name)
	f, err := s.sftpClient.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func (s *SSHStorage) ListMetadata(ctx context.Context, prefix string) ([]string, error) {
	if err := s.connect(); err != nil {
		return nil, err
	}
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

	entries, err := s.sftpClient.ReadDir(searchDir)
	if err != nil {
		return nil, nil // Assume dir doesn't exist
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && (basePrefix == "" || strings.HasPrefix(entry.Name(), basePrefix)) {
			relDir := ""
			if strings.Contains(prefix, "/") {
				if strings.HasSuffix(prefix, "/") {
					relDir = prefix
				} else {
					relDir = filepath.Dir(prefix) + "/"
				}
			}
			files = append(files, relDir+entry.Name())
		}
	}
	return files, nil
}

func (s *SSHStorage) Close() error {
	if s.sftpClient != nil {
		s.sftpClient.Close()
	}
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

// Runner implementation

func (s *SSHStorage) Run(ctx context.Context, name string, args []string, w io.Writer) error {
	return s.RunWithIO(ctx, name, args, nil, w)
}

func (s *SSHStorage) RunWithIO(ctx context.Context, name string, args []string, r io.Reader, w io.Writer) error {
	if err := s.connect(); err != nil {
		return err
	}
	session, err := s.client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	if r != nil {
		session.Stdin = r
	}
	if w != nil {
		session.Stdout = w
	}
	session.Stderr = os.Stderr

	// Properly escape arguments for the shell
	escapedArgs := make([]string, len(args))
	for i, arg := range args {
		// Escape single quotes and wrap in single quotes: 'arg' -> ''\''arg'\'''
		escapedArgs[i] = "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
	}

	cmd := name + " " + strings.Join(escapedArgs, " ")
	return session.Run(cmd)
}

var _ db.Runner = (*SSHStorage)(nil)
