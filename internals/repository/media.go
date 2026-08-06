package repository

import (
	"context"

	"github.com/amirdaaee/Glide/internals/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type IMediaRepository interface {
	Create(ctx context.Context, media *domain.MediaFile) error
	GetByFID(ctx context.Context, fid int64) (*domain.MediaFile, error)
	GetMany(ctx context.Context, ids []bson.ObjectID) ([]*domain.MediaFile, error)
	Delete(ctx context.Context, fid int64) error
}
