package domain

import (
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
	TaskID  string        `bson:"_id"`
	MediaID bson.ObjectID `bson:"MediaID"`
	Step    JobStep       `bson:"Step"`
	DoneAt  time.Time     `bson:"DoneAt"`
}
