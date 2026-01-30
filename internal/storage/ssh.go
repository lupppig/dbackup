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
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type SSHStorage struct {
	client     *ssh.Client
	sftpClient *sftp.Client
	remotePath string
	host       string
}

func NewSSHStorage(u *url.URL) (*SSHStorage, error) {
	user := u.User.Username()
	pass, _ := u.User.Password()
	host := u.Host
	if !strings.Contains(host, ":") {
		host = host + ":22"
	}

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
					fmt.Fprintf(os.Stderr, "SSH: Using agent authentication (found %d keys in %s)\n", len(signers), authSock)
				} else {
					fmt.Fprintf(os.Stderr, "SSH: Agent found at %s but it contains no keys. Run 'ssh-add' to load keys.\n", authSock)
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
						fmt.Fprintf(os.Stderr, "SSH: Loaded key %s\n", keyPath)
					} else {
						// Log skip if it's a passphrase protected key we can't handle yet
						if strings.Contains(err.Error(), "passphrase") {
							fmt.Fprintf(os.Stderr, "Warning: SSH key %s is encrypted. Please add it to your ssh-agent (ssh-add) to use it with dbackup.\n", k)
						}
					}
				}
			}
		}
	}

	if len(config.Auth) == 0 {
		return nil, fmt.Errorf("no supported SSH authentication methods found (checked Agent, common keys, and password)")
	}

	client, err := ssh.Dial("tcp", host, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s via SSH: %w (check if the host is reachable and your SSH keys/passwords are correct)", host, err)
	}

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to create sftp client: %w", err)
	}

	return &SSHStorage{
		client:     client,
		sftpClient: sftpClient,
		remotePath: u.Path,
		host:       host,
	}, nil
}

func (s *SSHStorage) Save(ctx context.Context, name string, r io.Reader) (string, error) {
	path := filepath.Join(s.remotePath, name)
	if err := s.sftpClient.MkdirAll(filepath.Dir(path)); err != nil {
		return "", err
	}

	f, err := s.sftpClient.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return "", err
	}
	return "sftp://" + s.host + path, nil
}

func (s *SSHStorage) Open(ctx context.Context, name string) (io.ReadCloser, error) {
	return s.sftpClient.Open(filepath.Join(s.remotePath, name))
}

func (s *SSHStorage) Location() string {
	return "sftp://" + s.host + s.remotePath
}

func (s *SSHStorage) PutMetadata(ctx context.Context, name string, data []byte) error {
	path := filepath.Join(s.remotePath, name)
	if err := s.sftpClient.MkdirAll(filepath.Dir(path)); err != nil {
		return err
	}
	f, err := s.sftpClient.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

func (s *SSHStorage) GetMetadata(ctx context.Context, name string) ([]byte, error) {
	path := filepath.Join(s.remotePath, name)
	f, err := s.sftpClient.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func (s *SSHStorage) Close() error {
	s.sftpClient.Close()
	return s.client.Close()
}

// Runner implementation

func (s *SSHStorage) Run(ctx context.Context, name string, args []string, w io.Writer) error {
	return s.RunWithIO(ctx, name, args, nil, w)
}

func (s *SSHStorage) RunWithIO(ctx context.Context, name string, args []string, r io.Reader, w io.Writer) error {
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
