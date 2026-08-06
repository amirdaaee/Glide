package repository

import (
	"context"

	"github.com/amirdaaee/Glide/internals/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type IUserRepository interface {
	GetByTelegramID(ctx context.Context, telegramID int64) (*domain.User, error)
	Create(ctx context.Context, user *domain.User) error
	UpsertByTelegramID(ctx context.Context, user *domain.User) error
	AddMedia(ctx context.Context, userID, mediaID bson.ObjectID) error
	ListMedia(ctx context.Context, userID bson.ObjectID) ([]bson.ObjectID, error)
}
