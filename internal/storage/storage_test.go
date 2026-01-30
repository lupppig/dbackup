package storage

import (
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := FromURI(tt.uri)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestScrub(t *testing.T) {
	assert.Equal(t, "sftp://user:********@host/path", Scrub("sftp://user:password@host/path"))
	assert.Equal(t, "local://path", Scrub("local://path"))
}
