package mongo

import (
	"context"
	"errors"
	"fmt"

	"github.com/amirdaaee/Glide/internals/domain"
	"github.com/amirdaaee/Glide/internals/repository"
	"github.com/chenmingyong0423/go-mongox/builder/query"
	"github.com/chenmingyong0423/go-mongox/builder/update"
	"github.com/chenmingyong0423/go-mongox/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type UserRepository struct {
	coll *mongox.Collection[domain.User]
}

var _ repository.IUserRepository = (*UserRepository)(nil)

func (r *UserRepository) GetByTelegramID(ctx context.Context, telegramID int64) (*domain.User, error) {
	user, err := r.coll.Finder().Filter(query.Eq("TelegramID", telegramID)).FindOne(ctx)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, repository.NotFoundError
		}
		return nil, fmt.Errorf("can not get user by telegram id: %w", err)
	}
	return user, nil
}

func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	if _, err := r.coll.Creator().InsertOne(ctx, user); err != nil {
		return fmt.Errorf("can not create user: %w", err)
	}
	return nil
}
func (r *UserRepository) AddMedia(ctx context.Context, userID, mediaID bson.ObjectID) error {
	updateVal := update.NewBuilder().
		Push("MediaList", mediaID).
		Build()
	if _, err := r.coll.Updater().Filter(query.Id(userID)).Updates(updateVal).UpdateOne(ctx); err != nil {
		return fmt.Errorf("can not add media to user: %w", err)
	}
	return nil
}
