package compress

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectAlgorithm(t *testing.T) {
	tests := []struct {
		filename string
		expected Algorithm
	}{
		{"backup.sql.gz", Gzip},
		{"backup.lz4", Lz4},
		{"data.zst", Zstd},
		{"archive.tar", Tar},
		{"raw.sql", None},
		{"no_extension", None},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			assert.Equal(t, tt.expected, DetectAlgorithm(tt.filename))
		})
	}
}
