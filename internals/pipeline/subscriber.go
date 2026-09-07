package pipeline

import "context"

// WorkHandler handles a subscribed work message.
type WorkHandler func(ctx context.Context, msg WorkMsg) error

// ResultHandler handles a subscribed result message.
type ResultHandler func(ctx context.Context, msg ResultMsg) error

// ISubscriber consumes pipeline work and result messages.
type ISubscriber interface {
	// SubscribeWork consumes work messages from subject in group.
	SubscribeWork(ctx context.Context, subject, group string, h WorkHandler) error
	// SubscribeResult consumes result messages from subject in group.
	SubscribeResult(ctx context.Context, subject, group string, h ResultHandler) error
}
