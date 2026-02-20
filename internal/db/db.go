package db

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/lupppig/dbackup/internal/logger"
)

type TLSConfig struct {
	Enabled    bool
	Mode       string
	CACert     string
	ClientCert string
	ClientKey  string
}

type ConnectionParams struct {
	DBType   string
	DBName   string
	Password string
	User     string
	Host     string
	Port     int
	DBUri    string

	TLS        TLSConfig
	IsPhysical bool
}

func (c *ConnectionParams) ParseURI() error {
	if c.DBUri == "" {
		return nil
	}
	u, err := url.Parse(c.DBUri)
	if err != nil {
		return err
	}

	if c.DBType == "" {
		c.DBType = u.Scheme
	}

	c.Host = u.Hostname()
	if p := u.Port(); p != "" {
		fmt.Sscanf(p, "%d", &c.Port)
	} else {
		switch u.Scheme {
		case "postgres", "postgresql":
			c.Port = 5432
			if c.DBType == "" || c.DBType == u.Scheme {
				c.DBType = "postgres"
			}
		case "mysql":
			c.Port = 3306
		}
	}

	if u.User != nil {
		c.User = u.User.Username()
		c.Password, _ = u.User.Password()
	}

	if c.DBType == "sqlite" {
		c.DBName = u.Path
		// If URI was sqlite://path/to/db, Path is path/to/db.
		// If URI was sqlite:///path/to/db, Path is /path/to/db.
		if u.Host != "" && !strings.HasPrefix(u.Path, "/") {
			c.DBName = u.Host + "/" + u.Path
		} else if u.Host != "" {
			c.DBName = u.Host + u.Path
		}
	} else {
		c.DBName = strings.TrimPrefix(u.Path, "/")
	}
	return nil
}

type BackUpOptions struct {
	Storage   string
	Compress  bool
	Algorithm string
	FileName  string
	OutputDir string
}

type Runner interface {
	Run(ctx context.Context, name string, args []string, w io.Writer) error
	RunWithIO(ctx context.Context, name string, args []string, r io.Reader, w io.Writer) error
}

type LocalRunner struct {
	logger *logger.Logger
}

func NewLocalRunner(l *logger.Logger) *LocalRunner {
	return &LocalRunner{logger: l}
}

func (r *LocalRunner) Run(ctx context.Context, name string, args []string, w io.Writer) error {
	return r.RunWithIO(ctx, name, args, nil, w)
}

func (r *LocalRunner) RunWithIO(ctx context.Context, name string, args []string, stdin io.Reader, stdout io.Writer) error {
	if r.logger != nil {
		r.logger.Debug("Executing command", "command", name, "args", strings.Join(args, " "))
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = stdout
	cmd.Stdin = stdin
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type DryRunRunner struct {
	logger *logger.Logger
}

func NewDryRunRunner(l *logger.Logger) *DryRunRunner {
	return &DryRunRunner{logger: l}
}

func (d *DryRunRunner) Run(ctx context.Context, name string, args []string, w io.Writer) error {
	return d.RunWithIO(ctx, name, args, nil, w)
}

func (d *DryRunRunner) RunWithIO(ctx context.Context, name string, args []string, stdin io.Reader, stdout io.Writer) error {
	if d.logger != nil {
		d.logger.Info("DRY RUN: would execute command", "command", name, "args", strings.Join(args, " "))
	}
	return nil
}

type DBAdapter interface {
	Name() string
	TestConnection(ctx context.Context, conn ConnectionParams, runner Runner) error
	BuildConnection(ctx context.Context, conn ConnectionParams) (string, error)
	RunBackup(ctx context.Context, conn ConnectionParams, runner Runner, w io.Writer) error
	RunRestore(ctx context.Context, conn ConnectionParams, runner Runner, r io.Reader) error
	SetLogger(l *logger.Logger)
}

var adapters = map[string]DBAdapter{}

func RegisterAdapter(adapter DBAdapter) {
	adapters[adapter.Name()] = adapter
}

func GetAdapter(name string) (DBAdapter, error) {
	adapter, ok := adapters[name]
	if !ok {
		return nil, fmt.Errorf("unsupported database: %s", name)
	}
	return adapter, nil
}
