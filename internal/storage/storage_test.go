package storage

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFromURI_Inference(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		wantErr bool
	}{
		{"Empty URI", "", false},
		{"Invalid Scheme", "invalid://path", true},
		{"Malformed URI", "sftp://[invalid-host", true},
		{"S3 Shorthand", "s3:bucket", false},
		{"GS Shorthand", "gs:bucket", false},
		{"SFTP Shorthand", "user@host:path", false},
		{"Docker Shorthand", "docker:container:path", false},
		{"FTP (Blocked by default)", "ftp://user:pass@host/path", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := FromURI(tt.uri, StorageOptions{})
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}

	t.Run("FTP Allowed with flag", func(t *testing.T) {
		_, err := FromURI("ftp://user:pass@host/path", StorageOptions{AllowInsecure: true})
		// Note: Dial might still fail if host is invalid, but it shouldn't be the "explicit opt-in" error.
		if err != nil && strings.Contains(err.Error(), "requires explicit opt-in") {
			t.Errorf("FTP should be allowed with AllowInsecure flag")
		}
	})
}

func TestScrub(t *testing.T) {
	assert.Equal(t, "sftp://user:********@host/path", Scrub("sftp://user:password@host/path"))
	assert.Equal(t, "local://path", Scrub("local://path"))
}
