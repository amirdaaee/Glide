package pipeline

import "context"

type IPublisher interface {
	PublishWork(ctx context.Context, subject string, msg WorkMsg) error
	PublishResult(ctx context.Context, subject string, msg ResultMsg) error
}
