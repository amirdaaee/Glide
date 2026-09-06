package domain

import (
	"context"

	"github.com/chenmingyong0423/go-mongox/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type MediaStatus string

const (
	MediaStatusReceived MediaStatus = "received"
	MediaStatusReady    MediaStatus = "ready"
	MediaStatusFailed   MediaStatus = "failed"
)

type MediaFileMeta struct {
	FileSize int64   `bson:"FileSize"`
	FileName string  `bson:"FileName"`
	MimeType string  `bson:"MimeType"`
	FileID   int64   `bson:"FileID"`
	Duration float64 `bson:"Duration"`
}

type MediaFile struct {
	mongox.Model `bson:",inline"`
	MessageID    int           `bson:"MessageID"`
	Meta         MediaFileMeta `bson:"Meta"`
	Status       MediaStatus   `bson:"Status"`
	StorageURL   string        `bson:"StorageURL,omitempty"`
}

type IMediaRepository interface {
	Create(ctx context.Context, media *MediaFile) error
	GetByFID(ctx context.Context, fid int64) (*MediaFile, error)
	GetMany(ctx context.Context, ids []bson.ObjectID) ([]*MediaFile, error)
	Delete(ctx context.Context, fid int64) error
	SetStatus(ctx context.Context, id bson.ObjectID, status MediaStatus) error
	SetStorageURL(ctx context.Context, id bson.ObjectID, url string) error
}
