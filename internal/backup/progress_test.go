package backup

import (
	"bytes"
	"testing"
)

func TestProgressReader_NilBar(t *testing.T) {
	data := []byte("hello world")
	r := bytes.NewReader(data)
	pr := NewProgressReader(r, nil)

	buf := make([]byte, len(data))
	n, err := pr.Read(buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != len(data) {
		t.Errorf("expected %d bytes, got %d", len(data), n)
	}
}

func TestProgressWriter_NilBar(t *testing.T) {
	var buf bytes.Buffer
	pw := NewProgressWriter(&buf, nil)

	data := []byte("hello world")
	n, err := pw.Write(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != len(data) {
		t.Errorf("expected %d bytes, got %d", len(data), n)
	}
	if buf.String() != string(data) {
		t.Errorf("expected %q, got %q", string(data), buf.String())
	}
}
