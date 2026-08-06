package mongo

import (
	"context"
	"errors"
	"fmt"

	"github.com/amirdaaee/Glide/internals/domain"
	"github.com/amirdaaee/Glide/internals/repository"
	"github.com/chenmingyong0423/go-mongox/v2"
	"github.com/chenmingyong0423/go-mongox/v2/builder/query"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type MediaRepository struct {
	coll *mongox.Collection[domain.MediaFile]
}

var _ repository.IMediaRepository = (*MediaRepository)(nil)

func (r *MediaRepository) Create(ctx context.Context, media *domain.MediaFile) error {
	if _, err := r.coll.Creator().InsertOne(ctx, media); err != nil {
		return fmt.Errorf("can not create media file: %w", err)
	}
	// TODO: is id applied?
	return nil
}

func (r *MediaRepository) GetByFID(ctx context.Context, fid int64) (*domain.MediaFile, error) {
	media, err := r.coll.Finder().Filter(query.Eq("Meta.FileID", fid)).FindOne(ctx)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, repository.NotFoundError
		}
		return nil, fmt.Errorf("can not get media file by fid: %w", err)
	}
	return media, nil
}

func (r *MediaRepository) GetMany(ctx context.Context, ids []bson.ObjectID) ([]*domain.MediaFile, error) {
	if len(ids) == 0 {
		return []*domain.MediaFile{}, nil
	}
	media, err := r.coll.Finder().Filter(query.In("_id", ids...)).Find(ctx)
	if err != nil {
		return nil, fmt.Errorf("can not get media files: %w", err)
	}
	return media, nil
}

func (r *MediaRepository) Delete(ctx context.Context, fid int64) error {
	if _, err := r.coll.Deleter().Filter(query.Eq("Meta.FileID", fid)).DeleteOne(ctx); err != nil {
		return fmt.Errorf("can not delete media file: %w", err)
	}
	return nil
}

func NewMediaRepository(db *mongox.Collection[domain.MediaFile]) repository.IMediaRepository {
	return &MediaRepository{coll: db}
}
