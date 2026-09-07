package mongo

import (
	"context"
	"fmt"
	"time"

	"github.com/amirdaaee/Glide/internals/domain"
	"github.com/chenmingyong0423/go-mongox/v2"
	"github.com/chenmingyong0423/go-mongox/v2/builder/query"
	"github.com/chenmingyong0423/go-mongox/v2/builder/update"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// StatusReplyRepository persists MediaStatusReply documents in MongoDB.
type StatusReplyRepository struct {
	coll *mongox.Collection[domain.MediaStatusReply]
}

var _ domain.IMediaStatusReplyRepository = (*StatusReplyRepository)(nil)

// Upsert creates or replaces the pending status reply for a media/user pair.
func (r *StatusReplyRepository) Upsert(ctx context.Context, reply *domain.MediaStatusReply) error {
	if reply == nil {
		return fmt.Errorf("status reply is nil")
	}
	if reply.MediaID.IsZero() || reply.UserID.IsZero() {
		return fmt.Errorf("media id and user id are required")
	}
	filter := query.NewBuilder().
		Eq("MediaID", reply.MediaID).
		Eq("UserID", reply.UserID).
		Build()
	updateVal := update.NewBuilder().
		Set("MediaID", reply.MediaID).
		Set("UserID", reply.UserID).
		Set("TelegramID", reply.TelegramID).
		Set("AccessHash", reply.AccessHash).
		Set("MessageID", reply.MessageID).
		Build()
	_, err := r.coll.Updater().Filter(filter).Updates(updateVal).UpdateOne(ctx, options.UpdateOne().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("can not upsert media status reply: %w", err)
	}
	return nil
}

// ListByMediaID returns pending status replies for a media file.
func (r *StatusReplyRepository) ListByMediaID(ctx context.Context, mediaID bson.ObjectID) ([]*domain.MediaStatusReply, error) {
	if mediaID.IsZero() {
		return nil, fmt.Errorf("media id is required")
	}
	replies, err := r.coll.Finder().Filter(query.Eq("MediaID", mediaID)).Find(ctx)
	if err != nil {
		return nil, fmt.Errorf("can not list media status replies: %w", err)
	}
	if replies == nil {
		return []*domain.MediaStatusReply{}, nil
	}
	return replies, nil
}

// DeleteByMediaID removes all pending status replies for a media file.
func (r *StatusReplyRepository) DeleteByMediaID(ctx context.Context, mediaID bson.ObjectID) error {
	if mediaID.IsZero() {
		return fmt.Errorf("media id is required")
	}
	if _, err := r.coll.Deleter().Filter(query.Eq("MediaID", mediaID)).DeleteMany(ctx); err != nil {
		return fmt.Errorf("can not delete media status replies: %w", err)
	}
	return nil
}

// Delete removes the pending status reply for a media/user pair.
func (r *StatusReplyRepository) Delete(ctx context.Context, mediaID, userID bson.ObjectID) error {
	if mediaID.IsZero() || userID.IsZero() {
		return fmt.Errorf("media id and user id are required")
	}
	filter := query.NewBuilder().
		Eq("MediaID", mediaID).
		Eq("UserID", userID).
		Build()
	if _, err := r.coll.Deleter().Filter(filter).DeleteOne(ctx); err != nil {
		return fmt.Errorf("can not delete media status reply: %w", err)
	}
	return nil
}

// ensureIndexes creates unique (MediaID, UserID) and MediaID indexes.
func (r *StatusReplyRepository) ensureIndexes(ctx context.Context) error {
	_, err := r.coll.Collection().Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "MediaID", Value: 1},
				{Key: "UserID", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "MediaID", Value: 1}},
		},
	})
	if err != nil {
		return fmt.Errorf("can not create status reply indexes: %w", err)
	}
	return nil
}

// NewStatusReplyRepository returns a MongoDB status-reply repository.
func NewStatusReplyRepository(db *mongox.Database, name string) (domain.IMediaStatusReplyRepository, error) {
	r := &StatusReplyRepository{
		coll: mongox.NewCollection[domain.MediaStatusReply](db, name),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := r.ensureIndexes(ctx); err != nil {
		return nil, err
	}
	return r, nil
}
