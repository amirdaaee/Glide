package pipeline

import "context"

// IPublisher publishes pipeline work and result messages.
type IPublisher interface {
	// PublishWork publishes a work message to subject.
	PublishWork(ctx context.Context, subject string, msg WorkMsg) error
	// PublishResult publishes a result message to subject.
	PublishResult(ctx context.Context, subject string, msg ResultMsg) error
}
