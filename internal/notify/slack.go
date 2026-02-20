package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type SlackNotifier struct {
	WebhookURL string
}

func NewSlackNotifier(url string) *SlackNotifier {
	return &SlackNotifier{WebhookURL: url}
}

type slackAttachment struct {
	Color  string `json:"color"`
	Title  string `json:"title"`
	Text   string `json:"text"`
	Fields []struct {
		Title string `json:"title"`
		Value string `json:"value"`
		Short bool   `json:"short"`
	} `json:"fields"`
	Footer string `json:"footer"`
	Ts     int64  `json:"ts"`
}

type slackPayload struct {
	Text        string            `json:"text"`
	Attachments []slackAttachment `json:"attachments"`
}

func (s *SlackNotifier) Notify(ctx context.Context, stats Stats) error {
	if s.WebhookURL == "" {
		return nil
	}

	color := "#36a64f"
	title := fmt.Sprintf("✅ %s Successful", stats.Operation)
	if stats.Status == StatusError {
		color = "#ff0000"
		title = fmt.Sprintf("❌ %s Failed", stats.Operation)
	}

	attachment := slackAttachment{
		Color:  color,
		Title:  title,
		Footer: "dbackup",
		Ts:     time.Now().Unix(),
	}

	attachment.Fields = []struct {
		Title string `json:"title"`
		Value string `json:"value"`
		Short bool   `json:"short"`
	}{
		{Title: "DB", Value: stats.Engine, Short: true},
		{Title: "Name", Value: stats.Database, Short: true},
		{Title: "File", Value: stats.FileName, Short: false},
		{Title: "Duration", Value: stats.Duration.String(), Short: true},
	}

	if stats.Size > 0 {
		attachment.Fields = append(attachment.Fields, struct {
			Title string `json:"title"`
			Value string `json:"value"`
			Short bool   `json:"short"`
		}{Title: "Size", Value: formatSize(stats.Size), Short: true})
	}

	if stats.Error != nil {
		attachment.Text = fmt.Sprintf("*Error:* %v", stats.Error)
	}

	payload := slackPayload{
		Attachments: []slackAttachment{attachment},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.WebhookURL, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack notification failed with status: %s", resp.Status)
	}

	return nil
}

func formatSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
