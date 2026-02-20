package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"text/template"
	"time"
)

type WebhookNotifier struct {
	URL      string
	Method   string
	Template string
	Headers  map[string]string
}

func NewWebhookNotifier(url, method, tmpl string, headers map[string]string) *WebhookNotifier {
	if method == "" {
		method = "POST"
	}
	return &WebhookNotifier{
		URL:      url,
		Method:   method,
		Template: tmpl,
		Headers:  headers,
	}
}

func (n *WebhookNotifier) Notify(ctx context.Context, stats Stats) error {
	if n.URL == "" {
		return nil
	}

	var body []byte
	var err error

	if n.Template != "" {
		body, err = n.renderTemplate(stats)
		if err != nil {
			return fmt.Errorf("failed to render webhook template: %w", err)
		}
	} else {
		// Default JSON payload
		body, _ = json.Marshal(stats)
	}

	req, err := http.NewRequestWithContext(ctx, n.Method, n.URL, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range n.Headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

func (n *WebhookNotifier) renderTemplate(stats Stats) ([]byte, error) {
	tmpl, err := template.New("webhook").Parse(n.Template)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	data := struct {
		Stats
		FormattedDuration string
	}{
		Stats:             stats,
		FormattedDuration: stats.Duration.Truncate(time.Second).String(),
	}

	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
