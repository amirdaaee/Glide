package mongo

import (
	"context"
	"errors"
	"fmt"

	"github.com/amirdaaee/Glide/internals/domain"
	"github.com/amirdaaee/Glide/internals/repository"
	"github.com/chenmingyong0423/go-mongox/builder/query"
	"github.com/chenmingyong0423/go-mongox/v2"
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

func NewMediaRepository(db *mongox.Collection[domain.MediaFile]) repository.IMediaRepository {
	return &MediaRepository{coll: db}
}
