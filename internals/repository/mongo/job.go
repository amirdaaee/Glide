package mongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/amirdaaee/Glide/internals/domain"
	"github.com/chenmingyong0423/go-mongox/v2"
	"github.com/chenmingyong0423/go-mongox/v2/builder/query"
	"github.com/chenmingyong0423/go-mongox/v2/builder/update"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// JobRepository persists MediaJob and ProcessedTask documents in MongoDB.
type JobRepository struct {
	coll      *mongox.Collection[domain.MediaJob]
	tasksColl *mongox.Collection[domain.ProcessedTask]
}

var _ domain.IJobRepository = (*JobRepository)(nil)

// Create inserts a media job.
func (r *JobRepository) Create(ctx context.Context, job *domain.MediaJob) error {
	if _, err := r.coll.Creator().InsertOne(ctx, job); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return domain.ErrAlreadyExists
		}
		return fmt.Errorf("can not create media job: %w", err)
	}
	return nil
}

// GetByMediaID returns the job for a media file.
func (r *JobRepository) GetByMediaID(ctx context.Context, mediaID bson.ObjectID) (*domain.MediaJob, error) {
	job, err := r.coll.Finder().Filter(query.Eq("MediaID", mediaID)).FindOne(ctx)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("can not get media job by media id: %w", err)
	}
	return job, nil
}

// GetByIdempotencyKey returns the job for an idempotency key.
func (r *JobRepository) GetByIdempotencyKey(ctx context.Context, key string) (*domain.MediaJob, error) {
	job, err := r.coll.Finder().Filter(query.Eq("IdempotencyKey", key)).FindOne(ctx)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("can not get media job by idempotency key: %w", err)
	}
	return job, nil
}

// CompareAndSetStep atomically moves the job from from to to when the version matches.
func (r *JobRepository) CompareAndSetStep(ctx context.Context, mediaID bson.ObjectID, from, to domain.JobStep, ver int) (bool, error) {
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
		return false, fmt.Errorf("can not compare-and-set media job step: %w", err)
	}
	return res.MatchedCount > 0, nil
}

// Save updates job fields by MediaID.
func (r *JobRepository) Save(ctx context.Context, job *domain.MediaJob) error {
	updateVal := update.NewBuilder().
		Set("Step", job.Step).
		Set("StatusVer", job.StatusVer).
		Set("Attempts", job.Attempts).
		Set("LastError", job.LastError).
		Set("LastTaskID", job.LastTaskID).
		Set("IdempotencyKey", job.IdempotencyKey).
		Build()
	res, err := r.coll.Updater().Filter(query.Eq("MediaID", job.MediaID)).Updates(updateVal).UpdateOne(ctx)
	if err != nil {
		return fmt.Errorf("can not save media job: %w", err)
	}
	if res.MatchedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// CreateProcessedTask inserts a processed-task record.
func (r *JobRepository) CreateProcessedTask(ctx context.Context, task *domain.ProcessedTask) error {
	if _, err := r.tasksColl.Creator().InsertOne(ctx, task); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return domain.ErrAlreadyExists
		}
		return fmt.Errorf("can not persist processed task: %w", err)
	}
	return nil
}

// HasProcessedTask reports whether taskID was already recorded for mediaID.
func (r *JobRepository) HasProcessedTask(ctx context.Context, mediaID bson.ObjectID, taskID string) (bool, error) {
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

// ensureIndexes creates unique MediaID and IdempotencyKey indexes.
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

// NewJobRepository returns a MongoDB job repository.
func NewJobRepository(db *mongox.Database, jobsName, tasksName string) (domain.IJobRepository, error) {
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
