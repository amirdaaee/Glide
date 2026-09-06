package domain

import (
	"context"
	"time"

	"github.com/chenmingyong0423/go-mongox/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type JobStep string

const (
	JobStepIngest   JobStep = "ingest"
	JobStepDownload JobStep = "download"
	JobStepUpload   JobStep = "upload"
	JobStepNotify   JobStep = "notify"
	JobStepDone     JobStep = "done"
	JobStepFailed   JobStep = "failed"
)

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

type ProcessedTask struct {
	mongox.Model `bson:",inline"`
	TaskID       string        `bson:"_id"`
	MediaID      bson.ObjectID `bson:"MediaID"`
	Step         JobStep       `bson:"Step"`
	DoneAt       time.Time     `bson:"DoneAt"`
}

type IJobRepository interface {
	Create(ctx context.Context, job *MediaJob) error
	GetByMediaID(ctx context.Context, mediaID bson.ObjectID) (*MediaJob, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*MediaJob, error)
	CompareAndSetStep(ctx context.Context, mediaID bson.ObjectID, from, to JobStep, ver int) (bool, error)
	Save(ctx context.Context, job *MediaJob) error
	CreateProcessedTask(ctx context.Context, task *ProcessedTask) error
	HasProcessedTask(ctx context.Context, mediaID bson.ObjectID, taskID string) (bool, error)
}
