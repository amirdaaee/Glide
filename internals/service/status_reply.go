package service

import (
	"context"
	"fmt"

	"github.com/amirdaaee/Glide/internals/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// IStatusReplyService manages pending Telegram status-reply messages.
type IStatusReplyService interface {
	// Upsert creates or replaces the pending status reply for a media/user pair.
	Upsert(ctx context.Context, reply *domain.MediaStatusReply) error
	// ListByMediaID returns pending status replies for a media file.
	ListByMediaID(ctx context.Context, mediaID bson.ObjectID) ([]*domain.MediaStatusReply, error)
	// DeleteByMediaID removes all pending status replies for a media file.
	DeleteByMediaID(ctx context.Context, mediaID bson.ObjectID) error
	// Delete removes the pending status reply for a media/user pair.
	Delete(ctx context.Context, mediaID, userID bson.ObjectID) error
}

// StatusReplyService implements IStatusReplyService.
type StatusReplyService struct {
	replies domain.IMediaStatusReplyRepository
}

var _ IStatusReplyService = (*StatusReplyService)(nil)

// Upsert creates or replaces the pending status reply for a media/user pair.
func (s *StatusReplyService) Upsert(ctx context.Context, reply *domain.MediaStatusReply) error {
	if err := s.replies.Upsert(ctx, reply); err != nil {
		return fmt.Errorf("can not upsert media status reply: %w", err)
	}
	return nil
}

// ListByMediaID returns pending status replies for a media file.
func (s *StatusReplyService) ListByMediaID(ctx context.Context, mediaID bson.ObjectID) ([]*domain.MediaStatusReply, error) {
	replies, err := s.replies.ListByMediaID(ctx, mediaID)
	if err != nil {
		return nil, fmt.Errorf("can not list media status replies: %w", err)
	}
	return replies, nil
}

// DeleteByMediaID removes all pending status replies for a media file.
func (s *StatusReplyService) DeleteByMediaID(ctx context.Context, mediaID bson.ObjectID) error {
	if err := s.replies.DeleteByMediaID(ctx, mediaID); err != nil {
		return fmt.Errorf("can not delete media status replies: %w", err)
	}
	return nil
}

// Delete removes the pending status reply for a media/user pair.
func (s *StatusReplyService) Delete(ctx context.Context, mediaID, userID bson.ObjectID) error {
	if err := s.replies.Delete(ctx, mediaID, userID); err != nil {
		return fmt.Errorf("can not delete media status reply: %w", err)
	}
	return nil
}

// NewStatusReplyService returns an IStatusReplyService backed by replies.
func NewStatusReplyService(replies domain.IMediaStatusReplyRepository) IStatusReplyService {
	return &StatusReplyService{replies: replies}
}
