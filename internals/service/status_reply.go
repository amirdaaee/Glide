package service

import (
	"context"
	"fmt"

	"github.com/amirdaaee/Glide/internals/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type IStatusReplyService interface {
	Upsert(ctx context.Context, reply *domain.MediaStatusReply) error
	ListByMediaID(ctx context.Context, mediaID bson.ObjectID) ([]*domain.MediaStatusReply, error)
	DeleteByMediaID(ctx context.Context, mediaID bson.ObjectID) error
	Delete(ctx context.Context, mediaID, userID bson.ObjectID) error
}

type StatusReplyService struct {
	replies domain.IMediaStatusReplyRepository
}

var _ IStatusReplyService = (*StatusReplyService)(nil)

func (s *StatusReplyService) Upsert(ctx context.Context, reply *domain.MediaStatusReply) error {
	if err := s.replies.Upsert(ctx, reply); err != nil {
		return fmt.Errorf("can not upsert media status reply: %w", err)
	}
	return nil
}

func (s *StatusReplyService) ListByMediaID(ctx context.Context, mediaID bson.ObjectID) ([]*domain.MediaStatusReply, error) {
	replies, err := s.replies.ListByMediaID(ctx, mediaID)
	if err != nil {
		return nil, fmt.Errorf("can not list media status replies: %w", err)
	}
	return replies, nil
}

func (s *StatusReplyService) DeleteByMediaID(ctx context.Context, mediaID bson.ObjectID) error {
	if err := s.replies.DeleteByMediaID(ctx, mediaID); err != nil {
		return fmt.Errorf("can not delete media status replies: %w", err)
	}
	return nil
}

func (s *StatusReplyService) Delete(ctx context.Context, mediaID, userID bson.ObjectID) error {
	if err := s.replies.Delete(ctx, mediaID, userID); err != nil {
		return fmt.Errorf("can not delete media status reply: %w", err)
	}
	return nil
}

func NewStatusReplyService(replies domain.IMediaStatusReplyRepository) IStatusReplyService {
	return &StatusReplyService{replies: replies}
}
