package pipeline

import "context"

type WorkHandler func(ctx context.Context, msg WorkMsg) error
type ResultHandler func(ctx context.Context, msg ResultMsg) error

type ISubscriber interface {
	SubscribeWork(ctx context.Context, subject, group string, h WorkHandler) error
	SubscribeResult(ctx context.Context, subject, group string, h ResultHandler) error
}
