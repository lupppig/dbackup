package notify

import "context"

type Notify interface {
	Send(ctx context.Context, message string) error
}

type SlackNotifier struct {
	WebhookURL string
}

func (s *SlackNotifier) Send(ctx context.Context, message string) error {
	return nil
}
