package storage

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChunker_CDC_Deduplication(t *testing.T) {
	// Create common data (~5MB)
	commonData := bytes.Repeat([]byte("Some very common data that should be deduplicated. "), 100000)

	// Stream 1: header1 + commonData
	stream1 := append([]byte("Header version 1.0 (2026-01-01)\n"), commonData...)
	// Stream 2: header2 (different length!) + commonData
	stream2 := append([]byte("Header version 2.0.1 (2026-01-30 15:00:00)\n"), commonData...)

	chunks1 := collectChunks(t, stream1)
	chunks2 := collectChunks(t, stream2)

	assert.NotEmpty(t, chunks1)
	assert.NotEmpty(t, chunks2)

	// Compare chunks. After the first few header chunks, they should sync up.
	matches := 0
	foundHashes1 := make(map[string]bool)
	for _, c := range chunks1 {
		foundHashes1[string(c)] = true
	}

	for _, c := range chunks2 {
		if foundHashes1[string(c)] {
			matches++
		}
	}

	t.Logf("Stream 1 chunks: %d, Stream 2 chunks: %d, Matches: %d", len(chunks1), len(chunks2), matches)
	assert.Greater(t, matches, 0, "Should have found at least some matching chunks via CDC")
	assert.Greater(t, float64(matches)/float64(len(chunks1)), 0.8, "Deduplication ratio should be high (>80%)")
}

func collectChunks(t *testing.T, data []byte) [][]byte {
	chunker := NewChunker(bytes.NewReader(data))
	var chunks [][]byte
	for {
		chunk, err := chunker.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		chunks = append(chunks, chunk)
	}
	return chunks
}

func TestChunker_DataIntegrity(t *testing.T) {
	data := bytes.Repeat([]byte("random data "), 5000)
	chunker := NewChunker(bytes.NewReader(data))

	var reconstructed []byte
	for {
		chunk, err := chunker.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		reconstructed = append(reconstructed, chunk...)
	}

	assert.Equal(t, data, reconstructed)
}
