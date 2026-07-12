package repository

import (
	"context"

	"github.com/amirdaaee/Glide/internals/domain"
)

type IMediaRepository interface {
	Create(ctx context.Context, media *domain.MediaFile) error
}
