package mongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/amirdaaee/Glide/internals/domain"
	"github.com/amirdaaee/Glide/internals/repository"
	"github.com/chenmingyong0423/go-mongox/v2"
	"github.com/chenmingyong0423/go-mongox/v2/builder/query"
	"github.com/chenmingyong0423/go-mongox/v2/builder/update"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type JobRepository struct {
	coll      *mongox.Collection[domain.MediaJob]
	tasksColl *mongox.Collection[domain.ProcessedTask]
}

var _ repository.IJobRepository = (*JobRepository)(nil)

func (r *JobRepository) Create(ctx context.Context, job *domain.MediaJob) error {
	if _, err := r.coll.Creator().InsertOne(ctx, job); err != nil {
		return fmt.Errorf("can not create media job: %w", err)
	}
	return nil
}

func (r *JobRepository) GetByMediaID(ctx context.Context, mediaID bson.ObjectID) (*domain.MediaJob, error) {
	job, err := r.coll.Finder().Filter(query.Eq("MediaID", mediaID)).FindOne(ctx)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, repository.NotFoundError
		}
		return nil, fmt.Errorf("can not get media job by media id: %w", err)
	}
	return job, nil
}

func (r *JobRepository) GetByIdempotencyKey(ctx context.Context, key string) (*domain.MediaJob, error) {
	job, err := r.coll.Finder().Filter(query.Eq("IdempotencyKey", key)).FindOne(ctx)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, repository.NotFoundError
		}
		return nil, fmt.Errorf("can not get media job by idempotency key: %w", err)
	}
	return job, nil
}

func (r *JobRepository) Transition(ctx context.Context, mediaID bson.ObjectID, from, to domain.JobStep, ver int) (bool, error) {
	filter := query.NewBuilder().
		Eq("MediaID", mediaID).
		Eq("Step", from).
		Eq("StatusVer", ver).
		Build()
	updateVal := update.NewBuilder().
		Set("Step", to).
		Set("StatusVer", ver+1).
		Build()
	res, err := r.coll.Updater().Filter(filter).Updates(updateVal).UpdateOne(ctx)
	if err != nil {
		return false, fmt.Errorf("can not transition media job: %w", err)
	}
	return res.MatchedCount > 0, nil
}

func (r *JobRepository) RecordFailure(ctx context.Context, mediaID bson.ObjectID, step domain.JobStep, errMsg string) error {
	updateVal := update.NewBuilder().
		Set("LastError", errMsg).
		Set("Step", domain.JobStepFailed).
		Inc("Attempts."+string(step), 1).
		Build()
	res, err := r.coll.Updater().Filter(query.Eq("MediaID", mediaID)).Updates(updateVal).UpdateOne(ctx)
	if err != nil {
		return fmt.Errorf("can not record media job failure: %w", err)
	}
	if res.MatchedCount == 0 {
		return repository.NotFoundError
	}
	return nil
}

func (r *JobRepository) MarkTaskDone(ctx context.Context, mediaID bson.ObjectID, taskID string, step domain.JobStep) error {
	task := &domain.ProcessedTask{
		TaskID:  taskID,
		MediaID: mediaID,
		Step:    step,
		DoneAt:  time.Now().UTC(),
	}
	if _, err := r.tasksColl.Creator().InsertOne(ctx, task); err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			return fmt.Errorf("can not persist processed task: %w", err)
		}
	}
	updateVal := update.NewBuilder().
		Set("LastTaskID", taskID).
		Set("LastError", "").
		Build()
	res, err := r.coll.Updater().Filter(query.Eq("MediaID", mediaID)).Updates(updateVal).UpdateOne(ctx)
	if err != nil {
		return fmt.Errorf("can not mark task done: %w", err)
	}
	if res.MatchedCount == 0 {
		return repository.NotFoundError
	}
	return nil
}

func (r *JobRepository) IsTaskDone(ctx context.Context, mediaID bson.ObjectID, taskID string) (bool, error) {
	filter := query.NewBuilder().
		Eq("_id", taskID).
		Eq("MediaID", mediaID).
		Build()
	_, err := r.tasksColl.Finder().Filter(filter).FindOne(ctx)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return false, nil
		}
		return false, fmt.Errorf("can not get processed task: %w", err)
	}
	return true, nil
}

func (r *JobRepository) ensureIndexes(ctx context.Context) error {
	_, err := r.coll.Collection().Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "MediaID", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "IdempotencyKey", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	})
	if err != nil {
		return fmt.Errorf("can not create job indexes: %w", err)
	}
	_, err = r.tasksColl.Collection().Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "MediaID", Value: 1}},
		},
	})
	if err != nil {
		return fmt.Errorf("can not create processed task indexes: %w", err)
	}
	return nil
}

func NewJobRepository(db *mongox.Database, jobsName, tasksName string) (repository.IJobRepository, error) {
	r := &JobRepository{
		coll:      mongox.NewCollection[domain.MediaJob](db, jobsName),
		tasksColl: mongox.NewCollection[domain.ProcessedTask](db, tasksName),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := r.ensureIndexes(ctx); err != nil {
		return nil, err
	}
	return r, nil
}
