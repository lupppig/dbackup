package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func executeCommand(root *cobra.Command, args ...string) (output string, err error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)

	err = root.Execute()
	return buf.String(), err
}

func TestBackupCommand_Flags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "Missing Database Engine",
			args:    []string{"backup"},
			wantErr: true,
		},
		{
			name:    "Missing Required Fields (Non-SQLite)",
			args:    []string{"backup", "--db", "postgres"},
			wantErr: true,
		},
		{
			name:    "DBUri with Host (Conflict)",
			args:    []string{"backup", "--db-uri", "postgres://...", "--host", "localhost"},
			wantErr: true,
		},
		{
			name:    "TLS Enabled without Mode (Defaults but checked in RunE)",
			args:    []string{"backup", "--db", "postgres", "--host", "h", "--user", "u", "--password", "p", "--dbname", "d", "--tls", "--tls-mode", "disable"},
			wantErr: true,
		},
		{
			name:    "TLS Client Cert without Key",
			args:    []string{"backup", "--db", "postgres", "--host", "h", "--user", "u", "--password", "p", "--dbname", "d", "--tls-client-cert", "cert.pem"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := executeCommand(rootCmd, tt.args...)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
