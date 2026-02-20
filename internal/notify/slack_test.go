package notify

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{500, "500 B"},
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
		{1048576, "1.00 MB"},
		{1048576 * 1024, "1.00 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, formatSize(tt.bytes))
		})
	}
}

func TestSlackNotifier_Notify_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var payload slackPayload
		err := json.NewDecoder(r.Body).Decode(&payload)
		assert.NoError(t, err)

		assert.Len(t, payload.Attachments, 1)
		att := payload.Attachments[0]
		assert.Equal(t, "#36a64f", att.Color)
		assert.Equal(t, "✅ Backup Successful", att.Title)
		assert.Len(t, att.Fields, 5) // DB, Name, File, Duration, Size

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier := NewSlackNotifier(server.URL)
	stats := Stats{
		Status:    StatusSuccess,
		Operation: "Backup",
		Engine:    "postgres",
		Database:  "testdb",
		FileName:  "test.sql.lz4",
		Duration:  5 * time.Second,
		Size:      1048576,
	}

	err := notifier.Notify(context.Background(), stats)
	assert.NoError(t, err)
}

func TestSlackNotifier_Notify_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload slackPayload
		json.NewDecoder(r.Body).Decode(&payload)

		att := payload.Attachments[0]
		assert.Equal(t, "#ff0000", att.Color)
		assert.Equal(t, "❌ Restore Failed", att.Title)
		assert.Contains(t, att.Text, "connection refused")

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier := NewSlackNotifier(server.URL)
	stats := Stats{
		Status:    StatusError,
		Operation: "Restore",
		Engine:    "mysql",
		Database:  "db1",
		FileName:  "backup.sql",
		Duration:  2 * time.Second,
		Error:     errors.New("connection refused"),
	}

	err := notifier.Notify(context.Background(), stats)
	assert.NoError(t, err)
}

func TestSlackNotifier_EmptyURL(t *testing.T) {
	notifier := NewSlackNotifier("")
	err := notifier.Notify(context.Background(), Stats{Operation: "Test"})
	assert.NoError(t, err) // Should silently return nil if no URL
}
