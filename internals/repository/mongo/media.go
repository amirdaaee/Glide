package mongo

import (
	"context"
	"fmt"

	"github.com/amirdaaee/Glide/internals/domain"
	"github.com/amirdaaee/Glide/internals/repository"
	"github.com/chenmingyong0423/go-mongox/v2"
)

type MediaRepository struct {
	coll *mongox.Collection[domain.MediaFile]
}

var _ repository.IMediaRepository = (*MediaRepository)(nil)

func (r *MediaRepository) Create(ctx context.Context, media *domain.MediaFile) (*domain.MediaFile, error) {
	_, err := r.coll.Creator().InsertOne(ctx, media)
	if err != nil {
		return nil, fmt.Errorf("can not create media file: %w", err)
	}
	// TODO: is id applied?
	return media, nil
}

func NewMediaRepository(db *mongox.Collection[domain.MediaFile]) repository.IMediaRepository {
	return &MediaRepository{coll: db}
}
