package domain

import (
	"context"
	"time"

	"github.com/chenmingyong0423/go-mongox/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// JobStep is a named step in the media ingest pipeline.
type JobStep string

const (
	// JobStepIngest extracts a thumbnail from the channel document.
	JobStepIngest JobStep = "ingest"
	// JobStepDownload downloads the file and uploads it to Byse.
	JobStepDownload JobStep = "download"
	// JobStepNotify tells the bot to update client status replies.
	JobStepNotify JobStep = "notify"
	// JobStepDone means the pipeline finished successfully.
	JobStepDone JobStep = "done"
	// JobStepFailed means the pipeline stopped with an error.
	JobStepFailed JobStep = "failed"
)

// MediaJob tracks pipeline progress for one media file.
type MediaJob struct {
	mongox.Model   `bson:",inline"`
	MediaID        bson.ObjectID  `bson:"MediaID"`
	Step           JobStep        `bson:"Step"`
	StatusVer      int            `bson:"StatusVer"`
	Attempts       map[string]int `bson:"Attempts,omitempty"`
	LastError      string         `bson:"LastError,omitempty"`
	LastTaskID     string         `bson:"LastTaskID,omitempty"`
	IdempotencyKey string         `bson:"IdempotencyKey"`
}

// ProcessedTask records a completed work item for idempotency.
type ProcessedTask struct {
	mongox.Model `bson:",inline"`
	TaskID       string        `bson:"_id"`
	MediaID      bson.ObjectID `bson:"MediaID"`
	Step         JobStep       `bson:"Step"`
	DoneAt       time.Time     `bson:"DoneAt"`
}

// IJobRepository persists media jobs and processed tasks.
type IJobRepository interface {
	// Create inserts a media job.
	Create(ctx context.Context, job *MediaJob) error
	// GetByMediaID returns the job for a media file.
	GetByMediaID(ctx context.Context, mediaID bson.ObjectID) (*MediaJob, error)
	// GetByIdempotencyKey returns the job for an idempotency key.
	GetByIdempotencyKey(ctx context.Context, key string) (*MediaJob, error)
	// CompareAndSetStep atomically moves the job from from to to when the version matches.
	CompareAndSetStep(ctx context.Context, mediaID bson.ObjectID, from, to JobStep, ver int) (bool, error)
	// Save updates job fields by MediaID.
	Save(ctx context.Context, job *MediaJob) error
	// CreateProcessedTask inserts a processed-task record.
	CreateProcessedTask(ctx context.Context, task *ProcessedTask) error
	// HasProcessedTask reports whether taskID was already recorded for mediaID.
	HasProcessedTask(ctx context.Context, mediaID bson.ObjectID, taskID string) (bool, error)
}
