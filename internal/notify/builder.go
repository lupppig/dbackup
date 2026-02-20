package notify

import (
	"github.com/lupppig/dbackup/internal/config"
)

func BuildNotifier(cfg *config.Config) Notifier {
	var notifiers []Notifier

	// Slack from config
	if cfg.Notifications.Slack.WebhookURL != "" {
		notifiers = append(notifiers, NewSlackNotifier(cfg.Notifications.Slack.WebhookURL, cfg.Notifications.Slack.Template))
	}

	// Generic Webhooks from config
	for _, w := range cfg.Notifications.Webhooks {
		if w.URL != "" {
			notifiers = append(notifiers, NewWebhookNotifier(w.URL, w.Method, w.Template, w.Headers))
		}
	}

	if len(notifiers) == 0 {
		return nil
	}
	if len(notifiers) == 1 {
		return notifiers[0]
	}
	return &MultiNotifier{Notifiers: notifiers}
}
