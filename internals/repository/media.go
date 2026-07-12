package repository

import (
	"context"

	"github.com/amirdaaee/Glide/internals/domain"
)

type IMediaRepository interface {
	Create(ctx context.Context, media *domain.MediaFile) error
	GetByFID(ctx context.Context, fid int64) (*domain.MediaFile, error)
}
