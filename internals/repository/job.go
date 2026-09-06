package repository

import (
	"context"

	"github.com/amirdaaee/Glide/internals/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type IJobRepository interface {
	Create(ctx context.Context, job *domain.MediaJob) error
	GetByMediaID(ctx context.Context, mediaID bson.ObjectID) (*domain.MediaJob, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*domain.MediaJob, error)
	Transition(ctx context.Context, mediaID bson.ObjectID, from, to domain.JobStep, ver int) (bool, error)
	RecordFailure(ctx context.Context, mediaID bson.ObjectID, step domain.JobStep, errMsg string) error
	MarkTaskDone(ctx context.Context, mediaID bson.ObjectID, taskID string, step domain.JobStep) error
	IsTaskDone(ctx context.Context, mediaID bson.ObjectID, taskID string) (bool, error)
}
